package nearprod

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func purgeFixture(t *testing.T) (*fixture, J) {
	t.Helper()
	f := newFixture(t, false)
	app := f.app(t, "purge", "app", nil)
	f.trust(t, str(app["id"]), "dev")
	assertPass(t, f.act(t, str(app["id"]), "up", J{"mode": "dev"}))
	app, e := f.S.Store.Stack(str(app["id"]))
	must(t, e)
	f.F.Set(func() {
		labels := J{LProject: app["projectName"], LOwner: f.S.Store.Get()["owner"], LStack: app["uid"]}
		f.F.Networks["purge-app_default"] = J{"Id": "purge-network-id", "Name": "purge-app_default", "Labels": labels, "Created": "2026-01-01"}
		f.F.Volumes["purge-app_data"] = J{"Name": "purge-app_data", "Labels": labels, "Driver": "local", "CreatedAt": "2026-01-01"}
		f.F.Images["sha256:"+strings.Repeat("c", 64)] = J{"Id": "sha256:" + strings.Repeat("c", 64), "Config": J{"Labels": labels}}
		for _, c := range f.F.Containers {
			if str(at(c, "Config", "Labels", LStack)) == str(app["uid"]) {
				c["Mounts"] = A{J{"Type": "volume", "Name": "purge-app_data", "Destination": "/data", "RW": true}, J{"Type": "bind", "Source": app["path"], "Destination": "/app", "RW": true}}
				obj(c["NetworkSettings"])["Networks"] = J{"purge-app_default": J{}}
				c["Image"] = "sha256:" + strings.Repeat("c", 64)
			}
		}
	})
	return f, app
}

func reviewedPurge(t *testing.T, f *fixture, app J) (J, J) {
	t.Helper()
	p, e := f.S.PurgeStackPreview(context.Background(), str(app["id"]))
	must(t, e)
	return p, J{"target": app["id"], "confirm": true, "acknowledgeDataLoss": true, "typedId": app["id"], "fingerprint": p["fingerprint"]}
}

func executePurge(t *testing.T, f *fixture, req J) J {
	t.Helper()
	op, e := f.S.PurgeStack(req)
	must(t, e)
	return waitOp(t, f.S, op)
}

func TestStackPurgeDeletesActiveAndArchivedResourcesPreservesCheckout(t *testing.T) {
	for _, archived := range []bool{false, true} {
		t.Run(map[bool]string{false: "active", true: "archived"}[archived], func(t *testing.T) {
			f, app := purgeFixture(t)
			if archived {
				assertPass(t, f.act(t, str(app["id"]), "stop", J{}))
				f.S.Refresh(context.Background())
				p, e := f.S.ArchiveStackPreview(str(app["id"]))
				must(t, e)
				_, e = f.S.ArchiveStack(context.Background(), J{"target": app["id"], "confirm": true, "fingerprint": p["fingerprint"]})
				must(t, e)
			}
			before := hash(f.S.Store.Get())
			p, req := reviewedPurge(t, f, app)
			if len(arr(p["blockers"])) != 0 || hash(f.S.Store.Get()) != before {
				t.Fatalf("preview mutated state or blocked: %v", p)
			}
			kinds := map[string]bool{}
			for _, r := range arr(p["resources"]) {
				kinds[str(at(r, "kind"))] = true
			}
			for _, kind := range []string{"container", "volume", "network", "image", "file"} {
				if !kinds[kind] {
					t.Fatalf("missing inventory: %s", kind)
				}
			}
			assertPass(t, executePurge(t, f, req))
			if len(allStacks(f.S.Store.Get())) != 0 || len(obj(f.S.Store.Get()["stackPurges"])) != 0 {
				t.Fatal("catalog or journal remains")
			}
			if len(f.F.Containers) != 0 || len(f.F.Networks) != 0 || len(f.F.Volumes) != 0 || len(f.F.Images) != 0 {
				t.Fatal("Docker resources remain")
			}
			_, e := os.Stat(filepath.Join(str(app["path"]), "compose.yaml"))
			must(t, e)
			if _, e = os.Stat(filepath.Join(f.S.Store.Home, "config", "generated", str(app["uid"])+".json")); !os.IsNotExist(e) {
				t.Fatal("generated overlay remains", e)
			}
		})
	}
}

