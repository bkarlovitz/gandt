package compose

import (
	"errors"
	"fmt"
	"net/mail"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrAccountRequired   = errors.New("active account is required")
	ErrSendAsRequired    = errors.New("send-as address is required")
	ErrRecipientRequired = errors.New("at least one recipient is required")
)

type Address struct {
	Name  string
	Email string
}

func NewAddress(email string) Address {
	parsed, err := mail.ParseAddress(strings.TrimSpace(email))
	if err != nil {
		return Address{Email: strings.TrimSpace(email)}
	}
	return Address{Name: parsed.Name, Email: parsed.Address}
}

func (a Address) String() string {
	if strings.TrimSpace(a.Name) == "" {
		return strings.TrimSpace(a.Email)
	}
	return (&mail.Address{Name: strings.TrimSpace(a.Name), Address: strings.TrimSpace(a.Email)}).String()
}

func (a Address) normalizedEmail() string {
	return strings.ToLower(strings.TrimSpace(a.Email))
}

func (a Address) Validate() error {
	email := strings.TrimSpace(a.Email)
	if email == "" {
		return errors.New("email address is required")
	}
	parsed, err := mail.ParseAddress(a.String())
	if err != nil {
		return fmt.Errorf("invalid email address %q: %w", email, err)
	}
	if parsed.Address != email {
		return fmt.Errorf("invalid email address %q", email)
	}
	return nil
}

func ParseAddressList(input string) ([]Address, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}
	parsed, err := mail.ParseAddressList(input)
	if err != nil {
		return nil, err
	}
	addresses := make([]Address, 0, len(parsed))
	for _, addr := range parsed {
		addresses = append(addresses, Address{Name: addr.Name, Email: addr.Address})
	}
	return addresses, nil
}

type Headers struct {
	ActiveAccountID string
	AccountEmail    string
	SendAs          Address
	To              []Address
	Cc              []Address
	Bcc             []Address
	Subject         string
}

func (h Headers) Recipients() []Address {
	total := len(h.To) + len(h.Cc) + len(h.Bcc)
	recipients := make([]Address, 0, total)
	recipients = append(recipients, h.To...)
	recipients = append(recipients, h.Cc...)
	recipients = append(recipients, h.Bcc...)
	return recipients
}

func (h Headers) ValidateDraft() error {
	if strings.TrimSpace(h.ActiveAccountID) == "" {
		return ErrAccountRequired
	}
	if err := h.SendAs.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrSendAsRequired, err)
	}
	if err := validateAddresses("to", h.To); err != nil {
		return err
	}
	if err := validateAddresses("cc", h.Cc); err != nil {
		return err
	}
	if err := validateAddresses("bcc", h.Bcc); err != nil {
		return err
	}
	return nil
}

func (h Headers) ValidateForSend() error {
	if err := h.ValidateDraft(); err != nil {
		return err
	}
	if len(h.Recipients()) == 0 {
		return ErrRecipientRequired
	}
	return nil
}

func validateAddresses(field string, addresses []Address) error {
	for i, address := range addresses {
		if err := address.Validate(); err != nil {
			return fmt.Errorf("%s[%d]: %w", field, i, err)
		}
	}
	return nil
}

type BodyMode string

const (
	BodyModeExternal BodyMode = "external"
	BodyModeInline   BodyMode = "inline"
)

type BodySource struct {
	Mode      BodyMode
	PlainText string
	HTML      string
}

type Attachment struct {
	Path         string
	Filename     string
	MimeType     string
	SizeBytes    int64
	AttachmentID string
	Inline       bool
	ContentID    string
}

func NewAttachment(path string, sizeBytes int64, mimeType string) Attachment {
	filename := filepath.Base(path)
	if filename == "." || filename == string(filepath.Separator) {
		filename = ""
	}
	return Attachment{
		Path:      path,
		Filename:  filename,
		MimeType:  strings.TrimSpace(mimeType),
		SizeBytes: sizeBytes,
	}
}

