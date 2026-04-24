package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type CacheStatsService struct {
	db *sqlx.DB
}

func NewCacheStatsService(db *sqlx.DB) CacheStatsService {
	return CacheStatsService{db: db}
}

type CacheStats struct {
	GeneratedAt time.Time
	Total       CacheTotalStats
	Accounts    []AccountCacheStats
	Labels      []LabelCacheStats
	Ages        []AgeCacheStats
	Attachments AttachmentCacheStats
	Rows        []TableRowCount
	FTS         FTSFootprintStats
}

type CacheTotalStats struct {
	SQLiteBytes     int64
	MessageCount    int   `db:"message_count"`
	BodyCount       int   `db:"body_count"`
	MessageBytes    int64 `db:"message_bytes"`
	BodyBytes       int64 `db:"body_bytes"`
	AttachmentBytes int64
	FTSBytes        int64
	TotalBytes      int64
}

type AccountCacheStats struct {
	AccountID       string `db:"account_id"`
	Email           string `db:"email"`
	MessageCount    int    `db:"message_count"`
	BodyCount       int    `db:"body_count"`
	AttachmentCount int    `db:"attachment_count"`
	MessageBytes    int64  `db:"message_bytes"`
	BodyBytes       int64  `db:"body_bytes"`
	AttachmentBytes int64  `db:"attachment_bytes"`
	TotalBytes      int64
}

type LabelCacheStats struct {
	AccountID       string `db:"account_id"`
	LabelID         string `db:"label_id"`
	LabelName       string `db:"label_name"`
	MessageCount    int    `db:"message_count"`
	BodyCount       int    `db:"body_count"`
	AttachmentCount int    `db:"attachment_count"`
	MessageBytes    int64  `db:"message_bytes"`
	BodyBytes       int64  `db:"body_bytes"`
	AttachmentBytes int64  `db:"attachment_bytes"`
	TotalBytes      int64
}

type AgeCacheStats struct {
	Bucket          string `db:"bucket"`
	MessageCount    int    `db:"message_count"`
	BodyCount       int    `db:"body_count"`
	AttachmentCount int    `db:"attachment_count"`
	MessageBytes    int64  `db:"message_bytes"`
	BodyBytes       int64  `db:"body_bytes"`
	AttachmentBytes int64  `db:"attachment_bytes"`
	TotalBytes      int64
}

type AttachmentCacheStats struct {
	AttachmentCount int   `db:"attachment_count"`
	CachedFileCount int   `db:"cached_file_count"`
	MetadataBytes   int64 `db:"metadata_bytes"`
	LocalBytes      int64 `db:"local_bytes"`
}

type TableRowCount struct {
	Table string
	Rows  int
}

type FTSFootprintStats struct {
	RowCount int   `db:"row_count"`
	Bytes    int64 `db:"bytes"`
}

func (s CacheStatsService) Summary(ctx context.Context, now time.Time) (CacheStats, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	stats := CacheStats{GeneratedAt: now.UTC()}

	total, err := s.total(ctx)
	if err != nil {
		return CacheStats{}, err
	}
	stats.Total = total

	if stats.Accounts, err = s.accounts(ctx); err != nil {
		return CacheStats{}, err
	}
	if stats.Labels, err = s.labels(ctx); err != nil {
		return CacheStats{}, err
	}
	if stats.Ages, err = s.ages(ctx, now.UTC()); err != nil {
		return CacheStats{}, err
	}
	if stats.Attachments, err = s.attachments(ctx); err != nil {
		return CacheStats{}, err
	}
	if stats.Rows, err = s.rows(ctx); err != nil {
		return CacheStats{}, err
	}
	if stats.FTS, err = s.fts(ctx); err != nil {
		return CacheStats{}, err
	}

	stats.Total.AttachmentBytes = stats.Attachments.LocalBytes
	stats.Total.FTSBytes = stats.FTS.Bytes
	stats.Total.TotalBytes = stats.Total.MessageBytes + stats.Total.BodyBytes + stats.Total.AttachmentBytes + stats.Total.FTSBytes
	return stats, nil
}

func (s CacheStatsService) total(ctx context.Context) (CacheTotalStats, error) {
	var pageCount int64
	if err := s.db.GetContext(ctx, &pageCount, "PRAGMA page_count"); err != nil {
		return CacheTotalStats{}, fmt.Errorf("read cache page count: %w", err)
	}
	var pageSize int64
	if err := s.db.GetContext(ctx, &pageSize, "PRAGMA page_size"); err != nil {
		return CacheTotalStats{}, fmt.Errorf("read cache page size: %w", err)
	}

	var total CacheTotalStats
	if err := s.db.GetContext(ctx, &total, `
SELECT
  COUNT(*) AS message_count,
  COALESCE(SUM(CASE WHEN body_plain IS NOT NULL OR body_html IS NOT NULL OR cached_at IS NOT NULL THEN 1 ELSE 0 END), 0) AS body_count,
  COALESCE(SUM(size_bytes), 0) AS message_bytes,
  COALESCE(SUM(LENGTH(COALESCE(body_plain, '')) + LENGTH(COALESCE(body_html, '')) + LENGTH(COALESCE(raw_headers, ''))), 0) AS body_bytes
FROM messages
`); err != nil {
		return CacheTotalStats{}, fmt.Errorf("read cache totals: %w", err)
	}
	total.SQLiteBytes = pageCount * pageSize
	return total, nil
}