func TestStackPurgeConfirmationAndChangedPreviewDoNotDelete(t *testing.T) {
	f, app := purgeFixture(t)
	_, req := reviewedPurge(t, f, app)
	wrong := copyJ(req)
	wrong["typedId"] = "wrong"
	_, e := f.S.PurgeStack(wrong)
	expectCode(t, e, "PURGE_CONFIRMATION")
	f.F.Set(func() { f.F.Volumes["purge-app_data"]["CreatedAt"] = "replaced" })
	op := executePurge(t, f, req)
	if str(at(op, "error", "code")) != "PREVIEW_CHANGED" || len(f.F.Containers) == 0 || len(obj(f.S.Store.Get()["stackPurges"])) != 0 {
		t.Fatal(op)
	}
}

func TestStackPurgeBlocksSharedAndForeignResources(t *testing.T) {
	for _, scenario := range []string{"shared-volume", "foreign-container", "external-driver", "unknown-bind", "wrong-engine"} {
		t.Run(scenario, func(t *testing.T) {
			f, app := purgeFixture(t)
			f.F.Set(func() {
				switch scenario {
				case "shared-volume":
					c := f.F.add("unrelated", "api", f.Root, J{}, false, "")
					c["Mounts"] = A{J{"Type": "volume", "Name": "purge-app_data"}}
				case "foreign-container":
					f.F.add(str(app["projectName"]), "foreign", f.Root, J{}, false, "")
				case "external-driver":
					f.F.Volumes["purge-app_data"]["Options"] = J{"device": "/host/data", "type": "none", "o": "bind"}
				case "unknown-bind":
					for _, c := range f.F.Containers {
						c["Mounts"] = A{J{"Type": "bind", "Source": "/unowned/data", "RW": true}}
					}
				case "wrong-engine":
					f.F.EngineID = "replacement-engine"
				}
			})
			p, e := f.S.PurgeStackPreview(context.Background(), str(app["id"]))
			if scenario == "wrong-engine" {
				expectCode(t, e, "ENGINE_IDENTITY")
				return
			}
			must(t, e)
			if len(arr(p["blockers"])) == 0 {
				t.Fatal("unsafe resource not blocked", p)
			}
			_, req := reviewedPurge(t, f, app)
			op := executePurge(t, f, req)
			if str(at(op, "error", "code")) != "PURGE_BLOCKED" || len(f.F.Volumes) == 0 {
				t.Fatal(op)
			}
		})
	}
}

func TestStackPurgeFailureRetainsPlanAndRetryRemovesOrphans(t *testing.T) {
	f, app := purgeFixture(t)
	// An anonymous volume must still be known after its container is removed.
	anonymous := strings.Repeat("d", 64)
	f.F.Set(func() {
		f.F.Volumes[anonymous] = J{"Name": anonymous, "Labels": J{}, "Driver": "local", "CreatedAt": "original"}
		for _, c := range f.F.Containers {
			c["Mounts"] = append(arr(c["Mounts"]), J{"Type": "volume", "Name": anonymous})
		}
		f.F.FailCmd = "volume rm"
	})
	_, req := reviewedPurge(t, f, app)
	op := executePurge(t, f, req)
	if str(at(op, "error", "code")) != "PURGE_PARTIAL" || len(f.F.Containers) != 0 {
		t.Fatal(op)
	}
	_, e := f.S.Store.Stack(str(app["id"]))
	expectCode(t, e, "STACK_PURGING")
	if !stackPurgePending(f.S.Store.Get(), str(app["uid"])) {
		t.Fatal("retry plan lost")
	}
	f.F.Set(func() { f.F.FailCmd = "" })
	p, req := reviewedPurge(t, f, app)
	if !truth(p["retry"]) {
		t.Fatal("retry not advertised")
	}
	assertPass(t, executePurge(t, f, req))
	if len(f.F.Volumes) != 0 {
		t.Fatal("orphan volume remains")
	}
}

func TestStackPurgeRetryRejectsReplacementVolume(t *testing.T) {
	f, app := purgeFixture(t)
	f.F.Set(func() { f.F.FailCmd = "volume rm" })
	_, req := reviewedPurge(t, f, app)
	executePurge(t, f, req)
	f.F.Set(func() { f.F.FailCmd = ""; f.F.Volumes["purge-app_data"]["CreatedAt"] = "replacement" })
	p, req := reviewedPurge(t, f, app)
	if len(arr(p["blockers"])) == 0 {
		t.Fatal("replacement not blocked")
	}
	op := executePurge(t, f, req)
	if str(op["state"]) != "failed" || f.F.Volumes["purge-app_data"] == nil {
		t.Fatal(op)
	}
}

