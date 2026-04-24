package ui

import "errors"

type AccountAddResult struct {
	Account         string
	Labels          []Label
	MessagesByLabel map[string][]Message
}

type AccountAdder interface {
	AddAccount() (AccountAddResult, error)
}

type AccountAdderFunc func() (AccountAddResult, error)

func (fn AccountAdderFunc) AddAccount() (AccountAddResult, error) {
	return fn()
}

type AccountRemoveResult struct {
	Account     string
	RevokeError bool
}

type AccountRemover interface {
	RemoveAccount(account string) (AccountRemoveResult, error)
}

type AccountRemoverFunc func(account string) (AccountRemoveResult, error)

func (fn AccountRemoverFunc) RemoveAccount(account string) (AccountRemoveResult, error) {
	return fn(account)
}

type CredentialReplacer interface {
	ReplaceCredentials() error
}

type CredentialReplacerFunc func() error

func (fn CredentialReplacerFunc) ReplaceCredentials() error {
	return fn()
}

type ThreadLoadRequest struct {
	Account string
	Message Message
}

type ThreadLoadResult struct {
	MessageID      string
	ThreadID       string
	Body           []string
	CacheState     string
	Attachments    []Attachment
	ThreadMessages []ThreadMessage
}

type ThreadLoader interface {
	LoadThread(ThreadLoadRequest) (ThreadLoadResult, error)
}

type ThreadLoaderFunc func(ThreadLoadRequest) (ThreadLoadResult, error)

func (fn ThreadLoaderFunc) LoadThread(request ThreadLoadRequest) (ThreadLoadResult, error) {
	return fn(request)
}

type RefreshKind string

const (
	RefreshDelta       RefreshKind = "delta"
	RefreshRelistLabel RefreshKind = "relist-label"
	RefreshAll         RefreshKind = "all"
)

type RefreshRequest struct {
	Kind      RefreshKind
	Account   string
	LabelID   string
	LabelName string
}

type RefreshResult struct {
	Summary string
}

type ManualRefresher interface {
	Refresh(RefreshRequest) (RefreshResult, error)
}

type ManualRefresherFunc func(RefreshRequest) (RefreshResult, error)

func (fn ManualRefresherFunc) Refresh(request RefreshRequest) (RefreshResult, error) {
	return fn(request)
}

type TriageActionKind string

const (
	TriageArchive     TriageActionKind = "archive"
	TriageTrash       TriageActionKind = "trash"
	TriageUntrash     TriageActionKind = "untrash"
	TriageSpam        TriageActionKind = "spam"
	TriageUnspam      TriageActionKind = "unspam"
	TriageStar        TriageActionKind = "star"
	TriageUnread      TriageActionKind = "unread"
	TriageLabelAdd    TriageActionKind = "label-add"
	TriageLabelRemove TriageActionKind = "label-remove"
	TriageMute        TriageActionKind = "mute"
)

type TriageActionRequest struct {
	Kind        TriageActionKind
	Account     string
	MessageID   string
	ThreadID    string
	LabelID     string
	LabelName   string
	Add         bool
	Undo        bool
	CreateLabel bool
}

type TriageActionResult struct {
	Summary   string
	LabelID   string
	LabelName string
}

type TriageActor interface {
	ApplyAction(TriageActionRequest) (TriageActionResult, error)
}

type TriageActorFunc func(TriageActionRequest) (TriageActionResult, error)

func (fn TriageActorFunc) ApplyAction(request TriageActionRequest) (TriageActionResult, error) {
	return fn(request)
}

type OfflineError struct {
	Err error
}

func (err OfflineError) Error() string {
	if err.Err == nil {
		return "offline"
	}
	return err.Err.Error()
}

func (err OfflineError) Unwrap() error {
	return err.Err
}

func (err OfflineError) Offline() bool {
	return true
}

func MarkOffline(err error) error {
	return OfflineError{Err: err}
}

func IsOfflineError(err error) bool {
	var offline interface{ Offline() bool }
	return errors.As(err, &offline) && offline.Offline()
}

type FatalError struct {
	Err error
}

func (err FatalError) Error() string {
	if err.Err == nil {
		return "fatal error"
	}
	return err.Err.Error()
}

func (err FatalError) Unwrap() error {
	return err.Err
}

func (err FatalError) Fatal() bool {
	return true
}

func MarkFatal(err error) error {
	return FatalError{Err: err}
}

func IsFatalError(err error) bool {
	var fatal interface{ Fatal() bool }
	return errors.As(err, &fatal) && fatal.Fatal()
}
