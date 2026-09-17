package nearprod

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func brewFixture(t *testing.T, prefix string) (string, string) {
	t.Helper()
	must(t, os.MkdirAll(prefix, 0755))
	var err error
	prefix, err = filepath.EvalSymlinks(prefix)
	must(t, err)
	keg := filepath.Join(prefix, "Cellar", "nearprod", "0.7.0", "bin", "nearprod")
	must(t, os.MkdirAll(filepath.Dir(keg), 0755))
	must(t, os.WriteFile(keg, []byte(BinaryMarker), 0755))
	opt := filepath.Join(prefix, "opt", "nearprod")
	must(t, os.MkdirAll(filepath.Dir(opt), 0755))
	must(t, os.Symlink(filepath.Join(prefix, "Cellar", "nearprod", "0.7.0"), opt))
	return keg, filepath.Join(opt, "bin", "nearprod")
}

func TestHomebrewStableStartupPath(t *testing.T) {
	f := newFixture(t, false)
	prefix := filepath.Join(f.Dir, "brew")
	keg, stable := brewFixture(t, prefix)
	sf := &systemFake{Base: f.F}
	s := &Startup{Home: filepath.Join(f.Dir, "user"), CatalogHome: f.Home, Platform: "darwin", UID: 501, Runner: sf, Executable: keg}
	got, err := s.startupBinary()
	must(t, err)
	if got != stable {
		t.Fatalf("got %s want %s", got, stable)
	}
	_, err = s.Enable(context.Background())
	must(t, err)
	data, err := os.ReadFile(s.File())
	must(t, err)
	stableArgument := "<key>ProgramArguments</key><array><string>" + stable + "</string>"
	kegArgument := "<key>ProgramArguments</key><array><string>" + keg + "</string>"
	if !strings.Contains(string(data), stableArgument) || strings.Contains(string(data), kegArgument) {
		t.Fatalf("unstable plist: %s", data)
	}
	if _, err := os.Stat(filepath.Join(s.Home, ".local", "bin", "nearprod")); !os.IsNotExist(err) {
		t.Fatal("Homebrew startup created a manual installation")
	}
}

func TestHomebrewInstallerRefusesSecondInstallation(t *testing.T) {
	dir := t.TempDir()
	keg, _ := brewFixture(t, filepath.Join(dir, "brew"))
	home := filepath.Join(dir, "home")
	_, err := InstallBinary(home, keg, true)
	expectCode(t, err, "PACKAGE_MANAGED")
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatal("installer changed HOME")
	}
}

func TestHomebrewBrokenOptAndForeignBinaryAreRejected(t *testing.T) {
	dir := t.TempDir()
	keg, stable := brewFixture(t, filepath.Join(dir, "brew"))
	s := &Startup{Home: dir, Executable: keg}
	must(t, os.Remove(filepath.Dir(filepath.Dir(stable))))
	_, err := s.startupBinary()
	expectCode(t, err, "HOMEBREW_LINK")
	if _, managed := homebrewPrefix(filepath.Join(dir, "Downloads", "nearprod")); managed {
		t.Fatal("a download was recognized as a Homebrew keg")
	}
}

func TestUnsafeInstallDirectoryIsRejected(t *testing.T) {
	unsafe := filepath.Join(t.TempDir(), "unsafe")
	must(t, os.Mkdir(unsafe, 0777))
	must(t, os.Chmod(unsafe, 0777))
	err := safeOwnedDir(filepath.Join(unsafe, "bin"))
	expectCode(t, err, "DIRECTORY_PERMISSIONS")
}