func TestStackPurgeBoundDatabaseAndCredentialWithoutSiblingLoss(t *testing.T) {
	for _, engine := range []string{"postgres", "mysql"} {
		for _, archived := range []bool{false, true} {
			t.Run(engine+map[bool]string{false: "active", true: "archived"}[archived], func(t *testing.T) {
				f, app := purgeFixture(t)
				instance := f.createInstance(t, engine)
				db := createDB(t, f, instance, "purge_owned")
				sibling := createDB(t, f, instance, "keep_sibling")
				request := J{"target": app["id"], "database": db["id"], "services": A{"api"}, "mode": "dev"}
				p, e := f.S.Infra.BindingPreview(context.Background(), request)
				must(t, e)
				request["confirm"], request["fingerprint"] = true, p["fingerprint"]
				_, e = f.S.Infra.Bind(context.Background(), request)
				must(t, e)
				if archived {
					assertPass(t, f.act(t, str(app["id"]), "stop", J{}))
					f.S.Refresh(context.Background())
					p, e = f.S.ArchiveStackPreview(str(app["id"]))
					must(t, e)
					_, e = f.S.ArchiveStack(context.Background(), J{"target": app["id"], "confirm": true, "fingerprint": p["fingerprint"]})
					must(t, e)
				}
				_, req := reviewedPurge(t, f, app)
				assertPass(t, executePurge(t, f, req))
				_, e = f.S.Infra.Database(str(sibling["id"]))
				must(t, e)
				if len(databaseLifecycleBindings(f.S.Store.Get(), str(db["id"]))) != 0 {
					t.Fatal("binding remains")
				}
				vault, e := f.S.Infra.Secrets(instance)
				must(t, e)
				if at(vault, "databases", str(db["id"])) != nil || at(vault, "databases", str(sibling["id"])) == nil {
					t.Fatal("wrong credential cleanup")
				}
				_, e = f.S.Infra.Instance(str(instance["id"]))
				must(t, e)
			})
		}
	}

}

func bindPurgeDatabase(t *testing.T, f *fixture, app, db J) {
	t.Helper()
	req := J{"target": app["id"], "database": db["id"], "services": A{"api"}, "mode": "dev"}
	p, e := f.S.Infra.BindingPreview(context.Background(), req)
	must(t, e)
	req["confirm"], req["fingerprint"] = true, p["fingerprint"]
	_, e = f.S.Infra.Bind(context.Background(), req)
	must(t, e)
}

func TestStackPurgeSharedDatabaseBlocksBeforeAnyMutation(t *testing.T) {
	f, app := purgeFixture(t)
	instance := f.createInstance(t, "postgres")
	db := createDB(t, f, instance, "shared_data")
	other := f.app(t, "other", "app", nil)
	bindPurgeDatabase(t, f, app, db)
	bindPurgeDatabase(t, f, other, db)
	p, req := reviewedPurge(t, f, app)
	if len(arr(p["blockers"])) == 0 {
		t.Fatal("shared DB not blocked")
	}
	before := hash(f.F.Containers)
	op := executePurge(t, f, req)
	if str(at(op, "error", "code")) != "PURGE_BLOCKED" || before != hash(f.F.Containers) || len(databaseLifecycleBindings(f.S.Store.Get(), str(db["id"]))) != 2 {
		t.Fatal(op)
	}
}

func TestStackPurgeDatabaseFailureResumesAccountAndMetadata(t *testing.T) {
	f, app := purgeFixture(t)
	instance := f.createInstance(t, "postgres")
	db := createDB(t, f, instance, "resume_data")
	bindPurgeDatabase(t, f, app, db)
	_, req := reviewedPurge(t, f, app)
	f.F.Set(func() { f.F.SQLFailContains = "DROP ROLE" })
	op := executePurge(t, f, req)
	if str(at(op, "error", "code")) != "PURGE_PARTIAL" {
		t.Fatal(op)
	}
	current, e := f.S.Infra.Database(str(db["id"]))
	must(t, e)
	if str(at(current, "purge", "phase")) != "account" {
		t.Fatal(current)
	}
	f.F.Set(func() { f.F.SQLFailContains = ""; f.F.SQLFailure = "" })
	_, req = reviewedPurge(t, f, app)
	assertPass(t, executePurge(t, f, req))
	if len(allStacks(f.S.Store.Get())) != 0 {
		t.Fatal("catalog remains")
	}
}

