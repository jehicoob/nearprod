package nearprod

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func createDB(t *testing.T, f *fixture, r J, name string) J {
	t.Helper()
	v, e := f.S.Infra.CreateDatabase(context.Background(), J{"instance": r["id"], "name": name, "confirm": true}, nil)
	must(t, e)
	return obj(v["database"])
}
func TestSQLInstancesBindingAndSeparation(t *testing.T) {
	for _, engine := range []string{"postgres", "mysql"} {
		t.Run(engine, func(t *testing.T) {
			f := newFixture(t, false)
			r := f.createInstance(t, engine)
			a := createDB(t, f, r, "alpha_dev")
			b := createDB(t, f, r, "beta_dev")
			if a["username"] == b["username"] {
				t.Fatal("shared user")
			}
			ca, e := f.S.Infra.Connection(str(a["id"]), true)
			must(t, e)
			cb, e := f.S.Infra.Connection(str(b["id"]), true)
			must(t, e)
			if ca["password"] == cb["password"] {
				t.Fatal("shared password")
			}
			v, e := f.S.Infra.Probe(context.Background(), str(a["id"]))
			must(t, e)
			if !truth(v["readWrite"]) {
				t.Fatal("probe absent")
			}
			masked, e := f.S.Infra.Connection(str(a["id"]), false)
			must(t, e)
			if strings.Contains(string(mustJSON(masked)), str(ca["password"])) || strings.Contains(string(mustJSON(f.S.Catalog())), str(ca["password"])) {
				t.Fatal("secret in default output")
			}
			app := f.app(t, "app", "api", webModel())
			req := J{"target": app["id"], "database": a["id"], "services": A{"api"}, "mode": "dev"}
			p, e := f.S.Infra.BindingPreview(context.Background(), req)
			must(t, e)
			req["confirm"] = true
			req["fingerprint"] = p["fingerprint"]
			bind, e := f.S.Infra.Bind(context.Background(), req)
			must(t, e)
			_, e = f.S.Infra.BindingCheck(context.Background(), str(at(bind, "binding", "id")))
			if e == nil {
				t.Fatal("claimed binding applied before start")
			}
			f.trust(t, str(app["id"]), "dev")
			assertPass(t, f.act(t, str(app["id"]), "up", nil))
			_, e = f.S.Infra.BindingCheck(context.Background(), str(at(bind, "binding", "id")))
			must(t, e)
			app, _ = f.S.Store.Stack(str(app["id"]))
			cs, e := f.S.Docker.Owned(context.Background(), app)
			must(t, e)
			for _, x := range cs {
				c := obj(x)
				has := at(c, "networks", str(r["network"])) != nil
				if has != (str(c["service"]) == "api") {
					t.Fatalf("network expanded unexpectedly %s", c["service"])
				}
			}
			pre, e := f.S.Infra.StopPreview(context.Background(), str(r["id"]))
			must(t, e)
			_, e = f.S.Infra.Stop(context.Background(), J{"instance": r["id"], "confirm": true, "fingerprint": pre["fingerprint"]}, nil)
			expectCode(t, e, "INSTANCE_IN_USE")
			assertPass(t, f.act(t, str(app["id"]), "stop", nil))
			inst, e := f.S.Infra.Container(context.Background(), r)
			must(t, e)
			if !truth(inst["running"]) {
				t.Fatal("app stop stopped shared database")
			}
			_, e = f.S.Infra.Unbind(J{"binding": at(bind, "binding", "id"), "confirm": true})
			must(t, e)
			if len(arr(f.S.Infra.State()["databases"])) != 2 {
				t.Fatal("unbind erased database")
			}
			// Contract assurance: no administrative password may occur in the command arguments.
			vault, e := f.S.Infra.Secrets(r)
			must(t, e)
			for _, call := range f.F.History() {
				s := strings.Join(call.Args, " ")
				if strings.Contains(s, str(vault["admin"])) || strings.Contains(s, str(ca["password"])) {
					t.Fatal("secret passed in argv")
				}
			}
		})
	}
}
func TestRedisDedicatedAndConnection(t *testing.T) {
	f := newFixture(t, false)
	r := f.createInstance(t, "redis")
	a := createDB(t, f, r, "cache_app")
	_, e := f.S.Infra.CreateDatabase(context.Background(), J{"instance": r["id"], "name": "other", "confirm": true}, nil)
	expectCode(t, e, "REDIS_DEDICATED")
	_, e = f.S.Infra.Probe(context.Background(), str(a["id"]))
	must(t, e)
	app := f.app(t, "app", "worker", nil)
	p, e := f.S.Infra.BindingPreview(context.Background(), J{"target": app["id"], "database": a["id"], "services": A{"api"}})
	must(t, e)
	if str(at(p, "mapping", "url")) != "REDIS_URL" {
		t.Fatal("redis would replace sql url")
	}
	_, e = f.S.Infra.Backup(context.Background(), J{"database": a["id"], "confirm": true})
	expectCode(t, e, "BACKUP_ENGINE")
	foundHost := false
	for _, call := range f.F.History() {
		if contains(call.Args, "redis-cli") {
			if contains(call.Args, "--host") {
				t.Fatal("redis-cli long host option is not portable")
			}
			foundHost = foundHost || contains(call.Args, "-h")
		}
	}
	if !foundHost {
		t.Fatal("redis client host option absent")
	}
}
func TestPersistentVolumeMissingBlocksReinitialization(t *testing.T) {
	f := newFixture(t, false)
	r := f.createInstance(t, "postgres")
	if !truth(r["initialized"]) {
		t.Fatal("not initialized")
	}
	before := len(f.F.History())
	f.F.Set(func() { delete(f.F.Volumes, str(r["volume"])) })
	_, e := f.S.Infra.Start(context.Background(), str(r["id"]), nil)
	expectCode(t, e, "DATA_VOLUME_MISSING")
	for _, c := range f.F.History()[before:] {
		if contains(c.Args, "create") && contains(c.Args, str(r["volume"])) {
			t.Fatal("recreated empty volume")
		}
	}
}
func TestFolderPersistenceAndSecretPermissions(t *testing.T) {
	for _, image := range []string{"postgres:17", "postgres:18", "mysql:8.4"} {
		t.Run(image, func(t *testing.T) {
			f := newFixture(t, false)
			engine := strings.Split(image, ":")[0]
			if image == "postgres:18" {
				f.F.Endpoint = "unix://" + filepath.Join(userHome(), ".colima", "default", "docker.sock")
				must(t, f.S.Store.Update(func(v J) error {
					v["runtime"] = J{"kind": "colima", "context": "colima", "profile": "default"}
					return nil
				}))
			}
			req := J{"engine": engine, "id": "main", "image": image, "persistence": J{"kind": "folder"}}
			p, e := f.S.Infra.Preview(context.Background(), req)
			must(t, e)
			want := filepath.Join(f.Home, "databases", engine, "main")
			if str(at(p, "definition", "persistence", "path")) != want {
				t.Fatal("default path")
			}
			req["confirm"] = true
			req["fingerprint"] = p["fingerprint"]
			_, e = f.S.Infra.Create(context.Background(), req, nil)
			must(t, e)
			r, e := f.S.Infra.Instance("main")
			must(t, e)
			file, e := f.S.Infra.Render(r)
			must(t, e)
			model, e := readJSON(file, 2<<20)
			must(t, e)
			mounts := arr(at(model, "services", "database", "volumes"))
			expected := "/var/lib/mysql"
			if image == "postgres:17" {
				expected = "/var/lib/postgresql/data"
			}
			if image == "postgres:18" {
				expected = "/var/lib/postgresql"
			}
			found := false
			for _, v := range mounts {
				found = found || str(obj(v)["target"]) == expected && str(obj(v)["source"]) == filepath.Join(want, "data")
			}
			if !found {
				t.Fatalf("wrong data mount: %v", mounts)
			}
			secret := str(at(model, "secrets", "admin", "file"))
			st, e := os.Stat(secret)
			must(t, e)
			if st.Mode().Perm() != 0444 {
				t.Fatalf("entrypoint switched UID could not read secret: %o", st.Mode().Perm())
			}
			parent, e := os.Stat(filepath.Dir(secret))
			must(t, e)
			if parent.Mode().Perm() != 0700 {
				t.Fatal("secret parent not private")
			}
			st, e = os.Stat(f.S.Infra.SecretFile(r))
			must(t, e)
			if st.Mode().Perm() != 0600 {
				t.Fatal("vault not private")
			}
			os.RemoveAll(filepath.Join(want, "data"))
			_, e = f.S.Infra.Start(context.Background(), "main", nil)
			if e == nil {
				t.Fatal("missing folder reinitialized")
			}
		})
	}
}

func TestPostgres18FolderRejectedOnNativeLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("restriction applies to native Linux")
	}
	f := newFixture(t, false)
	req := J{"engine": "postgres", "id": "pg18-folder", "image": "postgres:18", "persistence": J{"kind": "folder"}}
	before := len(f.F.History())
	_, e := f.S.Infra.Preview(context.Background(), req)
	expectCode(t, e, "FOLDER_PLATFORM_UNSUPPORTED")
	if len(f.F.History()) != before {
		t.Fatal("unsupported folder preview called Docker")
	}
	_, e = f.S.Infra.Create(context.Background(), merge(req, J{"confirm": true, "fingerprint": "unused"}), nil)
	expectCode(t, e, "FOLDER_PLATFORM_UNSUPPORTED")
	if len(f.F.History()) != before || len(arr(at(f.S.Store.Get(), "infra", "instances"))) != 0 {
		t.Fatal("unsupported folder create changed Docker or store")
	}
	path := filepath.Join(f.Home, "databases", "postgres", "pg18-folder")
	if _, e = os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("unsupported folder preview or create changed filesystem")
	}
	unsupported := ss(f.S.Infra.List(J{})["folderUnsupportedImages"])
	if !contains(unsupported, "postgres:18") {
		t.Fatal("UI capability missing")
	}
	restrictions := arr(f.S.Infra.List(J{})["folderRestrictions"])
	if len(restrictions) != 1 || str(obj(restrictions[0])["code"]) != "FOLDER_PLATFORM_UNSUPPORTED" || str(obj(restrictions[0])["message"]) == "" {
		t.Fatal("UI restriction details missing")
	}

	must(t, f.S.Store.Update(func(v J) error {
		v["runtime"] = J{"kind": "colima", "context": "colima", "profile": "default"}
		return nil
	}))
	f.F.Endpoint = "unix://" + filepath.Join(userHome(), ".colima", "default", "docker.sock")
	p, e := f.S.Infra.Preview(context.Background(), req)
	must(t, e)
	req["confirm"] = true
	req["fingerprint"] = p["fingerprint"]
	_, e = f.S.Infra.Create(context.Background(), req, nil)
	must(t, e)
	must(t, f.S.Store.Update(func(v J) error {
		v["runtime"] = J{"kind": "native", "context": "default", "profile": "default"}
		return nil
	}))
	before = len(f.F.History())
	_, e = f.S.Infra.Start(context.Background(), "pg18-folder", nil)
	expectCode(t, e, "FOLDER_PLATFORM_UNSUPPORTED")
	if len(f.F.History()) != before {
		t.Fatal("blocked existing folder instance called Docker")
	}
}

