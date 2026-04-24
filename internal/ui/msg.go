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

type triageDoneMsg struct {
	ID      int
	Request TriageActionRequest
	Result  TriageActionResult
	Err     error
}

type cacheDashboardDoneMsg struct {
	Result CacheDashboard
	Err    error
}

type cachePolicyLoadDoneMsg struct {
	Result CachePolicyTable
	Err    error
}

type cachePolicySaveDoneMsg struct {
	Row CachePolicyRow
	Err error
}

type cachePolicyResetDoneMsg struct {
	Row CachePolicyRow
	Err error
}

type cacheExclusionPreviewDoneMsg struct {
	Preview CacheExclusionPreview
	Err     error
}

type cacheExclusionConfirmDoneMsg struct {
	Result CacheExclusionResult
	Err    error
}

type cachePurgePreviewDoneMsg struct {
	Preview CachePurgePreview
	Err     error
}

type cachePurgeExecuteDoneMsg struct {
	Result CachePurgeResult
	Err    error
}

type cacheCompactDoneMsg struct {
	Err error
}

type cacheWipeDoneMsg struct {
	Result CacheWipeResult
	Err    error
}
