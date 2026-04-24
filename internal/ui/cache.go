package ui

import "time"

type CacheDashboard struct {
	GeneratedAt           time.Time
	SQLiteBytes           int64
	TotalBytes            int64
	MessageCount          int
	BodyCount             int
	AttachmentCount       int
	CachedAttachmentCount int
	MessageBytes          int64
	BodyBytes             int64
	AttachmentBytes       int64
	FTSBytes              int64
	FTSRows               int
	Accounts              []CacheDashboardAccount
	Labels                []CacheDashboardLabel
	Ages                  []CacheDashboardAge
	Rows                  []CacheDashboardRow
}

type CacheDashboardAccount struct {
	Email           string
	MessageCount    int
	BodyCount       int
	AttachmentCount int
	TotalBytes      int64
}

type CacheDashboardLabel struct {
	AccountEmail    string
	LabelID         string
	LabelName       string
	CacheDepth      string
	MessageCount    int
	BodyCount       int
	AttachmentCount int
	AttachmentBytes int64
	TotalBytes      int64
}

type CacheDashboardAge struct {
	Bucket          string
	MessageCount    int
	BodyCount       int
	AttachmentCount int
	TotalBytes      int64
}

type CacheDashboardRow struct {
	Table string
	Rows  int
}

type CacheDashboardLoader interface {
	LoadCacheDashboard() (CacheDashboard, error)
}

type CacheDashboardLoaderFunc func() (CacheDashboard, error)

func (fn CacheDashboardLoaderFunc) LoadCacheDashboard() (CacheDashboard, error) {
	return fn()
}
