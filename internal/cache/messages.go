package cache

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

var (
	ErrThreadNotFound     = errors.New("thread not found")
	ErrMessageNotFound    = errors.New("message not found")
	ErrMessageLabelAbsent = errors.New("message label mapping not found")
	ErrAttachmentNotFound = errors.New("attachment not found")
)

type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Thread struct {
	AccountID       string
	ID              string
	Snippet         string
	HistoryID       string
	LastMessageDate *time.Time
}

type Message struct {
	AccountID    string
	ID           string
	ThreadID     string
	FromAddr     string
	ToAddrs      []string
	CcAddrs      []string
	BccAddrs     []string
	Subject      string
	Date         *time.Time
	Snippet      string
	SizeBytes    int
	BodyPlain    *string
	BodyHTML     *string
	RawHeaders   []Header
	InternalDate *time.Time
	FetchedFull  bool
	CachedAt     *time.Time
}

type MessageSummary struct {
	AccountID       string
	ID              string
	ThreadID        string
	FromAddr        string
	Subject         string
	Snippet         string
	Date            *time.Time
	InternalDate    *time.Time
	ThreadCount     int
	Unread          bool
	AttachmentCount int
	BodyCached      bool
}

type MessageLabel struct {
	AccountID string `db:"account_id"`
	MessageID string `db:"message_id"`
	LabelID   string `db:"label_id"`
}

type Attachment struct {
	AccountID    string
	MessageID    string
	PartID       string
	Filename     string
	MimeType     string
	SizeBytes    int
	AttachmentID string
	LocalPath    string
}

type ThreadRepository struct {
	db *sqlx.DB
}

func NewThreadRepository(db *sqlx.DB) ThreadRepository {
	return ThreadRepository{db: db}
}

func (r ThreadRepository) Upsert(ctx context.Context, thread Thread) error {
	if strings.TrimSpace(thread.AccountID) == "" {
		return errors.New("thread account id is required")
	}
	if strings.TrimSpace(thread.ID) == "" {
		return errors.New("thread id is required")
	}

	_, err := r.db.ExecContext(ctx, `
INSERT INTO threads (account_id, id, snippet, history_id, last_message_date)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(account_id, id) DO UPDATE SET
  snippet = excluded.snippet,
  history_id = excluded.history_id,
  last_message_date = excluded.last_message_date
`, thread.AccountID, thread.ID, nullIfEmpty(thread.Snippet), nullIfEmpty(thread.HistoryID), timeOrNil(thread.LastMessageDate))
	if err != nil {
		return fmt.Errorf("upsert thread: %w", err)
	}
	return nil
}

func (r ThreadRepository) Get(ctx context.Context, accountID string, id string) (Thread, error) {
	var row threadRow
	err := r.db.GetContext(ctx, &row, `
SELECT account_id, id, snippet, history_id, last_message_date
FROM threads
WHERE account_id = ? AND id = ?
`, accountID, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Thread{}, ErrThreadNotFound
	}
	if err != nil {
		return Thread{}, fmt.Errorf("get thread: %w", err)
	}
	return row.thread(), nil
}

func (r ThreadRepository) List(ctx context.Context, accountID string, limit int) ([]Thread, error) {
	query := `
SELECT account_id, id, snippet, history_id, last_message_date
FROM threads
WHERE account_id = ?
ORDER BY last_message_date DESC, id
`
	args := []any{accountID}
	if limit > 0 {
		query += "LIMIT ?"
		args = append(args, limit)
	}

	rows := []threadRow{}
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("list threads: %w", err)
	}
	return mapThreadRows(rows), nil
}

type MessageRepository struct {
	db *sqlx.DB
}

func NewMessageRepository(db *sqlx.DB) MessageRepository {
	return MessageRepository{db: db}
}

