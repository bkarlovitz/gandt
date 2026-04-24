package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

const appDirName = "gandt"

// Paths contains the on-disk locations owned by G&T.
type Paths struct {
	ConfigDir     string
	ConfigFile    string
	DataDir       string
	AttachmentDir string
	LogDir        string
	LogFile       string
}

// DefaultPaths resolves G&T's config and data paths for the current platform.
func DefaultPaths() (Paths, error) {
	return resolvePaths(runtime.GOOS, os.Getenv, os.UserHomeDir)
}

// EnsureDirs creates the directories needed on startup with private permissions.
func EnsureDirs(paths Paths) error {
	for _, dir := range []string{
		paths.ConfigDir,
		paths.DataDir,
		paths.AttachmentDir,
		paths.LogDir,
	} {
		if dir == "" {
			return errors.New("config path is empty")
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}

	return nil
}

func resolvePaths(goos string, getenv func(string) string, homeDir func() (string, error)) (Paths, error) {
	home, err := homeDir()
	if err != nil {
		return Paths{}, err
	}
	if home == "" {
		return Paths{}, errors.New("home directory is empty")
	}

	configBase := getenv("XDG_CONFIG_HOME")
	dataBase := getenv("XDG_DATA_HOME")

	if configBase == "" {
		configBase = defaultConfigBase(goos, getenv, home)
	}
	if dataBase == "" {
		dataBase = defaultDataBase(goos, getenv, home)
	}

	paths := Paths{
		ConfigDir: filepath.Join(configBase, appDirName),
		DataDir:   filepath.Join(dataBase, appDirName),
	}
	paths.ConfigFile = filepath.Join(paths.ConfigDir, "config.toml")
	paths.AttachmentDir = filepath.Join(paths.DataDir, "attachments")
	paths.LogDir = filepath.Join(paths.DataDir, "logs")
	paths.LogFile = filepath.Join(paths.LogDir, "gandt.log")

	return paths, nil
}

func defaultConfigBase(goos string, getenv func(string) string, home string) string {
	if goos == "windows" {
		if appData := getenv("APPDATA"); appData != "" {
			return appData
		}
		return filepath.Join(home, "AppData", "Roaming")
	}

	return filepath.Join(home, ".config")
}

func defaultDataBase(goos string, getenv func(string) string, home string) string {
	if goos == "windows" {
		if localAppData := getenv("LOCALAPPDATA"); localAppData != "" {
			return localAppData
		}
		return filepath.Join(home, "AppData", "Local")
	}

	return filepath.Join(home, ".local", "share")
}
