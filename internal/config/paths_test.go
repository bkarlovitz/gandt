package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePathsUsesXDGOverrides(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "home", "test")
	paths, err := resolvePaths("linux", fakeEnv(map[string]string{
		"XDG_CONFIG_HOME": filepath.Join(home, "xdg-config"),
		"XDG_DATA_HOME":   filepath.Join(home, "xdg-data"),
	}), fakeHome(home))
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}

	assertEqual(t, paths.ConfigDir, filepath.Join(home, "xdg-config", "gandt"))
	assertEqual(t, paths.ConfigFile, filepath.Join(home, "xdg-config", "gandt", "config.toml"))
	assertEqual(t, paths.DataDir, filepath.Join(home, "xdg-data", "gandt"))
	assertEqual(t, paths.AttachmentDir, filepath.Join(home, "xdg-data", "gandt", "attachments"))
	assertEqual(t, paths.LogDir, filepath.Join(home, "xdg-data", "gandt", "logs"))
	assertEqual(t, paths.LogFile, filepath.Join(home, "xdg-data", "gandt", "logs", "gandt.log"))
}

func TestResolvePathsUsesUnixDefaults(t *testing.T) {
	home := filepath.Join(string(filepath.Separator), "Users", "test")
	paths, err := resolvePaths("darwin", fakeEnv(nil), fakeHome(home))
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}

	assertEqual(t, paths.ConfigDir, filepath.Join(home, ".config", "gandt"))
	assertEqual(t, paths.DataDir, filepath.Join(home, ".local", "share", "gandt"))
}

func TestResolvePathsUsesWindowsDefaults(t *testing.T) {
	home := `C:\Users\test`
	paths, err := resolvePaths("windows", fakeEnv(map[string]string{
		"APPDATA":      `C:\Users\test\AppData\Roaming`,
		"LOCALAPPDATA": `C:\Users\test\AppData\Local`,
	}), fakeHome(home))
	if err != nil {
		t.Fatalf("resolve paths: %v", err)
	}

	assertEqual(t, paths.ConfigDir, filepath.Join(`C:\Users\test\AppData\Roaming`, "gandt"))
	assertEqual(t, paths.DataDir, filepath.Join(`C:\Users\test\AppData\Local`, "gandt"))
}

func TestEnsureDirsCreatesPrivateDirectories(t *testing.T) {
	root := t.TempDir()
	paths := Paths{
		ConfigDir:     filepath.Join(root, "config"),
		DataDir:       filepath.Join(root, "data"),
		AttachmentDir: filepath.Join(root, "data", "attachments"),
		LogDir:        filepath.Join(root, "data", "logs"),
	}

	if err := EnsureDirs(paths); err != nil {
		t.Fatalf("ensure dirs: %v", err)
	}

	for _, dir := range []string{paths.ConfigDir, paths.DataDir, paths.AttachmentDir, paths.LogDir} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("stat %s: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("%s is not a directory", dir)
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("%s permissions are too broad: %s", dir, info.Mode().Perm())
		}
	}
}

func fakeEnv(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}

func fakeHome(home string) func() (string, error) {
	return func() (string, error) {
		return home, nil
	}
}

func assertEqual(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