func (r MessageRepository) Upsert(ctx context.Context, message Message) error {
	if strings.TrimSpace(message.AccountID) == "" {
		return errors.New("message account id is required")
	}
	if strings.TrimSpace(message.ID) == "" {
		return errors.New("message id is required")
	}
	if strings.TrimSpace(message.ThreadID) == "" {
		return errors.New("message thread id is required")
	}

	toAddrs, err := encodeStringSlice(message.ToAddrs)
	if err != nil {
		return err
	}
	ccAddrs, err := encodeStringSlice(message.CcAddrs)
	if err != nil {
		return err
	}
	bccAddrs, err := encodeStringSlice(message.BccAddrs)
	if err != nil {
		return err
	}
	headers, err := encodeHeaders(message.RawHeaders)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, `
INSERT INTO messages (
  account_id, id, thread_id, from_addr, to_addrs, cc_addrs, bcc_addrs,
  subject, date, snippet, size_bytes, body_plain, body_html, raw_headers,
  internal_date, fetched_full, cached_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(account_id, id) DO UPDATE SET
  thread_id = excluded.thread_id,
  from_addr = excluded.from_addr,
  to_addrs = excluded.to_addrs,
  cc_addrs = excluded.cc_addrs,
  bcc_addrs = excluded.bcc_addrs,
  subject = excluded.subject,
  date = excluded.date,
  snippet = excluded.snippet,
  size_bytes = excluded.size_bytes,
  body_plain = COALESCE(excluded.body_plain, messages.body_plain),
  body_html = COALESCE(excluded.body_html, messages.body_html),
  raw_headers = excluded.raw_headers,
  internal_date = excluded.internal_date,
  fetched_full = CASE WHEN excluded.fetched_full = 1 THEN excluded.fetched_full ELSE messages.fetched_full END,
  cached_at = COALESCE(excluded.cached_at, messages.cached_at)
`, message.AccountID, message.ID, message.ThreadID, nullIfEmpty(message.FromAddr), toAddrs, ccAddrs, bccAddrs,
		nullIfEmpty(message.Subject), timeOrNil(message.Date), nullIfEmpty(message.Snippet), message.SizeBytes,
		stringPtrOrNil(message.BodyPlain), stringPtrOrNil(message.BodyHTML), headers,
		timeOrNil(message.InternalDate), boolInt(message.FetchedFull), timeOrNil(message.CachedAt))
	if err != nil {
		return fmt.Errorf("upsert message: %w", err)
	}
	return nil
}

func (r MessageRepository) Delete(ctx context.Context, accountID string, id string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM messages WHERE account_id = ? AND id = ?", accountID, id)
	if err != nil {
		return fmt.Errorf("delete message: %w", err)
	}
	return requireAffected(result, ErrMessageNotFound)
}

func (r MessageRepository) Get(ctx context.Context, accountID string, id string) (Message, error) {
	var row messageRow
	err := r.db.GetContext(ctx, &row, messageSelectSQL+`
WHERE account_id = ? AND id = ?
`, accountID, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Message{}, ErrMessageNotFound
	}
	if err != nil {
		return Message{}, fmt.Errorf("get message: %w", err)
	}
	return row.message()
}

func (r MessageRepository) ListByThread(ctx context.Context, accountID string, threadID string) ([]Message, error) {
	rows := []messageRow{}
	if err := r.db.SelectContext(ctx, &rows, messageSelectSQL+`
WHERE account_id = ? AND thread_id = ?
ORDER BY internal_date, date, id
`, accountID, threadID); err != nil {
		return nil, fmt.Errorf("list messages by thread: %w", err)
	}
	return mapMessageRows(rows)
}

func (r MessageRepository) ListByLabel(ctx context.Context, accountID string, labelID string, limit int) ([]Message, error) {
	query := messageSelectSQL + `
WHERE account_id = ? AND id IN (
  SELECT message_id FROM message_labels WHERE account_id = ? AND label_id = ?
)
ORDER BY internal_date DESC, date DESC, id
`
	args := []any{accountID, accountID, labelID}
	if limit > 0 {
		query += "LIMIT ?"
		args = append(args, limit)
	}

	rows := []messageRow{}
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("list messages by label: %w", err)
	}
	return mapMessageRows(rows)
}

func (r MessageRepository) ListSummariesByLabel(ctx context.Context, accountID string, labelID string, limit int) ([]MessageSummary, error) {
	query := `
SELECT m.account_id, m.id, m.thread_id, m.from_addr, m.subject, m.snippet, m.date, m.internal_date,
       (
         SELECT COUNT(*) FROM messages tm
         WHERE tm.account_id = m.account_id AND tm.thread_id = m.thread_id
       ) AS thread_count,
       EXISTS (
         SELECT 1 FROM message_labels unread
         WHERE unread.account_id = m.account_id AND unread.message_id = m.id AND unread.label_id = 'UNREAD'
       ) AS unread,
       (
         SELECT COUNT(*) FROM attachments a
         WHERE a.account_id = m.account_id AND a.message_id = m.id
       ) AS attachment_count,
       CASE WHEN m.body_plain IS NOT NULL OR m.body_html IS NOT NULL OR m.cached_at IS NOT NULL THEN 1 ELSE 0 END AS body_cached
FROM messages m
JOIN message_labels ml ON ml.account_id = m.account_id AND ml.message_id = m.id
WHERE m.account_id = ? AND ml.label_id = ?
ORDER BY m.internal_date DESC, m.date DESC, m.id DESC
`
	args := []any{accountID, labelID}
	if limit > 0 {
		queryLimit := limit * 4
		if queryLimit < limit {
			queryLimit = limit
		}
		query += "LIMIT ?"
		args = append(args, queryLimit)
	}

	rows := []messageSummaryRow{}
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("list message summaries by label: %w", err)
	}

	summaries := make([]MessageSummary, 0, len(rows))
	seenThreads := map[string]bool{}
	for _, row := range rows {
		if seenThreads[row.ThreadID] {
			continue
		}
		seenThreads[row.ThreadID] = true
		summaries = append(summaries, row.summary())
		if limit > 0 && len(summaries) >= limit {
			break
		}
	}
	return summaries, nil
}