func TestStackPurgeClearsAppliedRoutesAndPreservesSibling(t *testing.T) {
	f, app := purgeFixture(t)
	other := f.app(t, "other", "app", nil)
	must(t, f.S.Store.Update(func(state J) error {
		for _, raw := range arr(state["stacks"]) {
			stack := obj(raw)
			stack["proxyApplied"] = J{"mode": "dev", "routes": A{J{"host": str(stack["slug"]) + "-" + str(stack["product"]) + ".localhost", "service": "api", "port": 8080}}}
		}
		return nil
	}))
	must(t, f.S.Proxy.SyncRoutes())
	_, req := reviewedPurge(t, f, app)
	assertPass(t, executePurge(t, f, req))
	_, e := f.S.Store.Stack(str(other["id"]))
	must(t, e)
	routes, e := os.ReadFile(str(f.S.Proxy.Paths()["dynamic"]))
	must(t, e)
	if strings.Contains(string(routes), "app-purge.localhost") || !strings.Contains(string(routes), "app-other.localhost") {
		t.Fatal("wrong route cleanup")
	}
}

func TestStackPurgeRejectsChangedGeneratedFileBeforeRemovingIt(t *testing.T) {
	f, app := purgeFixture(t)
	file := filepath.Join(f.S.Store.Home, "config", "generated", str(app["uid"])+".json")
	f.F.Set(func() { f.F.FailCmd = "volume rm" })
	_, req := reviewedPurge(t, f, app)
	executePurge(t, f, req)
	must(t, os.WriteFile(file, []byte("changed"), 0600))
	f.F.Set(func() { f.F.FailCmd = "" })
	p, req := reviewedPurge(t, f, app)
	if len(arr(p["blockers"])) == 0 {
		t.Fatal("changed overlay not blocked")
	}
	op := executePurge(t, f, req)
	if str(op["state"]) != "failed" {
		t.Fatal(op)
	}
	data, e := os.ReadFile(file)
	must(t, e)
	if string(data) != "changed" {
		t.Fatal("changed overlay deleted")
	}
}

func TestStackPurgeAcceptsMountOrderingButRejectsChangedMounts(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "order-only", true: "changed-content"}[changed], func(t *testing.T) {
			f, app := purgeFixture(t)
			f.F.Set(func() {
				for _, container := range f.F.Containers {
					container["Mounts"] = append(arr(container["Mounts"]), J{"Type": "bind", "Source": filepath.Join(str(app["path"]), "settings"), "Destination": "/settings", "RW": false})
				}
			})
			_, req := reviewedPurge(t, f, app)
			f.F.Set(func() {
				for _, container := range f.F.Containers {
					mounts := arr(container["Mounts"])
					mounts[0], mounts[2] = mounts[2], mounts[0]
					if changed {
						obj(mounts[1])["Destination"] = "/changed-data"
					}
				}
			})
			op := executePurge(t, f, req)
			if changed {
				if str(at(op, "error", "code")) != "PREVIEW_CHANGED" || len(f.F.Containers) == 0 {
					t.Fatal(op)
				}
			} else {
				assertPass(t, op)
			}
		})
	}
}

func TestStackPurgeImageTagOrderDoesNotChangeIdentity(t *testing.T) {
	original := J{"Id": "sha256:fixture", "RepoTags": A{"fixture:dev", "fixture:verify"}}
	reordered := J{"Id": "sha256:fixture", "RepoTags": A{"fixture:verify", "fixture:dev"}}
	if hash(purgeDockerIdentity("image", original)) != hash(purgeDockerIdentity("image", reordered)) {
		t.Fatal("tag ordering changed identity")
	}
	reordered["RepoTags"] = A{"fixture:dev", "other:latest"}
	if hash(purgeDockerIdentity("image", original)) == hash(purgeDockerIdentity("image", reordered)) {
		t.Fatal("different tags did not change identity")
	}
}
