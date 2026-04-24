package config

import (
	"errors"
	"os"

	charmlog "github.com/charmbracelet/log"
)

type LogFile struct {
	file *os.File
}

func (l *LogFile) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	return l.file.Close()
}

// InitFileLogger routes the Charm logger to G&T's log file without touching stderr.
func InitFileLogger(paths Paths, version string) (*LogFile, error) {
	if paths.LogDir == "" || paths.LogFile == "" {
		return nil, errors.New("log path is empty")
	}
	if err := os.MkdirAll(paths.LogDir, 0o700); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(paths.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}

	logger := charmlog.NewWithOptions(file, charmlog.Options{
		ReportTimestamp: true,
		TimeFunction:    charmlog.NowUTC,
		Formatter:       charmlog.LogfmtFormatter,
	})
	charmlog.SetDefault(logger)
	logger.Info("startup",
		"version", version,
		"config_dir", paths.ConfigDir,
		"data_dir", paths.DataDir,
		"log_file", paths.LogFile,
	)

	return &LogFile{file: file}, nil
}
