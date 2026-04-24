package compose

import "time"

const DefaultAutosaveInterval = 30 * time.Second

type Autosaver struct {
	Interval time.Duration
	LastSave time.Time
	Dirty    bool
}

func NewAutosaver(interval time.Duration) Autosaver {
	if interval <= 0 {
		interval = DefaultAutosaveInterval
	}
	return Autosaver{Interval: interval}
}

func (a Autosaver) MarkDirty() Autosaver {
	a.Dirty = true
	return a
}

func (a Autosaver) MarkSaved(at time.Time) Autosaver {
	a.Dirty = false
	a.LastSave = at
	return a
}

func (a Autosaver) Due(now time.Time) bool {
	if !a.Dirty {
		return false
	}
	if a.LastSave.IsZero() {
		return true
	}
	return !now.Before(a.LastSave.Add(a.Interval))
}
