package ui

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
