package nearprod

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func updateTestBinary(version, platform string) []byte {
	identity, _ := json.Marshal(J{"service": "nearprod", "version": version, "marker": BinaryMarker, "language": "Go", "platform": platform})
	return []byte("#!/bin/sh\n# " + BinaryMarker + "\nif [ \"$1\" = \"--identity\" ]; then\n  printf '%s\\n' '" + string(identity) + "'\n  exit 0\nfi\nif [ \"$1\" = \"--version\" ]; then\n  printf '%s\\n' '" + version + "'\n  exit 0\nfi\nexit 2\n")
}

func updatePathDependentBinary(version, platform string) []byte {
	valid, _ := json.Marshal(J{"service": "nearprod", "version": version, "marker": BinaryMarker, "language": "Go", "platform": platform})
	invalid, _ := json.Marshal(J{"service": "nearprod", "version": "9.9.9", "marker": BinaryMarker, "language": "Go", "platform": platform})
	return []byte("#!/bin/sh\n# " + BinaryMarker + "\nif [ \"$1\" = \"--identity\" ]; then\n  case \"$0\" in\n    */.update-*/nearprod) printf '%s\\n' '" + string(valid) + "' ;;\n    *) printf '%s\\n' '" + string(invalid) + "' ;;\n  esac\n  exit 0\nfi\nexit 2\n")
}

func updateTestArchive(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var raw bytes.Buffer
	gz := gzip.NewWriter(&raw)
	tw := tar.NewWriter(gz)
	for name, data := range entries {
		mode := int64(0644)
		if name == "nearprod" {
			mode = 0755
		}
		must(t, tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(data)), Typeflag: tar.TypeReg}))
		_, err := tw.Write(data)
		must(t, err)
	}
	must(t, tw.Close())
	must(t, gz.Close())
	return raw.Bytes()
}

func updateTestArchiveEntries(t *testing.T, entries []struct {
	header tar.Header
	data   []byte
}) []byte {
	t.Helper()
	var raw bytes.Buffer
	gz := gzip.NewWriter(&raw)
	tw := tar.NewWriter(gz)
	for _, entry := range entries {
		header := entry.header
		header.Size = int64(len(entry.data))
		must(t, tw.WriteHeader(&header))
		if len(entry.data) > 0 {
			_, err := tw.Write(entry.data)
			must(t, err)
		}
	}
	must(t, tw.Close())
	must(t, gz.Close())
	return raw.Bytes()
}

func updateTestDigest(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func updateTestServer(t *testing.T, version, platform string, archive []byte, manifestHash string) (*httptest.Server, string) {
	t.Helper()
	archiveName := "nearprod_" + version + "_" + strings.ReplaceAll(platform, "/", "_") + ".tar.gz"
	archiveDigest := updateTestDigest(archive)
	manifest := []byte(manifestHash + "  " + archiveName + "\n")
	manifestDigest := updateTestDigest(manifest)
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			_ = json.NewEncoder(w).Encode(J{
				"tag_name": "v" + version, "html_url": server.URL + "/release", "draft": false, "prerelease": false,
				"assets": A{
					J{"name": archiveName, "browser_download_url": server.URL + "/download/v" + version + "/" + archiveName, "digest": "sha256:" + archiveDigest, "state": "uploaded", "size": len(archive)},
					J{"name": "SHA256SUMS.txt", "browser_download_url": server.URL + "/download/v" + version + "/SHA256SUMS.txt", "digest": "sha256:" + manifestDigest, "state": "uploaded", "size": len(manifest)},
				},
			})
		case "/download/v" + version + "/" + archiveName:
			_, _ = w.Write(archive)
		case "/download/v" + version + "/SHA256SUMS.txt":
			_, _ = w.Write(manifest)
		default:
			http.NotFound(w, r)
		}
	}))
	return server, archiveName
}