func (a Attachment) ValidateMetadata() error {
	if strings.TrimSpace(a.Filename) == "" {
		return errors.New("attachment filename is required")
	}
	if a.SizeBytes < 0 {
		return errors.New("attachment size cannot be negative")
	}
	return nil
}

type DraftID struct {
	GmailDraftID   string
	GmailMessageID string
	ThreadID       string
}

func (id DraftID) Empty() bool {
	return strings.TrimSpace(id.GmailDraftID) == "" && strings.TrimSpace(id.GmailMessageID) == "" && strings.TrimSpace(id.ThreadID) == ""
}

type ComposeKind string

const (
	ComposeKindNew      ComposeKind = "new"
	ComposeKindReply    ComposeKind = "reply"
	ComposeKindReplyAll ComposeKind = "reply_all"
	ComposeKindForward  ComposeKind = "forward"
)

type OriginalMessage struct {
	AccountID string
	MessageID string
	ThreadID  string
	From      Address
	To        []Address
	Cc        []Address
	Subject   string
	Date      time.Time
	BodyPlain string
}

type ReplyContext struct {
	Kind     ComposeKind
	Original OriginalMessage
	Self     []Address
}

func NewReplyContext(original OriginalMessage, self Address, replyAll bool) ReplyContext {
	kind := ComposeKindReply
	if replyAll {
		kind = ComposeKindReplyAll
	}
	return ReplyContext{Kind: kind, Original: original, Self: []Address{self}}
}

func (c ReplyContext) Recipients() []Address {
	recipients := []Address{c.Original.From}
	if c.Kind == ComposeKindReplyAll {
		recipients = append(recipients, c.Original.To...)
		recipients = append(recipients, c.Original.Cc...)
	}
	return withoutSelf(dedupeAddresses(recipients), c.Self)
}

func (c ReplyContext) Subject() string {
	return prefixSubject("Re:", c.Original.Subject)
}

type ForwardContext struct {
	Original OriginalMessage
}

func NewForwardContext(original OriginalMessage) ForwardContext {
	return ForwardContext{Original: original}
}

func (c ForwardContext) Subject() string {
	return prefixSubject("Fwd:", c.Original.Subject)
}

type SendStatus string

const (
	SendStatusEditing     SendStatus = "editing"
	SendStatusSavingDraft SendStatus = "saving_draft"
	SendStatusDraftSaved  SendStatus = "draft_saved"
	SendStatusSending     SendStatus = "sending"
	SendStatusQueued      SendStatus = "queued"
	SendStatusSent        SendStatus = "sent"
	SendStatusFailed      SendStatus = "failed"
)

type SendState struct {
	Status      SendStatus
	Attempts    int
	LastError   string
	NextRetryAt *time.Time
	SentAt      *time.Time
}

type Draft struct {
	Kind        ComposeKind
	Headers     Headers
	Body        BodySource
	Attachments []Attachment
	DraftID     DraftID
	Reply       *ReplyContext
	Forward     *ForwardContext
	SendState   SendState
}

func (d Draft) ValidateForSend() error {
	if err := d.Headers.ValidateForSend(); err != nil {
		return err
	}
	for i, attachment := range d.Attachments {
		if err := attachment.ValidateMetadata(); err != nil {
			return fmt.Errorf("attachment[%d]: %w", i, err)
		}
	}
	return nil
}

func prefixSubject(prefix string, subject string) string {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return prefix
	}
	lower := strings.ToLower(subject)
	if strings.HasPrefix(lower, strings.ToLower(prefix)) {
		return subject
	}
	return prefix + " " + subject
}

func dedupeAddresses(addresses []Address) []Address {
	seen := map[string]bool{}
	out := make([]Address, 0, len(addresses))
	for _, address := range addresses {
		key := address.normalizedEmail()
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, address)
	}
	return out
}

func withoutSelf(addresses []Address, self []Address) []Address {
	selfSet := map[string]bool{}
	for _, address := range self {
		if key := address.normalizedEmail(); key != "" {
			selfSet[key] = true
		}
	}
	out := make([]Address, 0, len(addresses))
	for _, address := range addresses {
		if !selfSet[address.normalizedEmail()] {
			out = append(out, address)
		}
	}
	return out
}