func (s CacheStatsService) accounts(ctx context.Context) ([]AccountCacheStats, error) {
	rows := []AccountCacheStats{}
	if err := s.db.SelectContext(ctx, &rows, `
WITH message_stats AS (
  SELECT account_id,
         COUNT(*) AS message_count,
         SUM(CASE WHEN body_plain IS NOT NULL OR body_html IS NOT NULL OR cached_at IS NOT NULL THEN 1 ELSE 0 END) AS body_count,
         SUM(size_bytes) AS message_bytes,
         SUM(LENGTH(COALESCE(body_plain, '')) + LENGTH(COALESCE(body_html, '')) + LENGTH(COALESCE(raw_headers, ''))) AS body_bytes
  FROM messages
  GROUP BY account_id
),
attachment_stats AS (
  SELECT account_id,
         COUNT(*) AS attachment_count,
         SUM(size_bytes) AS attachment_bytes
  FROM attachments
  GROUP BY account_id
)
SELECT a.id AS account_id,
       a.email AS email,
       COALESCE(ms.message_count, 0) AS message_count,
       COALESCE(ms.body_count, 0) AS body_count,
       COALESCE(ast.attachment_count, 0) AS attachment_count,
       COALESCE(ms.message_bytes, 0) AS message_bytes,
       COALESCE(ms.body_bytes, 0) AS body_bytes,
       COALESCE(ast.attachment_bytes, 0) AS attachment_bytes
FROM accounts a
LEFT JOIN message_stats ms ON ms.account_id = a.id
LEFT JOIN attachment_stats ast ON ast.account_id = a.id
ORDER BY a.email
`); err != nil {
		return nil, fmt.Errorf("read account cache stats: %w", err)
	}
	for i := range rows {
		rows[i].TotalBytes = rows[i].MessageBytes + rows[i].BodyBytes + rows[i].AttachmentBytes
	}
	return rows, nil
}

func (s CacheStatsService) labels(ctx context.Context) ([]LabelCacheStats, error) {
	rows := []LabelCacheStats{}
	if err := s.db.SelectContext(ctx, &rows, `
WITH message_stats AS (
  SELECT ml.account_id,
         ml.label_id,
         COUNT(*) AS message_count,
         SUM(CASE WHEN m.body_plain IS NOT NULL OR m.body_html IS NOT NULL OR m.cached_at IS NOT NULL THEN 1 ELSE 0 END) AS body_count,
         SUM(m.size_bytes) AS message_bytes,
         SUM(LENGTH(COALESCE(m.body_plain, '')) + LENGTH(COALESCE(m.body_html, '')) + LENGTH(COALESCE(m.raw_headers, ''))) AS body_bytes
  FROM message_labels ml
  JOIN messages m ON m.account_id = ml.account_id AND m.id = ml.message_id
  GROUP BY ml.account_id, ml.label_id
),
attachment_stats AS (
  SELECT ml.account_id,
         ml.label_id,
         COUNT(a.part_id) AS attachment_count,
         SUM(a.size_bytes) AS attachment_bytes
  FROM message_labels ml
  JOIN attachments a ON a.account_id = ml.account_id AND a.message_id = ml.message_id
  GROUP BY ml.account_id, ml.label_id
)
SELECT l.account_id,
       l.id AS label_id,
       l.name AS label_name,
       COALESCE(ms.message_count, 0) AS message_count,
       COALESCE(ms.body_count, 0) AS body_count,
       COALESCE(ast.attachment_count, 0) AS attachment_count,
       COALESCE(ms.message_bytes, 0) AS message_bytes,
       COALESCE(ms.body_bytes, 0) AS body_bytes,
       COALESCE(ast.attachment_bytes, 0) AS attachment_bytes
FROM labels l
LEFT JOIN message_stats ms ON ms.account_id = l.account_id AND ms.label_id = l.id
LEFT JOIN attachment_stats ast ON ast.account_id = l.account_id AND ast.label_id = l.id
ORDER BY l.account_id, CASE l.type WHEN 'system' THEN 0 ELSE 1 END, l.name COLLATE NOCASE
`); err != nil {
		return nil, fmt.Errorf("read label cache stats: %w", err)
	}
	for i := range rows {
		rows[i].TotalBytes = rows[i].MessageBytes + rows[i].BodyBytes + rows[i].AttachmentBytes
	}
	return rows, nil
}