func TestStableVersionComparison(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{{"0.7.1", "0.7.2", -1}, {"1.0.0", "1.0.0", 0}, {"2.0.0", "1.99.99", 1}} {
		got, err := compareStableVersions(tc.a, tc.b)
		must(t, err)
		if got != tc.want {
			t.Fatalf("compareStableVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
	for _, invalid := range []string{"dev", "v1.0.0", "1.0", "01.0.0", "1.0.0-beta", "999999999999999999999.0.0"} {
		if _, err := parseStableVersion(invalid); err == nil {
			t.Fatalf("accepted invalid version %q", invalid)
		}
	}
}

func TestUpdateOriginProviderAndManifestGuards(t *testing.T) {
	u := &Updater{AllowedOrigins: map[string]bool{"https://api.github.com": true}}
	_, err := u.requestBytes(context.Background(), "https://example.com/release", "application/json", 100)
	expectCode(t, err, "UPDATE_URL")
	_, err = u.requestBytes(context.Background(), "https://api.github.com/\x00release", "application/json", 100)
	expectCode(t, err, "UPDATE_URL")
	if allowedUpdateHost("evil.githubusercontent.com") || allowedUpdateHost("github.com.example.org") {
		t.Fatal("redirect host allowlist is too broad")
	}
	client := newUpdateHTTPClient()
	redirect, _ := http.NewRequest(http.MethodGet, "https://github.com:444/file", nil)
	if err = client.CheckRedirect(redirect, nil); err == nil {
		t.Fatal("alternate HTTPS port accepted for redirect")
	}

	home := t.TempDir()
	manual := filepath.Join(home, ".local", "bin", "nearprod")
	must(t, os.MkdirAll(filepath.Dir(manual), 0700))
	must(t, os.WriteFile(manual, []byte(BinaryMarker), 0755))
	if got := updateProvider(manual, home); got != "manual" {
		t.Fatalf("manual provider = %q", got)
	}
	keg := filepath.Join(home, "brew", "Cellar", "nearprod", "1.0.0", "bin", "nearprod")
	must(t, os.MkdirAll(filepath.Dir(keg), 0700))
	must(t, os.WriteFile(keg, []byte(BinaryMarker), 0755))
	if got := updateProvider(keg, home); got != "homebrew" {
		t.Fatalf("Homebrew provider = %q", got)
	}
	if got := updateProvider(filepath.Join(home, "Downloads", "nearprod"), home); got != "unmanaged" {
		t.Fatalf("unmanaged provider = %q", got)
	}
	digest := strings.Repeat("a", 64)
	_, err = checksumEntry([]byte(digest+"  archive.tar.gz\n"+digest+"  archive.tar.gz\n"), "archive.tar.gz")
	expectCode(t, err, "UPDATE_CHECKSUM")
	_, err = checksumEntry([]byte(digest+"  another.tar.gz\n"), "archive.tar.gz")
	expectCode(t, err, "UPDATE_CHECKSUM")
}

func TestExtractUpdateBinaryRejectsUnsafeEntries(t *testing.T) {
	binary := updateTestBinary("1.1.0", runtimePlatform())
	regular := func(name string) struct {
		header tar.Header
		data   []byte
	} {
		return struct {
			header tar.Header
			data   []byte
		}{tar.Header{Name: name, Mode: 0755, Typeflag: tar.TypeReg}, binary}
	}
	for name, entries := range map[string][]struct {
		header tar.Header
		data   []byte
	}{
		"absolute":  {regular("/nearprod")},
		"backslash": {regular(`folder\nearprod`)},
		"duplicate": {regular("nearprod"), regular("nearprod")},
		"symlink":   {{header: tar.Header{Name: "nearprod", Mode: 0755, Typeflag: tar.TypeSymlink, Linkname: "target"}}},
		"hardlink":  {{header: tar.Header{Name: "nearprod", Mode: 0755, Typeflag: tar.TypeLink, Linkname: "target"}}},
	} {
		t.Run(name, func(t *testing.T) {
			archive := filepath.Join(t.TempDir(), "update.tar.gz")
			must(t, os.WriteFile(archive, updateTestArchiveEntries(t, entries), 0600))
			err := extractUpdateBinary(archive, filepath.Join(t.TempDir(), "nearprod"))
			expectCode(t, err, "UPDATE_ARCHIVE")
		})
	}
}

func TestUpdaterDownloadsVerifiesInstallsAndCleans(t *testing.T) {
	oldVersion := Version
	Version = "1.0.0"
	t.Cleanup(func() { Version = oldVersion })
	platform := runtimePlatform()
	archive := updateTestArchive(t, map[string][]byte{
		"nearprod":               updateTestBinary("1.1.0", platform),
		"LICENSE":                []byte("MIT\n"),
		"THIRD_PARTY_NOTICES.md": []byte("notices\n"),
	})
	server, _ := updateTestServer(t, "1.1.0", platform, archive, updateTestDigest(archive))
	defer server.Close()

	home := t.TempDir()
	installed := filepath.Join(home, ".local", "bin", "nearprod")
	must(t, os.MkdirAll(filepath.Dir(installed), 0700))
	previous := updateTestBinary("1.0.0", platform)
	must(t, os.WriteFile(installed, previous, 0755))
	u := &Updater{Client: server.Client(), APIURL: server.URL + "/latest", AssetBase: server.URL + "/download", Executable: installed, Home: home, OS: runtimeOS(), Arch: runtimeArch(), AllowedOrigins: map[string]bool{server.URL: true}}

	plan, err := u.Check(context.Background())
	must(t, err)
	if !plan.Available || plan.Provider != "manual" || !truth(plan.Summary["canInstall"]) {
		t.Fatal(plan.Summary)
	}
	result, err := u.Apply(context.Background(), plan)
	must(t, err)
	if str(result["status"]) != "updated" || str(result["currentVersion"]) != "1.1.0" || truth(result["canInstall"]) || !truth(result["temporaryFilesRemoved"]) {
		t.Fatal(result)
	}
	identity, err := verifyUpdateBinary(context.Background(), installed, "1.1.0", platform)
	must(t, err)
	if str(identity["version"]) != "1.1.0" {
		t.Fatal(identity)
	}
	backup, err := os.ReadFile(str(result["previous"]))
	must(t, err)
	if !bytes.Equal(backup, previous) {
		t.Fatal("previous executable was not preserved exactly")
	}
	record, err := readJSON(filepath.Join(home, ".local", "share", "nearprod", "installation.json"), 1<<20)
	must(t, err)
	if str(record["version"]) != "1.1.0" || !strings.Contains(str(record["release"]), string(filepath.Separator)+"1.1.0-") {
		t.Fatal(record)
	}
	residue, err := filepath.Glob(filepath.Join(home, ".local", "share", "nearprod", ".update-*"))
	must(t, err)
	if len(residue) != 0 {
		t.Fatal("temporary update files remain", residue)
	}
}

func TestUpdaterRejectsChecksumMismatchBeforeReplacing(t *testing.T) {
	oldVersion := Version
	Version = "1.0.0"
	t.Cleanup(func() { Version = oldVersion })
	platform := runtimePlatform()
	archive := updateTestArchive(t, map[string][]byte{"nearprod": updateTestBinary("1.1.0", platform)})
	server, _ := updateTestServer(t, "1.1.0", platform, archive, strings.Repeat("0", 64))
	defer server.Close()
	home := t.TempDir()
	installed := filepath.Join(home, ".local", "bin", "nearprod")
	must(t, os.MkdirAll(filepath.Dir(installed), 0700))
	before := updateTestBinary("1.0.0", platform)
	must(t, os.WriteFile(installed, before, 0755))
	u := &Updater{Client: server.Client(), APIURL: server.URL + "/latest", AssetBase: server.URL + "/download", Executable: installed, Home: home, OS: runtimeOS(), Arch: runtimeArch(), AllowedOrigins: map[string]bool{server.URL: true}}
	plan, err := u.Check(context.Background())
	must(t, err)
	_, err = u.Apply(context.Background(), plan)
	expectCode(t, err, "UPDATE_CHECKSUM")
	after, readErr := os.ReadFile(installed)
	must(t, readErr)
	if !bytes.Equal(after, before) {
		t.Fatal("checksum failure replaced the installed binary")
	}
}

func TestUpdaterRejectsArchiveTraversal(t *testing.T) {
	oldVersion := Version
	Version = "1.0.0"
	t.Cleanup(func() { Version = oldVersion })
	platform := runtimePlatform()
	archive := updateTestArchive(t, map[string][]byte{"../outside": []byte("bad"), "nearprod": updateTestBinary("1.1.0", platform)})
	server, _ := updateTestServer(t, "1.1.0", platform, archive, updateTestDigest(archive))
	defer server.Close()
	home := t.TempDir()
	installed := filepath.Join(home, ".local", "bin", "nearprod")
	must(t, os.MkdirAll(filepath.Dir(installed), 0700))
	before := updateTestBinary("1.0.0", platform)
	must(t, os.WriteFile(installed, before, 0755))
	u := &Updater{Client: server.Client(), APIURL: server.URL + "/latest", AssetBase: server.URL + "/download", Executable: installed, Home: home, OS: runtimeOS(), Arch: runtimeArch(), AllowedOrigins: map[string]bool{server.URL: true}}
	plan, err := u.Check(context.Background())
	must(t, err)
	_, err = u.Apply(context.Background(), plan)
	expectCode(t, err, "UPDATE_ARCHIVE")
	if _, statErr := os.Stat(filepath.Join(home, "outside")); !os.IsNotExist(statErr) {
		t.Fatal("archive wrote outside the temporary directory")
	}
	after, readErr := os.ReadFile(installed)
	must(t, readErr)
	if !bytes.Equal(after, before) {
		t.Fatal("unsafe archive replaced the installed binary")
	}
}

func TestUpdaterRollsBackWhenInstalledIdentityChanges(t *testing.T) {
	oldVersion := Version
	Version = "1.0.0"
	t.Cleanup(func() { Version = oldVersion })
	platform := runtimePlatform()
	archive := updateTestArchive(t, map[string][]byte{"nearprod": updatePathDependentBinary("1.1.0", platform)})
	server, _ := updateTestServer(t, "1.1.0", platform, archive, updateTestDigest(archive))
	defer server.Close()
	home := t.TempDir()
	installed := filepath.Join(home, ".local", "bin", "nearprod")
	must(t, os.MkdirAll(filepath.Dir(installed), 0700))
	before := updateTestBinary("1.0.0", platform)
	must(t, os.WriteFile(installed, before, 0755))
	recordFile := filepath.Join(home, ".local", "share", "nearprod", "installation.json")
	priorRecord := J{"version": "1.0.0", "bin": installed}
	must(t, writeJSON(recordFile, priorRecord))
	u := &Updater{Client: server.Client(), APIURL: server.URL + "/latest", AssetBase: server.URL + "/download", Executable: installed, Home: home, OS: runtimeOS(), Arch: runtimeArch(), AllowedOrigins: map[string]bool{server.URL: true}}
	plan, err := u.Check(context.Background())
	must(t, err)
	_, err = u.Apply(context.Background(), plan)
	expectCode(t, err, "UPDATE_BINARY")
	after, readErr := os.ReadFile(installed)
	must(t, readErr)
	if !bytes.Equal(after, before) {
		t.Fatal("post-install verification failure did not restore the previous binary")
	}
	record, readErr := readJSON(recordFile, 1<<20)
	must(t, readErr)
	if str(record["version"]) != "1.0.0" {
		t.Fatal("post-install verification failure changed installation metadata", record)
	}
	residue, globErr := filepath.Glob(filepath.Join(home, ".local", "share", "nearprod", ".update-*"))
	must(t, globErr)
	if len(residue) != 0 {
		t.Fatal("rollback left update temporary files", residue)
	}
}

func TestUpdateConfirmationDefaultsToNo(t *testing.T) {
	for input, want := range map[string]bool{"s\n": true, "yes\n": true, "N\n": false, "\n": false, "anything\n": false} {
		var output bytes.Buffer
		got, err := confirmUpdate(strings.NewReader(input), &output, "1.0.0", "1.1.0")
		must(t, err)
		if got != want || !strings.Contains(output.String(), "1.0.0 -> 1.1.0") {
			t.Fatalf("confirmation %q = %v, want %v; output=%q", input, got, want, output.String())
		}
	}
}

func runtimeOS() string {
	return strings.Split(runtimePlatform(), "/")[0]
}

func runtimeArch() string {
	return strings.Split(runtimePlatform(), "/")[1]
}

func runtimePlatform() string {
	return fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
}