type MessageLabelRepository struct {
	db *sqlx.DB
}

func NewMessageLabelRepository(db *sqlx.DB) MessageLabelRepository {
	return MessageLabelRepository{db: db}
}

func (r MessageLabelRepository) Upsert(ctx context.Context, mapping MessageLabel) error {
	if strings.TrimSpace(mapping.AccountID) == "" {
		return errors.New("message label account id is required")
	}
	if strings.TrimSpace(mapping.MessageID) == "" {
		return errors.New("message label message id is required")
	}
	if strings.TrimSpace(mapping.LabelID) == "" {
		return errors.New("message label label id is required")
	}

	_, err := r.db.ExecContext(ctx, `
INSERT INTO message_labels (account_id, message_id, label_id)
VALUES (?, ?, ?)
ON CONFLICT(account_id, message_id, label_id) DO NOTHING
`, mapping.AccountID, mapping.MessageID, mapping.LabelID)
	if err != nil {
		return fmt.Errorf("upsert message label: %w", err)
	}
	return nil
}

func (r MessageLabelRepository) Get(ctx context.Context, accountID string, messageID string, labelID string) (MessageLabel, error) {
	var mapping MessageLabel
	err := r.db.GetContext(ctx, &mapping, `
SELECT account_id, message_id, label_id
FROM message_labels
WHERE account_id = ? AND message_id = ? AND label_id = ?
`, accountID, messageID, labelID)
	if errors.Is(err, sql.ErrNoRows) {
		return MessageLabel{}, ErrMessageLabelAbsent
	}
	if err != nil {
		return MessageLabel{}, fmt.Errorf("get message label: %w", err)
	}
	return mapping, nil
}

func (r MessageLabelRepository) ListForMessage(ctx context.Context, accountID string, messageID string) ([]MessageLabel, error) {
	mappings := []MessageLabel{}
	if err := r.db.SelectContext(ctx, &mappings, `
SELECT account_id, message_id, label_id
FROM message_labels
WHERE account_id = ? AND message_id = ?
ORDER BY label_id
`, accountID, messageID); err != nil {
		return nil, fmt.Errorf("list message labels: %w", err)
	}
	return mappings, nil
}

func (r MessageLabelRepository) ReplaceForMessage(ctx context.Context, accountID string, messageID string, labelIDs []string) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin replace message labels: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM message_labels WHERE account_id = ? AND message_id = ?", accountID, messageID); err != nil {
		return fmt.Errorf("delete old message labels: %w", err)
	}
	for _, labelID := range labelIDs {
		mapping := MessageLabel{AccountID: accountID, MessageID: messageID, LabelID: labelID}
		if err := upsertMessageLabel(ctx, tx, mapping); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit replace message labels: %w", err)
	}
	return nil
}

func (r MessageLabelRepository) Delete(ctx context.Context, accountID string, messageID string, labelID string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM message_labels WHERE account_id = ? AND message_id = ? AND label_id = ?", accountID, messageID, labelID)
	if err != nil {
		return fmt.Errorf("delete message label: %w", err)
	}
	return requireAffected(result, ErrMessageLabelAbsent)
}

type AttachmentRepository struct {
	db *sqlx.DB
}

func NewAttachmentRepository(db *sqlx.DB) AttachmentRepository {
	return AttachmentRepository{db: db}
}

