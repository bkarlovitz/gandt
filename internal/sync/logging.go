package sync

type Logger interface {
	LogSyncEvent(event string, fields map[string]any)
}

type LoggerFunc func(string, map[string]any)

func (fn LoggerFunc) LogSyncEvent(event string, fields map[string]any) {
	fn(event, fields)
}

type noopLogger struct{}

func (noopLogger) LogSyncEvent(string, map[string]any) {}

type DeltaOption func(*DeltaSynchronizer)

func WithLogger(logger Logger) DeltaOption {
	return func(s *DeltaSynchronizer) {
		if logger != nil {
			s.logger = logger
		}
	}
}
