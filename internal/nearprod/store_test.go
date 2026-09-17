package nearprod

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func tempHome(t *testing.T) string {
	t.Helper()
	p, e := os.MkdirTemp("", "np-home-")
	must(t, e)
	p, _ = filepath.EvalSymlinks(p)
	t.Cleanup(func() { os.RemoveAll(p) })
	return p
}
func legacyState(t *testing.T, home string) J {
	t.Helper()
	v := initialState()
	v["version"] = 3
	v["owner"] = "legacy-owner-keep-exactly"
	v["runtime"] = J{"kind": "native", "context": "default", "profile": "default"}
	path := filepath.Join(home, "project-outside-release")
	must(t, os.MkdirAll(path, 0700))
	must(t, os.WriteFile(filepath.Join(path, "compose.yaml"), []byte("services: {}\n"), 0600))
	v["roots"] = A{path}
	v["groups"] = A{J{"id": "maximo-puntaje", "name": "Máximo Puntaje"}}
	v["stacks"] = A{J{"id": "maximo-puntaje/api", "product": "maximo-puntaje", "slug": "api", "name": "Laravel API", "uid": "stack-UID_keep123", "projectName": "maximo-puntaje-api", "path": path, "modes": J{"dev": J{"files": A{filepath.Join(path, "compose.yaml")}, "envFiles": A{}, "profiles": A{}}}, "routes": A{J{"host": "api-maximo-puntaje.localhost", "service": "caddy", "port": 80}}, "trust": J{"dev": J{"fingerprint": "legacy-fingerprint", "allowUnsafe": true}}, "binding": J{"endpoint": "unix:///var/run/docker.sock", "engineId": "fixture-engine", "adoptedIds": A{strings.Repeat("a", 64)}}, "activeMode": "dev", "expectedServices": A{"caddy"}, "unknownExtension": J{"doNotDrop": "value"}}}
	uid := "0123456789abcdef"
	r := J{"id": "mysql-main", "uid": uid, "name": "MySQL", "engine": "mysql", "requestedImage": "mysql:8.4", "resolvedImage": "mysql@sha256:" + strings.Repeat("a", 64), "imageId": "sha256:" + strings.Repeat("b", 64), "projectName": "np-infra-legacy-" + uid, "network": "np-data-legacy-" + uid, "volume": "np-data-legacy-" + uid, "hostname": "np-mysql-" + uid, "initialized": true, "persistence": J{"kind": "volume"}, "memoryMiB": 768, "maxConnections": 32, "hostPort": 13306, "binding": J{"endpoint": "unix:///var/run/docker.sock", "engineId": "fixture-engine"}}
	v["infra"] = J{"instances": A{r}, "databases": A{J{"id": "mysql-db-a", "instanceUid": uid, "name": "maximo_dev", "username": "maximo_app", "state": "ready"}}, "bindings": A{J{"id": "bind-existing", "databaseId": "mysql-db-a", "stackUid": "stack-UID_keep123", "mode": "dev", "services": A{"php"}, "mapping": J{"url": "DATABASE_URL"}}}}
	vault := J{"owner": v["owner"], "uid": uid, "admin": strings.Repeat("1", 48), "databases": J{"mysql-db-a": J{"password": strings.Repeat("2", 48)}}}
	must(t, writeJSON(filepath.Join(home, "infra", uid, "vault.json"), vault))
	return v
}
func TestMigrationPreservesProjectAndDataIdentities(t *testing.T) {
	h := tempHome(t)
	v := legacyState(t, h)
	raw, _ := json.MarshalIndent(v, "", "  ")
	old := filepath.Join(h, "catalog.json")
	must(t, os.WriteFile(old, raw, 0600))
	beforeVault, _ := os.ReadFile(filepath.Join(h, "infra", "0123456789abcdef", "vault.json"))
	p, e := MigrationPreview(h)
	must(t, e)
	if !truth(p["requiresMigration"]) {
		t.Fatal(p)
	}
	if _, e := os.Stat(filepath.Join(h, "config")); !os.IsNotExist(e) {
		t.Fatal("preview wrote config")
	}
	s, e := OpenStore(h)
	must(t, e)
	after := s.Get()
	for _, k := range []string{"owner", "roots", "groups", "stacks", "runtime", "infra", "toolPaths"} {
		if hash(v[k]) != hash(after[k]) {
			t.Errorf("modified legacy %s", k)
		}
	}
	marker, e := readJSON(old, 1<<20)
	must(t, e)
	if integer(marker["version"]) != -1 {
		t.Fatal("legacy writer not blocked")
	}
	j, e := readJSON(filepath.Join(h, "config", "migration.json"), 1<<20)
	must(t, e)
	b, e := os.ReadFile(str(j["backup"]))
	must(t, e)
	if string(b) != string(raw) {
		t.Fatal("backup not exact")
	}
	vault, _ := os.ReadFile(filepath.Join(h, "infra", "0123456789abcdef", "vault.json"))
	if string(vault) != string(beforeVault) {
		t.Fatal("vault changed")
	}
	s2, e := OpenStore(h)
	must(t, e)
	if hash(s.Get()) != hash(s2.Get()) {
		t.Fatal("second opening mutated catalog")
	}
	for _, p := range []string{s.File, str(j["backup"])} {
		st, e := os.Stat(p)
		must(t, e)
		if st.Mode().Perm() != 0600 {
			t.Errorf("permissions %s=%o", p, st.Mode().Perm())
		}
	}
}
func TestMigrationFailureCases(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(string)
		code    string
	}{
		{"corrupt", func(h string) { os.WriteFile(filepath.Join(h, "catalog.json"), []byte("{"), 0600) }, "INVALID_JSON"},
		{"future-version", func(h string) { v := initialState(); v["version"] = 99; writeJSON(filepath.Join(h, "catalog.json"), v) }, "SCHEMA_UNSUPPORTED"},
		{"fractional-version", func(h string) {
			v := initialState()
			v["version"] = 3.5
			writeJSON(filepath.Join(h, "catalog.json"), v)
		}, "SCHEMA_UNSUPPORTED"},
		{"missing-with-databases", func(h string) { os.MkdirAll(filepath.Join(h, "databases", "mysql"), 0700) }, "CATALOG_MISSING"},
		{"missing-with-resource-vault", func(h string) { writeJSON(filepath.Join(h, "config", "resources", "fake", "vault.json"), J{}) }, "CATALOG_MISSING"},
		{"legacy-marker-without-canonical", func(h string) { writeJSON(filepath.Join(h, "catalog.json"), J{"version": -1}) }, "SCHEMA_UNSUPPORTED"},
		{"legacy-symlink", func(h string) {
			os.WriteFile(filepath.Join(h, "other"), []byte("{}"), 0600)
			os.Symlink(filepath.Join(h, "other"), filepath.Join(h, "catalog.json"))
		}, "UNSAFE_FILE"},
		{"config-symlink", func(h string) {
			os.MkdirAll(filepath.Join(h, "elsewhere"), 0700)
			os.Symlink(filepath.Join(h, "elsewhere"), filepath.Join(h, "config"))
		}, "SYMLINK_PATH"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := tempHome(t)
			tc.prepare(h)
			_, e := OpenStore(h)
			expectCode(t, e, tc.code)
			if _, e := os.Stat(filepath.Join(h, "config", "catalog.json")); !os.IsNotExist(e) {
				t.Fatal("unexpected canonical created")
			}
		})
	}
}
func TestDualCatalogRefused(t *testing.T) {
	h := tempHome(t)
	v := legacyState(t, h)
	must(t, writeJSON(filepath.Join(h, "catalog.json"), v))
	_, e := OpenStore(h)
	must(t, e)
	must(t, writeJSON(filepath.Join(h, "catalog.json"), v))
	_, e = OpenStore(h)
	expectCode(t, e, "DUAL_CATALOG")
}
func TestMigrationInterruptedJournalResumes(t *testing.T) {
	for _, modified := range []bool{false, true} {
		t.Run(strBool(modified), func(t *testing.T) {
			h := tempHome(t)
			v := legacyState(t, h)
			must(t, writeJSON(filepath.Join(h, "catalog.json"), v))
			_, e := OpenStore(h)
			must(t, e)
			j, e := readJSON(filepath.Join(h, "config", "migration.json"), 1<<20)
			must(t, e)
			b, e := os.ReadFile(str(j["backup"]))
			must(t, e)
			must(t, os.WriteFile(filepath.Join(h, "catalog.json"), b, 0600))
			j["phase"] = "prepared"
			must(t, writeJSON(filepath.Join(h, "config", "migration.json"), j))
			if modified {
				must(t, os.WriteFile(filepath.Join(h, "catalog.json"), append(b, ' '), 0600))
			}
			_, e = OpenStore(h)
			if modified {
				expectCode(t, e, "MIGRATION_CONFLICT")
			} else {
				must(t, e)
				j, _ = readJSON(filepath.Join(h, "config", "migration.json"), 1<<20)
				if str(j["phase"]) != "committed" {
					t.Fatal(j)
				}
			}
		})
	}
}
func strBool(b bool) string {
	if b {
		return "changed"
	}
	return "unchanged"
}
func TestStoreRecoveryAndAtomicWrites(t *testing.T) {
	h := tempHome(t)
	s, e := OpenStore(h)
	must(t, e)
	must(t, s.Update(func(v J) error {
		v["operations"] = A{J{"id": "active", "state": "running"}, J{"id": "old", "state": "succeeded"}}
		return nil
	}))
	s, e = OpenStore(h)
	must(t, e)
	if str(at(arr(s.Get()["operations"])[0], "state")) != "interrupted" {
		t.Fatal(s.Get())
	}
	before, _ := os.ReadFile(s.File)
	e = s.Update(func(v J) error { v["version"] = 100; return nil })
	if e == nil {
		t.Fatal("invalid update committed")
	}
	after, _ := os.ReadFile(s.File)
	if string(before) != string(after) {
		t.Fatal("failed update modified file")
	}
	var wg sync.WaitGroup
	for n := 0; n < 30; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			e := s.Update(func(v J) error {
				v["groups"] = append(arr(v["groups"]), J{"id": "group-" + str(n), "name": "Grupo"})
				return nil
			})
			if e != nil {
				t.Error(e)
			}
		}(n)
	}
	wg.Wait()
	if len(arr(s.Get()["groups"])) != 30 {
		t.Fatal("lost updates")
	}
}
func TestLockHonorsOldAgentAndConcurrentGoStarts(t *testing.T) {
	h := tempHome(t)
	sock := filepath.Join(h, "agent.sock")
	ln, e := net.Listen("unix", sock)
	must(t, e)
	_, e = AcquireAgentLock(h)
	expectCode(t, e, "AGENT_RUNNING")
	ln.Close()
	const count = 12
	var wg sync.WaitGroup
	ch := make(chan func(), count)
	for n := 0; n < count; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, e := AcquireAgentLock(h)
			if e == nil {
				ch <- release
			}
		}()
	}
	wg.Wait()
	close(ch)
	if len(ch) != 1 {
		t.Errorf("writers=%d", len(ch))
	}
	for release := range ch {
		release()
	}
	r, e := AcquireAgentLock(h)
	must(t, e)
	r()
}
func TestConfigBackupIncludesSecretsNotPhysicalData(t *testing.T) {
	h := tempHome(t)
	v := legacyState(t, h)
	must(t, writeJSON(filepath.Join(h, "catalog.json"), v))
	s, e := OpenStore(h)
	must(t, e)
	svc := NewService(context.Background(), s, newFake())
	defer svc.Close()
	must(t, os.MkdirAll(filepath.Join(h, "databases", "ignored"), 0700))
	must(t, os.WriteFile(filepath.Join(h, "databases", "ignored", "data"), []byte("PHYSICAL"), 0600))
	r, e := svc.ConfigBackup(context.Background())
	must(t, e)
	f, e := os.Open(str(r["file"]))
	must(t, e)
	defer f.Close()
	gz, e := gzip.NewReader(f)
	must(t, e)
	tr := tar.NewReader(gz)
	names := []string{}
	var secret bool
	for {
		hdr, e := tr.Next()
		if e == io.EOF {
			break
		}
		must(t, e)
		names = append(names, hdr.Name)
		b, _ := io.ReadAll(tr)
		if strings.Contains(string(b), "PHYSICAL") {
			t.Fatal("copied data")
		}
		if strings.HasSuffix(hdr.Name, "vault.json") {
			secret = true
		}
		if hdr.Mode != 0600 {
			t.Fatal("permissions")
		}
	}
	if !secret || !contains(names, "config/catalog.json") || !contains(names, "MANIFEST.json") {
		t.Fatal(names)
	}
}
func TestCatalogRejectsPathTraversalAndBrokenReferences(t *testing.T) {
	for _, which := range []string{"stack-uid", "managed-dir", "runtime", "root", "duplicate-group", "missing-infra-list"} {
		t.Run(which, func(t *testing.T) {
			h := tempHome(t)
			v := legacyState(t, h)
			v["version"] = SchemaVersion
			switch which {
			case "stack-uid":
				obj(arr(v["stacks"])[0])["uid"] = "../../foreign"
			case "managed-dir":
				obj(arr(at(v, "infra", "instances"))[0])["managedDir"] = "../../outside"
			case "runtime":
				obj(v["runtime"])["kind"] = "remote"
			case "root":
				v["roots"] = A{"relative"}
			case "duplicate-group":
				v["groups"] = append(arr(v["groups"]), arr(v["groups"])[0])
			case "missing-infra-list":
				obj(v["infra"])["databases"] = "invalid"
			}
			if validateState(v) == nil {
				t.Fatal("accepted invalid catalog")
			}
		})
	}
}

