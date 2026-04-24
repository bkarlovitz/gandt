package ui

// SyncUpdateMsg is reserved for background service updates once Gmail sync exists.
type SyncUpdateMsg struct {
	AccountID string
	Summary   string
}

// ErrorMsg carries non-fatal errors back to the root Bubble Tea model.
type ErrorMsg struct {
	Err error
}

type addAccountDoneMsg struct {
	Result AccountAddResult
	Err    error
}

type replaceCredentialsDoneMsg struct {
	Err error
}

type threadLoadDoneMsg struct {
	Result ThreadLoadResult
	Err    error
}