func (s CacheStatsService) ages(ctx context.Context, now time.Time) ([]AgeCacheStats, error) {
	rows := []AgeCacheStats{}
	if err := s.db.SelectContext(ctx, &rows, `
WITH bucketed AS (
  SELECT account_id,
         id,
         CASE
           WHEN COALESCE(internal_date, date, cached_at) IS NULL THEN 'unknown'
           WHEN COALESCE(internal_date, date, cached_at) >= ? THEN '0-7d'
           WHEN COALESCE(internal_date, date, cached_at) >= ? THEN '8-30d'
           WHEN COALESCE(internal_date, date, cached_at) >= ? THEN '31-90d'
           WHEN COALESCE(internal_date, date, cached_at) >= ? THEN '91-365d'
           ELSE '365d+'
         END AS bucket,
         CASE WHEN body_plain IS NOT NULL OR body_html IS NOT NULL OR cached_at IS NOT NULL THEN 1 ELSE 0 END AS body_cached,
         COALESCE(size_bytes, 0) AS message_bytes,
         LENGTH(COALESCE(body_plain, '')) + LENGTH(COALESCE(body_html, '')) + LENGTH(COALESCE(raw_headers, '')) AS body_bytes
  FROM messages
),
message_stats AS (
  SELECT bucket,
         COUNT(*) AS message_count,
         SUM(body_cached) AS body_count,
         SUM(message_bytes) AS message_bytes,
         SUM(body_bytes) AS body_bytes
  FROM bucketed
  GROUP BY bucket
),
attachment_stats AS (
  SELECT b.bucket,
         COUNT(a.part_id) AS attachment_count,
         SUM(a.size_bytes) AS attachment_bytes
  FROM bucketed b
  JOIN attachments a ON a.account_id = b.account_id AND a.message_id = b.id
  GROUP BY b.bucket
)
SELECT ms.bucket,
       COALESCE(ms.message_count, 0) AS message_count,
       COALESCE(ms.body_count, 0) AS body_count,
       COALESCE(ast.attachment_count, 0) AS attachment_count,
       COALESCE(ms.message_bytes, 0) AS message_bytes,
       COALESCE(ms.body_bytes, 0) AS body_bytes,
       COALESCE(ast.attachment_bytes, 0) AS attachment_bytes
FROM message_stats ms
LEFT JOIN attachment_stats ast ON ast.bucket = ms.bucket
ORDER BY CASE ms.bucket
  WHEN '0-7d' THEN 0
  WHEN '8-30d' THEN 1
  WHEN '31-90d' THEN 2
  WHEN '91-365d' THEN 3
  WHEN '365d+' THEN 4
  ELSE 5
END
`, now.AddDate(0, 0, -7), now.AddDate(0, 0, -30), now.AddDate(0, 0, -90), now.AddDate(0, 0, -365)); err != nil {
		return nil, fmt.Errorf("read message age cache stats: %w", err)
	}
	for i := range rows {
		rows[i].TotalBytes = rows[i].MessageBytes + rows[i].BodyBytes + rows[i].AttachmentBytes
	}
	return rows, nil
}

func (s CacheStatsService) attachments(ctx context.Context) (AttachmentCacheStats, error) {
	var stats AttachmentCacheStats
	if err := s.db.GetContext(ctx, &stats, `
SELECT
  COUNT(*) AS attachment_count,
  SUM(CASE WHEN local_path IS NOT NULL AND local_path != '' THEN 1 ELSE 0 END) AS cached_file_count,
  COALESCE(SUM(LENGTH(COALESCE(filename, '')) + LENGTH(COALESCE(mime_type, '')) + LENGTH(COALESCE(attachment_id, '')) + LENGTH(COALESCE(local_path, ''))), 0) AS metadata_bytes,
  COALESCE(SUM(size_bytes), 0) AS local_bytes
FROM attachments
`); err != nil {
		return AttachmentCacheStats{}, fmt.Errorf("read attachment cache stats: %w", err)
	}
	return stats, nil
}

func (s CacheStatsService) fts(ctx context.Context) (FTSFootprintStats, error) {
	var stats FTSFootprintStats
	if err := s.db.GetContext(ctx, &stats, `
SELECT
  COUNT(*) AS row_count,
  COALESCE(SUM(LENGTH(COALESCE(subject, '')) + LENGTH(COALESCE(from_addr, '')) + LENGTH(COALESCE(to_addrs, '')) + LENGTH(COALESCE(snippet, '')) + LENGTH(COALESCE(body_plain, ''))), 0) AS bytes
FROM messages_fts
`); err != nil {
		return FTSFootprintStats{}, fmt.Errorf("read FTS cache footprint: %w", err)
	}
	return stats, nil
}

func (s CacheStatsService) rows(ctx context.Context) ([]TableRowCount, error) {
	tables := []string{
		"schema_version",
		"accounts",
		"labels",
		"threads",
		"messages",
		"message_labels",
		"attachments",
		"outbox",
		"sync_policies",
		"cache_exclusions",
		"message_annotations",
		"messages_fts",
	}
	rows := make([]TableRowCount, 0, len(tables))
	for _, table := range tables {
		var count int
		if err := s.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM "+table); err != nil {
			return nil, fmt.Errorf("count %s rows: %w", table, err)
		}
		rows = append(rows, TableRowCount{Table: table, Rows: count})
	}
	return rows, nil
}
