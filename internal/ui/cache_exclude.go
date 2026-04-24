package ui

type CacheExclusionRequest struct {
	Account    string
	MatchType  string
	MatchValue string
}

type CacheExclusionPreview struct {
	Request         CacheExclusionRequest
	MessageCount    int
	BodyCount       int
	AttachmentCount int
	EstimatedBytes  int64
}

type CacheExclusionResult struct {
	Preview                CacheExclusionPreview
	DeletedMessages        int
	DeletedAttachmentFiles int
	AttachmentDeleteErrors []string
}

type CacheExclusionStore interface {
	PreviewCacheExclusion(CacheExclusionRequest) (CacheExclusionPreview, error)
	ConfirmCacheExclusion(CacheExclusionRequest) (CacheExclusionResult, error)
}

type CacheExclusionStoreFunc struct {
	PreviewFn func(CacheExclusionRequest) (CacheExclusionPreview, error)
	ConfirmFn func(CacheExclusionRequest) (CacheExclusionResult, error)
}

func (fn CacheExclusionStoreFunc) PreviewCacheExclusion(request CacheExclusionRequest) (CacheExclusionPreview, error) {
	return fn.PreviewFn(request)
}

func (fn CacheExclusionStoreFunc) ConfirmCacheExclusion(request CacheExclusionRequest) (CacheExclusionResult, error) {
	return fn.ConfirmFn(request)
}

func validCacheExclusionType(matchType string) bool {
	switch matchType {
	case "sender", "domain", "label":
		return true
	default:
		return false
	}
}