func (r AttachmentRepository) Upsert(ctx context.Context, attachment Attachment) error {
	if strings.TrimSpace(attachment.AccountID) == "" {
		return errors.New("attachment account id is required")
	}
	if strings.TrimSpace(attachment.MessageID) == "" {
		return errors.New("attachment message id is required")
	}
	if strings.TrimSpace(attachment.PartID) == "" {
		return errors.New("attachment part id is required")
	}

	_, err := r.db.ExecContext(ctx, `
INSERT INTO attachments (account_id, message_id, part_id, filename, mime_type, size_bytes, attachment_id, local_path)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(account_id, message_id, part_id) DO UPDATE SET
  filename = excluded.filename,
  mime_type = excluded.mime_type,
  size_bytes = excluded.size_bytes,
  attachment_id = excluded.attachment_id,
  local_path = excluded.local_path
`, attachment.AccountID, attachment.MessageID, attachment.PartID, nullIfEmpty(attachment.Filename), nullIfEmpty(attachment.MimeType), attachment.SizeBytes, nullIfEmpty(attachment.AttachmentID), nullIfEmpty(attachment.LocalPath))
	if err != nil {
		return fmt.Errorf("upsert attachment: %w", err)
	}
	return nil
}

func (r AttachmentRepository) Get(ctx context.Context, accountID string, messageID string, partID string) (Attachment, error) {
	var row attachmentRow
	err := r.db.GetContext(ctx, &row, `
SELECT account_id, message_id, part_id, filename, mime_type, size_bytes, attachment_id, local_path
FROM attachments
WHERE account_id = ? AND message_id = ? AND part_id = ?
`, accountID, messageID, partID)
	if errors.Is(err, sql.ErrNoRows) {
		return Attachment{}, ErrAttachmentNotFound
	}
	if err != nil {
		return Attachment{}, fmt.Errorf("get attachment: %w", err)
	}
	return row.attachment(), nil
}

func (r AttachmentRepository) ListForMessage(ctx context.Context, accountID string, messageID string) ([]Attachment, error) {
	rows := []attachmentRow{}
	if err := r.db.SelectContext(ctx, &rows, `
SELECT account_id, message_id, part_id, filename, mime_type, size_bytes, attachment_id, local_path
FROM attachments
WHERE account_id = ? AND message_id = ?
ORDER BY part_id
`, accountID, messageID); err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}

	attachments := make([]Attachment, 0, len(rows))
	for _, row := range rows {
		attachments = append(attachments, row.attachment())
	}
	return attachments, nil
}

const messageSelectSQL = `
SELECT account_id, id, thread_id, from_addr, to_addrs, cc_addrs, bcc_addrs,
  subject, date, snippet, size_bytes, body_plain, body_html, raw_headers,
  internal_date, fetched_full, cached_at
FROM messages
`

type threadRow struct {
	AccountID       string         `db:"account_id"`
	ID              string         `db:"id"`
	Snippet         sql.NullString `db:"snippet"`
	HistoryID       sql.NullString `db:"history_id"`
	LastMessageDate sql.NullTime   `db:"last_message_date"`
}

func (row threadRow) thread() Thread {
	return Thread{
		AccountID:       row.AccountID,
		ID:              row.ID,
		Snippet:         row.Snippet.String,
		HistoryID:       row.HistoryID.String,
		LastMessageDate: nullTimePtr(row.LastMessageDate),
	}
}

type messageRow struct {
	AccountID    string         `db:"account_id"`
	ID           string         `db:"id"`
	ThreadID     string         `db:"thread_id"`
	FromAddr     sql.NullString `db:"from_addr"`
	ToAddrs      string         `db:"to_addrs"`
	CcAddrs      string         `db:"cc_addrs"`
	BccAddrs     string         `db:"bcc_addrs"`
	Subject      sql.NullString `db:"subject"`
	Date         sql.NullTime   `db:"date"`
	Snippet      sql.NullString `db:"snippet"`
	SizeBytes    int            `db:"size_bytes"`
	BodyPlain    sql.NullString `db:"body_plain"`
	BodyHTML     sql.NullString `db:"body_html"`
	RawHeaders   string         `db:"raw_headers"`
	InternalDate sql.NullTime   `db:"internal_date"`
	FetchedFull  int            `db:"fetched_full"`
	CachedAt     sql.NullTime   `db:"cached_at"`
}

type messageSummaryRow struct {
	AccountID       string         `db:"account_id"`
	ID              string         `db:"id"`
	ThreadID        string         `db:"thread_id"`
	FromAddr        sql.NullString `db:"from_addr"`
	Subject         sql.NullString `db:"subject"`
	Snippet         sql.NullString `db:"snippet"`
	Date            sql.NullTime   `db:"date"`
	InternalDate    sql.NullTime   `db:"internal_date"`
	ThreadCount     int            `db:"thread_count"`
	Unread          int            `db:"unread"`
	AttachmentCount int            `db:"attachment_count"`
	BodyCached      int            `db:"body_cached"`
}

