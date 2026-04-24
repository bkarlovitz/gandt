package ui

type CacheWipeResult struct {
	DatabaseFilesRemoved   int
	AttachmentFilesRemoved int
	AttachmentDeleteErrors []string
}

type CacheWipeStore interface {
	WipeCache() (CacheWipeResult, error)
}

type CacheWipeStoreFunc func() (CacheWipeResult, error)

func (fn CacheWipeStoreFunc) WipeCache() (CacheWipeResult, error) {
	return fn()
}