func TestActual061GoldenMigrationAndRouting(t *testing.T) {
	raw, e := os.ReadFile("../../tests/fixtures/catalog-0.6.1.json")
	must(t, e)
	var before J
	must(t, json.Unmarshal(raw, &before))
	h := tempHome(t)
	must(t, os.WriteFile(filepath.Join(h, "catalog.json"), raw, 0600))
	s, e := OpenStore(h)
	must(t, e)
	after := s.Get()
	for _, k := range []string{"owner", "runtime", "roots", "groups", "stacks", "infra", "proxy"} {
		if hash(before[k]) != hash(after[k]) {
			t.Fatalf("actual JS-generated field %s changed", k)
		}
	}
	expected, e := readJSON("../../tests/fixtures/identities-0.6.1.json", 100000)
	must(t, e)
	st := obj(arr(after["stacks"])[0])
	if hash(proxyNames(str(after["owner"]))) != hash(expected["proxyNames"]) || routeKey(st, str(expected["host"])) != str(expected["routeKey"]) || serviceAlias(st, str(expected["service"])) != str(expected["serviceAlias"]) {
		t.Fatal("JS and Go routing identities differ")
	}
}

func TestMigrationDoesNotRoundUnknownIntegerFields(t *testing.T) {
	h := tempHome(t)
	v := initialState()
	v["version"] = 3
	v["extension"] = J{"large": json.Number("9007199254740993123")}
	raw, e := json.Marshal(v)
	must(t, e)
	must(t, os.WriteFile(filepath.Join(h, "catalog.json"), raw, 0600))
	s, e := OpenStore(h)
	must(t, e)
	if str(at(s.Get(), "extension", "large")) != "9007199254740993123" {
		t.Fatal("migration rounded an unknown JSON integer")
	}
}