func (row messageSummaryRow) summary() MessageSummary {
	return MessageSummary{
		AccountID:       row.AccountID,
		ID:              row.ID,
		ThreadID:        row.ThreadID,
		FromAddr:        row.FromAddr.String,
		Subject:         row.Subject.String,
		Snippet:         row.Snippet.String,
		Date:            nullTimePtr(row.Date),
		InternalDate:    nullTimePtr(row.InternalDate),
		ThreadCount:     row.ThreadCount,
		Unread:          row.Unread == 1,
		AttachmentCount: row.AttachmentCount,
		BodyCached:      row.BodyCached == 1,
	}
}

func (row messageRow) message() (Message, error) {
	toAddrs, err := decodeStringSlice(row.ToAddrs)
	if err != nil {
		return Message{}, err
	}
	ccAddrs, err := decodeStringSlice(row.CcAddrs)
	if err != nil {
		return Message{}, err
	}
	bccAddrs, err := decodeStringSlice(row.BccAddrs)
	if err != nil {
		return Message{}, err
	}
	headers, err := decodeHeaders(row.RawHeaders)
	if err != nil {
		return Message{}, err
	}
	return Message{
		AccountID:    row.AccountID,
		ID:           row.ID,
		ThreadID:     row.ThreadID,
		FromAddr:     row.FromAddr.String,
		ToAddrs:      toAddrs,
		CcAddrs:      ccAddrs,
		BccAddrs:     bccAddrs,
		Subject:      row.Subject.String,
		Date:         nullTimePtr(row.Date),
		Snippet:      row.Snippet.String,
		SizeBytes:    row.SizeBytes,
		BodyPlain:    nullStringPtr(row.BodyPlain),
		BodyHTML:     nullStringPtr(row.BodyHTML),
		RawHeaders:   headers,
		InternalDate: nullTimePtr(row.InternalDate),
		FetchedFull:  row.FetchedFull == 1,
		CachedAt:     nullTimePtr(row.CachedAt),
	}, nil
}

type attachmentRow struct {
	AccountID    string         `db:"account_id"`
	MessageID    string         `db:"message_id"`
	PartID       string         `db:"part_id"`
	Filename     sql.NullString `db:"filename"`
	MimeType     sql.NullString `db:"mime_type"`
	SizeBytes    int            `db:"size_bytes"`
	AttachmentID sql.NullString `db:"attachment_id"`
	LocalPath    sql.NullString `db:"local_path"`
}

func (row attachmentRow) attachment() Attachment {
	return Attachment{
		AccountID:    row.AccountID,
		MessageID:    row.MessageID,
		PartID:       row.PartID,
		Filename:     row.Filename.String,
		MimeType:     row.MimeType.String,
		SizeBytes:    row.SizeBytes,
		AttachmentID: row.AttachmentID.String,
		LocalPath:    row.LocalPath.String,
	}
}

func mapThreadRows(rows []threadRow) []Thread {
	threads := make([]Thread, 0, len(rows))
	for _, row := range rows {
		threads = append(threads, row.thread())
	}
	return threads
}

func mapMessageRows(rows []messageRow) ([]Message, error) {
	messages := make([]Message, 0, len(rows))
	for _, row := range rows {
		message, err := row.message()
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, nil
}

func upsertMessageLabel(ctx context.Context, exec sqlx.ExtContext, mapping MessageLabel) error {
	if strings.TrimSpace(mapping.LabelID) == "" {
		return nil
	}
	_, err := exec.ExecContext(ctx, `
INSERT INTO message_labels (account_id, message_id, label_id)
VALUES (?, ?, ?)
ON CONFLICT(account_id, message_id, label_id) DO NOTHING
`, mapping.AccountID, mapping.MessageID, mapping.LabelID)
	if err != nil {
		return fmt.Errorf("upsert message label: %w", err)
	}
	return nil
}

func encodeStringSlice(values []string) (string, error) {
	if values == nil {
		values = []string{}
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("encode string slice: %w", err)
	}
	return string(encoded), nil
}

func decodeStringSlice(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var out []string
	if err := json.Unmarshal([]byte(value), &out); err != nil {
		return nil, fmt.Errorf("decode string slice: %w", err)
	}
	return out, nil
}

func encodeHeaders(headers []Header) (string, error) {
	if headers == nil {
		headers = []Header{}
	}
	encoded, err := json.Marshal(headers)
	if err != nil {
		return "", fmt.Errorf("encode headers: %w", err)
	}
	return string(encoded), nil
}

func decodeHeaders(value string) ([]Header, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var out []Header
	if err := json.Unmarshal([]byte(value), &out); err != nil {
		return nil, fmt.Errorf("decode headers: %w", err)
	}
	return out, nil
}

func stringPtrOrNil(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func timeOrNil(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC()
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	out := value.Time
	return &out
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	out := value.String
	return &out
}
