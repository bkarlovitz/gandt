package ui

type SyncUpdateMsg struct {
	AccountID string
	Summary   string
	Err       error
	Stopped   bool
	Fallback  bool
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

type refreshDoneMsg struct {
	Request RefreshRequest
	Result  RefreshResult
	Err     error
}
