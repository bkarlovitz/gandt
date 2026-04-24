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
