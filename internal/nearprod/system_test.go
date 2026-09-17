package nearprod

import (
	"context"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type systemFake struct {
	Base                               *fakeRunner
	Prefix                             string
	Calls                              []fakeCall
	Loaded, Pinned, Fail, ChangeConfig bool
}

func (s *systemFake) Run(ctx context.Context, n string, a []string, o RunOptions) (Result, error) {
	s.Calls = append(s.Calls, fakeCall{Name: n, Args: append([]string{}, a...)})
	switch filepath.Base(n) {
	case "brew":
		if contains(a, "info") {
			name := a[len(a)-1]
			return fakeOK(J{"formulae": A{J{"name": name, "versions": J{"stable": "2.40.0"}, "versioned_formulae": A{}, "installed": A{J{"version": "2.40.0"}}, "pinned": s.Pinned, "dependencies": A{}, "keg_only": false}}})
		}
		if contains(a, "--prefix") {
			return fakeOK(s.Prefix)
		}
		if contains(a, "install") || contains(a, "upgrade") {
			if s.Fail {
				return fakeFail("install failed")
			}
			if s.ChangeConfig {
				writeJSON(dockerConfigPath(), J{"changed": true})
			}
			return fakeOK("")
		}
	case "launchctl":
		if s.Fail {
			return fakeFail("permission failure")
		}
		if contains(a, "print") {
			if !s.Loaded {
				return fakeFail("not loaded")
			}
			return fakeOK("running")
		}
		if contains(a, "bootstrap") {
			s.Loaded = true
		}
		return fakeOK("")
	}
	return s.Base.Run(ctx, n, a, o)
}
func TestHomebrewRepairPreservesConfigAndDetectsManagers(t *testing.T) {
	f := newFixture(t, false)
	prefix := filepath.Join(f.Dir, "brew")
	os.MkdirAll(filepath.Join(prefix, "bin"), 0700)
	os.WriteFile(filepath.Join(prefix, "bin", "brew"), []byte("#!/bin/sh\nexit 0\n"), 0700)
	t.Setenv("PATH", filepath.Join(prefix, "bin")+":"+os.Getenv("PATH"))
	t.Setenv("DOCKER_CONFIG", filepath.Join(f.Dir, "docker"))
	config := J{"auths": J{"registry.example": J{"auth": "secret"}}, "currentContext": "original", "cliPluginsExtraDirs": A{"/existing"}}
	must(t, writeJSON(dockerConfigPath(), config))
	sf := &systemFake{Base: f.F, Prefix: prefix}
	m := &ToolsManager{Runner: sf, Store: f.S.Store, Runtime: f.S.Runtime, Platform: "darwin"}
	req := J{"tool": "compose", "action": "repair"}
	p, e := m.Preview(context.Background(), req)
	must(t, e)
	req["confirm"] = true
	req["fingerprint"] = p["fingerprint"]
	v, e := m.Apply(context.Background(), req, nil)
	must(t, e)
	if !truth(v["verified"]) {
		t.Fatal(v)
	}
	after, e := readJSON(dockerConfigPath(), 1<<20)
	must(t, e)
	if after["currentContext"] != "original" || str(at(after, "auths", "registry.example", "auth")) != "secret" || len(arr(after["cliPluginsExtraDirs"])) != 2 {
		t.Fatal("config overwritten")
	}
	if str(v["configBackup"]) == "" {
		t.Fatal("no backup")
	}
	p, e = m.Preview(context.Background(), req)
	must(t, e)
	req["fingerprint"] = p["fingerprint"]
	_, e = m.Apply(context.Background(), req, nil)
	must(t, e)
	after, _ = readJSON(dockerConfigPath(), 1<<20)
	if len(arr(after["cliPluginsExtraDirs"])) != 2 {
		t.Fatal("duplicate plugins")
	}
	for _, p := range []struct{ path, want string }{{"/Users/a/.local/share/fnm/node-versions/v24/bin/node", "fnm"}, {"/opt/nvm/versions/node/v22/bin/node", "nvm"}, {"/Users/a/.volta/tools/image/node", "volta"}, {"/Users/a/.local/share/mise/installs/node", "mise"}} {
		if nodeManager(p.path) != p.want {
			t.Fatal(p)
		}
	}
	for _, c := range sf.Calls {
		if contains(c.Args, "upgrade") || contains(c.Args, "install") || contains(c.Args, "unpin") {
			t.Fatal("repair changed packages")
		}
	}
	sf.Pinned = true
	_, e = m.Preview(context.Background(), J{"tool": "compose", "action": "upgrade"})
	expectCode(t, e, "FORMULA_PINNED")
	_, e = m.Preview(context.Background(), J{"tool": "compose", "formula": "malicious/tap/x"})
	expectCode(t, e, "VERSION_UNAVAILABLE")
}
func TestToolInstallGuardsAndPostDownloadRace(t *testing.T) {
	f := newFixture(t, false)
	prefix := filepath.Join(f.Dir, "brew")
	os.MkdirAll(filepath.Join(prefix, "bin"), 0700)
	os.WriteFile(filepath.Join(prefix, "bin", "brew"), []byte("#!/bin/sh\n"), 0700)
	t.Setenv("PATH", filepath.Join(prefix, "bin")+":"+os.Getenv("PATH"))
	t.Setenv("DOCKER_CONFIG", filepath.Join(f.Dir, "dc"))
	writeJSON(dockerConfigPath(), J{"auths": J{}})
	sf := &systemFake{Base: f.F, Prefix: prefix}
	m := &ToolsManager{Runner: sf, Store: f.S.Store, Runtime: f.S.Runtime, Platform: "darwin"}
	req := J{"tool": "compose", "action": "install"}
	p, e := m.Preview(context.Background(), req)
	must(t, e)
	req["confirm"] = true
	req["fingerprint"] = p["fingerprint"]
	sf.ChangeConfig = true
	_, e = m.Apply(context.Background(), req, nil)
	expectCode(t, e, "DOCKER_CONFIG_CHANGED")
	doc, _ := readJSON(dockerConfigPath(), 1<<20)
	if !truth(doc["changed"]) {
		t.Fatal("concurrent config lost")
	}
	m.Platform = "linux"
	_, e = m.Versions(context.Background(), "compose")
	expectCode(t, e, "PACKAGE_PLATFORM")
}
func TestStartupNativeBinaryAndForeignPlistProtection(t *testing.T) {
	f := newFixture(t, false)
	home := filepath.Join(f.Dir, "user")
	bin := filepath.Join(home, ".local", "bin", "nearprod")
	os.MkdirAll(filepath.Dir(bin), 0700)
	os.WriteFile(bin, []byte(BinaryMarker), 0755)
	sf := &systemFake{Base: f.F}
	st := &Startup{Home: home, CatalogHome: f.Home, Platform: "darwin", UID: 501, Runner: sf, Executable: bin}
	v, e := st.Enable(context.Background())
	must(t, e)
	if !truth(v["enabled"]) || !sf.Loaded {
		t.Fatal(v)
	}
	data, e := os.ReadFile(st.File())
	must(t, e)
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	for {
		_, e := dec.Token()
		if e == io.EOF {
			break
		}
		must(t, e)
	}
	for _, bad := range []string{"fnm_multishells", "<string>node</string>", "<string>ui</string>", "<string>colima</string>"} {
		if strings.Contains(string(data), bad) {
			t.Fatal("startup polluted")
		}
	}
	_, e = st.Enable(context.Background())
	must(t, e)
	n := 0
	for _, c := range sf.Calls {
		if contains(c.Args, "bootstrap") {
			n++
		}
	}
	if n != 1 {
		t.Fatal("duplicate startup", n)
	}
	_, e = st.Disable(context.Background())
	must(t, e)
	for _, c := range sf.Calls {
		if contains(c.Args, "bootout") || contains(c.Args, "kill") {
			t.Fatal("disable killed current session")
		}
	}
	os.WriteFile(st.File(), []byte("<plist>foreign</plist>"), 0600)
	_, e = st.Enable(context.Background())
	expectCode(t, e, "STARTUP_FOREIGN")
	st.Platform = "linux"
	_, e = st.Enable(context.Background())
	expectCode(t, e, "STARTUP_UNSUPPORTED")
}
func TestStartupFailureRestoresPreviousPlist(t *testing.T) {
	f := newFixture(t, false)
	home := filepath.Join(f.Dir, "user")
	bin := filepath.Join(home, ".local", "bin", "nearprod")
	os.MkdirAll(filepath.Dir(bin), 0700)
	os.WriteFile(bin, []byte(BinaryMarker), 0755)
	sf := &systemFake{Base: f.F}
	st := &Startup{Home: home, CatalogHome: f.Home, Platform: "darwin", UID: 501, Runner: sf}
	_, e := st.Enable(context.Background())
	must(t, e)
	before, _ := os.ReadFile(st.File())
	sf.Fail = true
	_, e = st.Enable(context.Background())
	if e == nil {
		t.Fatal("failed launchctl reported success")
	}
	after, _ := os.ReadFile(st.File())
	if string(before) != string(after) {
		t.Fatal("previous config lost")
	}
}