func TestImageMajor(t *testing.T) {
	for image, want := range map[string]int{
		"postgres:18":          18,
		"postgres:18.1":        18,
		"postgres:18-alpine":   18,
		"postgres:18-bookworm": 18,
		"postgres:17":          17,
	} {
		if got := imageMajor(image); got != want {
			t.Errorf("imageMajor(%q) = %d, want %d", image, got, want)
		}
	}
}

func TestInfraPreviewValidations(t *testing.T) {
	f := newFixture(t, false)
	for _, req := range []J{
		{"engine": "traefik"}, {"engine": "postgres", "image": "postgres:latest"}, {"engine": "mysql", "image": "mysql:5.7"}, {"engine": "postgres", "hostPort": 80}, {"engine": "mysql", "memoryMiB": 128}, {"engine": "postgres", "persistence": J{"kind": "none"}}, {"engine": "postgres", "persistence": J{"kind": "folder", "path": f.Home}}, {"engine": "postgres", "id": "../x"},
	} {
		if _, e := f.S.Infra.Preview(context.Background(), req); e == nil {
			t.Fatalf("invalid request accepted: %v", req)
		}
	}
	foreign := filepath.Join(f.Dir, "old-data")
	os.Mkdir(foreign, 0700)
	os.WriteFile(filepath.Join(foreign, "data.txt"), []byte("keep"), 0600)
	_, e := f.S.Infra.Preview(context.Background(), J{"engine": "postgres", "persistence": J{"kind": "folder", "path": foreign}})
	if e == nil {
		t.Fatal("adopted foreign data")
	}
	_, e = f.S.Infra.Ports(context.Background(), J{"start": 16000, "end": 17000})
	if e == nil {
		t.Fatal("unbounded port scan")
	}
}
func TestCredentialLossAndRetrySafety(t *testing.T) {
	f := newFixture(t, false)
	r := f.createInstance(t, "postgres")
	a := createDB(t, f, r, "alpha")
	c, e := f.S.Infra.Connection(str(a["id"]), true)
	must(t, e)
	d := createDB(t, f, r, "alpha")
	if d["id"] != a["id"] {
		t.Fatal("retry new id")
	}
	c2, e := f.S.Infra.Connection(str(a["id"]), true)
	must(t, e)
	if c["password"] != c2["password"] {
		t.Fatal("retry rotated")
	}
	os.Remove(f.S.Infra.SecretFile(r))
	_, e = f.S.Infra.Start(context.Background(), str(r["id"]), nil)
	if e == nil {
		t.Fatal("lost secrets regenerated")
	}
	if _, e = os.Stat(f.S.Infra.SecretFile(r)); !os.IsNotExist(e) {
		t.Fatal("new secret written")
	}
}
func TestBackupRestoreAndGuards(t *testing.T) {
	for _, engine := range []string{"postgres", "mysql"} {
		t.Run(engine, func(t *testing.T) {
			f := newFixture(t, false)
			r := f.createInstance(t, engine)
			a := createDB(t, f, r, "source")
			b := createDB(t, f, r, "restored")
			backup, e := f.S.Infra.Backup(context.Background(), J{"database": a["id"], "confirm": true})
			must(t, e)
			file := str(backup["file"])
			st, e := os.Stat(file)
			must(t, e)
			if st.Mode().Perm() != 0600 || st.Size() == 0 {
				t.Fatal("bad dump")
			}
			req := J{"database": b["id"], "file": file, "confirm": true, "trustedBackup": true}
			_, e = f.S.Infra.Restore(context.Background(), req)
			must(t, e)
			os.WriteFile(file, []byte("changed"), 0600)
			_, e = f.S.Infra.Restore(context.Background(), req)
			expectCode(t, e, "BACKUP_CHECKSUM")
			_, e = f.S.Infra.Backup(context.Background(), J{"database": a["id"], "directory": filepath.Join(f.Home, "databases", "x"), "confirm": true})
			expectCode(t, e, "BACKUP_PATH")
		})
	}
}

func TestLegacyBindingIDRetainedOnEdit(t *testing.T) {
	f := newFixture(t, false)
	r := f.createInstance(t, "postgres")
	d := createDB(t, f, r, "legacy_dev")
	st := f.app(t, "legacy", "api", webModel())
	const oldID = "binding-0123456789abcdefabcd"
	must(t, f.S.Store.Update(func(v J) error {
		infra := obj(v["infra"])
		infra["bindings"] = A{J{"id": oldID, "databaseId": d["id"], "stackUid": st["uid"], "mode": "dev", "services": A{"api"}, "mapping": J{"url": "DATABASE_URL"}}}
		return nil
	}))
	req := J{"target": st["id"], "database": d["id"], "services": A{"api"}, "mode": "dev", "mapping": J{"url": "APPLICATION_DATABASE_URL"}}
	p, e := f.S.Infra.BindingPreview(context.Background(), req)
	must(t, e)
	req["confirm"] = true
	req["fingerprint"] = p["fingerprint"]
	result, e := f.S.Infra.Bind(context.Background(), req)
	must(t, e)
	if str(at(result, "binding", "id")) != oldID || len(arr(f.S.Infra.State()["bindings"])) != 1 {
		t.Fatal("legacy binding duplicated or renamed")
	}
}
