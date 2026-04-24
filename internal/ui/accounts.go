package ui

import "errors"

type AccountAddResult struct {
	Account string
	Labels  []Label
}

type AccountAdder interface {
	AddAccount() (AccountAddResult, error)
}

type AccountAdderFunc func() (AccountAddResult, error)

func (fn AccountAdderFunc) AddAccount() (AccountAddResult, error) {
	return fn()
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
	MessageID   string
	ThreadID    string
	Body        []string
	CacheState  string
	Attachments []Attachment
}

type ThreadLoader interface {
	LoadThread(ThreadLoadRequest) (ThreadLoadResult, error)
}

type ThreadLoaderFunc func(ThreadLoadRequest) (ThreadLoadResult, error)

func (fn ThreadLoaderFunc) LoadThread(request ThreadLoadRequest) (ThreadLoadResult, error) {
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
