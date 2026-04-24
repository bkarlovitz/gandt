package ui

import (
	"context"
	"errors"

	"github.com/bkarlovitz/gandt/internal/compose"
)

type AccountAddResult struct {
	Account         string
	DisplayName     string
	Color           string
	Labels          []Label
	MessagesByLabel map[string][]Message
}

type AccountState struct {
	Account     string
	DisplayName string
	Color       string
	SyncStatus  string
	Unread      int
	Mailbox     Mailbox
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
	BodyHTML       string
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

type BrowserOpener interface {
	OpenMessage(account string, message Message) error
}

type BrowserOpenerFunc func(account string, message Message) error

func (fn BrowserOpenerFunc) OpenMessage(account string, message Message) error {
	return fn(account, message)
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

type SearchRequest struct {
	Account string
	Query   string
	Mode    SearchMode
	Limit   int
}

type SearchResult struct {
	Account  string
	Query    string
	Mode     SearchMode
	Messages []Message
}

type SearchRunner interface {
	Search(context.Context, SearchRequest) (SearchResult, error)
}

type SearchRunnerFunc func(context.Context, SearchRequest) (SearchResult, error)

func (fn SearchRunnerFunc) Search(ctx context.Context, request SearchRequest) (SearchResult, error) {
	return fn(ctx, request)
}

type RecentSearch struct {
	Account  string
	Query    string
	Mode     SearchMode
	LastUsed string
}

type RecentSearchStore interface {
	ListRecentSearches(account string, limit int) ([]RecentSearch, error)
	DeleteRecentSearch(account string, query string, mode SearchMode) error
}

type RecentSearchStoreFunc struct {
	ListFn   func(account string, limit int) ([]RecentSearch, error)
	DeleteFn func(account string, query string, mode SearchMode) error
}

func (fn RecentSearchStoreFunc) ListRecentSearches(account string, limit int) ([]RecentSearch, error) {
	return fn.ListFn(account, limit)
}

func (fn RecentSearchStoreFunc) DeleteRecentSearch(account string, query string, mode SearchMode) error {
	return fn.DeleteFn(account, query, mode)
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

type ComposeOperation string

const (
	ComposeOperationSaveDraft ComposeOperation = "save_draft"
	ComposeOperationSend      ComposeOperation = "send"
)

type ComposeRequest struct {
	Account string
	Draft   compose.Draft
}

type ComposeResult struct {
	Operation ComposeOperation
	Status    compose.SendStatus
	DraftID   compose.DraftID
	Summary   string
}

type ComposeActor interface {
	SaveDraft(ComposeRequest) (ComposeResult, error)
	Send(ComposeRequest) (ComposeResult, error)
}

type ComposeActorFunc struct {
	SaveDraftFn func(ComposeRequest) (ComposeResult, error)
	SendFn      func(ComposeRequest) (ComposeResult, error)
}

func (fn ComposeActorFunc) SaveDraft(request ComposeRequest) (ComposeResult, error) {
	if fn.SaveDraftFn == nil {
		return ComposeResult{}, errors.New("save draft unavailable")
	}
	return fn.SaveDraftFn(request)
}

func (fn ComposeActorFunc) Send(request ComposeRequest) (ComposeResult, error) {
	if fn.SendFn == nil {
		return ComposeResult{}, errors.New("send unavailable")
	}
	return fn.SendFn(request)
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
