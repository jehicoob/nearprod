package nearprod

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func webModel() J {
	return J{"services": J{"web": J{"image": "fixture:test", "healthcheck": J{"test": A{"CMD", "true"}}}, "api": J{"image": "fixture:test"}, "db": J{"image": "postgres:17"}}}
}
func addRoute(t *testing.T, f *fixture, st J, service, host string, port int) J {
	t.Helper()
	in := copyJ(st)
	in["routes"] = A{J{"host": host, "service": service, "port": port}}
	s, e := f.S.Edit(str(st["id"]), in)
	must(t, e)
	return s
}
func enableProxy(t *testing.T, f *fixture, port int) {
	t.Helper()
	req := J{"port": port}
	pre, e := f.S.Proxy.Preview(context.Background(), req)
	must(t, e)
	req["confirm"] = true
	req["fingerprint"] = pre["fingerprint"]
	_, e = f.S.Proxy.Start(context.Background(), req, nil)
	must(t, e)
}

func TestOfflineRegistrationDiscoveryAndGroups(t *testing.T) {
	f := newFixture(t, false)
	f.F.Set(func() { f.F.Connected = false })
	a := f.app(t, "first", "api", nil)
	b := f.app(t, "second", "api", nil)
	if a["uid"] == b["uid"] || a["projectName"] == b["projectName"] {
		t.Fatal("identity collision")
	}
	if len(f.F.History()) != 0 {
		t.Fatal("registration invoked engine")
	}
	path := str(a["path"])
	os.WriteFile(filepath.Join(path, ".env"), []byte("TOP_SECRET=hide\n"), 0600)
	os.WriteFile(filepath.Join(path, "compose.dev.yaml"), []byte("services:\n  worker:\n    profiles: [jobs]\n"), 0600)
	must(t, os.MkdirAll(filepath.Join(path, "node_modules", "ignored"), 0700))
	os.WriteFile(filepath.Join(path, "node_modules", "ignored", "compose.yaml"), []byte("{}"), 0600)
	opts, e := f.S.ProjectOptions(J{"path": path})
	must(t, e)
	if strings.Contains(string(mustJSON(opts)), "hide") {
		t.Fatal("env secret exposed")
	}
	v, e := f.S.Discover(context.Background(), J{"root": f.Root})
	must(t, e)
	if strings.Contains(string(mustJSON(v)), "node_modules") {
		t.Fatal("dependencies scanned")
	}
	_, e = f.S.Group(J{"id": "first", "name": "Primero"}, true)
	must(t, e)
	input := copyJ(a)
	input["product"] = "moved"
	input["name"] = "API"
	moved, e := f.S.Edit(str(a["id"]), input)
	must(t, e)
	if moved["uid"] != a["uid"] || moved["projectName"] != a["projectName"] {
		t.Fatal("moving group renamed Docker identity")
	}
	_, e = f.S.Remove(context.Background(), str(moved["id"]), false)
	expectCode(t, e, "CONFIRM_REQUIRED")
	beforeRemove := len(f.F.History())
	_, e = f.S.Remove(context.Background(), str(moved["id"]), true)
	expectCode(t, e, "DOCKER_UNAVAILABLE")
	if len(f.F.History()) <= beforeRemove {
		t.Fatal("remove did not require a fresh Docker observation")
	}
	_, e = f.S.Store.Stack(str(moved["id"]))
	must(t, e)
}
func TestBatchRegistrationAtomicAndDuplicateRoutes(t *testing.T) {
	f := newFixture(t, false)
	a := f.app(t, "first", "api", nil)
	in := copyJ(a)
	in["product"] = "batch"
	in["slug"] = "new"
	in["projectName"] = "new"
	_, e := f.S.Register(J{"definitions": A{in, in}}, true)
	expectCode(t, e, "STACK_DUPLICATE")
	if len(arr(f.S.Store.Get()["stacks"])) != 1 {
		t.Fatal("partial batch")
	}
	addRoute(t, f, a, "api", "one.localhost", 8000)
	b := f.app(t, "other", "api", nil)
	in = copyJ(b)
	in["routes"] = A{J{"host": "one.localhost", "service": "api", "port": 8000}}
	_, e = f.S.Edit(str(b["id"]), in)
	if e == nil {
		t.Fatal("duplicate domain accepted")
	}
	if len(arr(at(f.S.Store.Get(), "stacks"))) != 2 {
		t.Fatal("registry changed")
	}
}
func TestLifecycleBuildAndRecoveryWithoutYAML(t *testing.T) {
	f := newFixture(t, false)
	a := f.app(t, "app", "api", nil)
	id := str(a["id"])
	f.trust(t, id, "dev")
	assertPass(t, f.act(t, id, "up", J{"wait": true}))
	for _, c := range f.F.History() {
		if contains(c.Args, "up") && contains(c.Args, "--build") {
			t.Fatal("start forced build")
		}
	}
	os.Remove(filepath.Join(str(a["path"]), "compose.yaml"))
	assertPass(t, f.act(t, id, "stop", nil))
	assertPass(t, f.act(t, id, "restart", nil))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got := make(chan J, 2)
	done := make(chan error, 1)
	go func() { done <- f.S.Logs(ctx, id, J{"follow": true}, func(v J) { got <- v; cancel() }) }()
	select {
	case v := <-got:
		if strings.Contains(str(v["text"]), "private123") {
			t.Fatal("log unredacted")
		}
	case <-time.After(time.Second):
		t.Fatal("logs missing")
	}
	<-done
	for _, c := range f.F.History() {
		if contains(c.Args, "rm") || contains(c.Args, "down") || contains(c.Args, "prune") {
			t.Fatal("destructive lifecycle")
		}
	}
}
func TestExplicitRebuildAndChangedApproval(t *testing.T) {
	f := newFixture(t, false)
	a := f.app(t, "app", "api", nil)
	id := str(a["id"])
	op := f.act(t, id, "up", nil)
	if str(op["state"]) != "failed" {
		t.Fatal("untrusted start")
	}
	f.trust(t, id, "dev")
	assertPass(t, f.act(t, id, "rebuild", nil))
	hasBuild := false
	for _, c := range f.F.History() {
		hasBuild = hasBuild || contains(c.Args, "build") || contains(c.Args, "--build")
	}
	if !hasBuild {
		t.Fatal("build missing")
	}
	writeJSON(filepath.Join(str(a["path"]), "compose.yaml"), J{"services": J{"api": J{"image": "changed:test"}}})
	op = f.act(t, id, "up", nil)
	if str(op["state"]) != "failed" {
		t.Fatal("changed source trusted")
	}
}
func TestGroupPartialFailureAndInvalidSibling(t *testing.T) {
	f := newFixture(t, false)
	a := f.app(t, "app", "a", nil)
	b := f.app(t, "app", "b", nil)
	f.trust(t, str(a["id"]), "dev")
	f.trust(t, str(b["id"]), "dev")
	f.F.Set(func() { f.F.FailProject = str(b["projectName"]) })
	op := f.act(t, "app", "up", nil)
	if str(op["state"]) != "failed" {
		t.Fatal("partial presented success")
	}
	cs, e := f.S.Docker.Containers(context.Background(), false)
	must(t, e)
	if len(cs) != 1 || !truth(obj(cs[0])["running"]) {
		t.Fatal("success rolled back")
	}
	os.Remove(filepath.Join(str(b["path"]), "compose.yaml"))
	assertPass(t, f.act(t, str(a["id"]), "up", nil))
}
func TestUnsafePreviewAndVerifyMode(t *testing.T) {
	f := newFixture(t, false)
	a := f.app(t, "unsafe", "api", J{"services": J{"api": J{"image": "x", "privileged": true, "volumes": A{J{"type": "bind", "source": f.Root, "target": "/src"}}, "command": A{"mix", "phx.server"}}}})
	p, e := f.S.Preview(context.Background(), str(a["id"]), "dev")
	must(t, e)
	if len(arr(p["risks"])) == 0 {
		t.Fatal("no risks")
	}
	_, e = f.S.Trust(context.Background(), str(a["id"]), J{"mode": "dev", "fingerprint": p["fingerprint"]})
	expectCode(t, e, "RISK_APPROVAL_REQUIRED")
	p, e = f.S.Preview(context.Background(), str(a["id"]), "verify")
	must(t, e)
	if len(arr(p["blockers"])) == 0 {
		t.Fatal("verify allowed bind/dev")
	}
	_, e = f.S.Trust(context.Background(), str(a["id"]), J{"mode": "verify", "fingerprint": p["fingerprint"], "allowUnsafe": true})
	expectCode(t, e, "VERIFY_BLOCKED")
}
func TestAdoptRequiresPathAndEngineMatches(t *testing.T) {
	f := newFixture(t, false)
	a := f.app(t, "owned", "api", nil)
	id := str(a["id"])
	c := f.F.Add(str(a["projectName"]), "api", str(a["path"]), J{}, true, "")
	f.trust(t, id, "dev")
	op := f.act(t, id, "stop", nil)
	if str(op["state"]) != "failed" {
		t.Fatal("external stopped")
	}
	p, e := f.S.Adoption(context.Background(), id)
	must(t, e)
	_, e = f.S.Adopt(context.Background(), id, J{"confirm": true, "fingerprint": p["fingerprint"]})
	must(t, e)
	assertPass(t, f.act(t, id, "stop", nil))
	f.F.Set(func() { f.F.EngineID = "changed" })
	op = f.act(t, id, "restart", nil)
	if str(op["state"]) != "failed" {
		t.Fatal("engine identity not checked")
	}
	f.F.Set(func() {
		f.F.EngineID = "fixture-engine"
		obj(f.F.Containers[str(c["Id"])]["Config"])["Labels"] = J{LProject: a["projectName"], LWorking: "/elsewhere", LService: "api"}
	})
	p, e = f.S.Adoption(context.Background(), id)
	must(t, e)
	if truth(p["allowed"]) {
		t.Fatal("adopt foreign path")
	}
}
func TestProxyOverlayRoutingAndScope(t *testing.T) {
	f := newFixture(t, false)
	a := f.app(t, "shop", "web", webModel())
	a = addRoute(t, f, a, "web", "shop.localhost", 5173)
	enableProxy(t, f, 8080)
	f.trust(t, str(a["id"]), "dev")
	assertPass(t, f.act(t, str(a["id"]), "up", nil))
	current, _ := f.S.Store.Stack(str(a["id"]))
	if len(arr(at(current, "proxyApplied", "routes"))) != 1 {
		t.Fatal("route not applied")
	}
	cs, e := f.S.Docker.Owned(context.Background(), current)
	must(t, e)
	netName := str(proxyNames(str(f.S.Store.Get()["owner"]))["network"])
	for _, v := range cs {
		c := obj(v)
		has := at(c, "networks", netName) != nil
		if has != (str(c["service"]) == "web") {
			t.Fatalf("wrong network consumer: %v", c)
		}
	}
	if !strings.Contains(string(mustJSON(f.S.Status())), ":8080") {
		t.Fatal("nonstandard proxy port missing")
	}
	for _, c := range f.F.History() {
		if contains(c.Args, "up") {
			m, e := fakeModel(c.Args)
			must(t, e)
			if strings.Contains(string(mustJSON(m)), "/var/run/docker.sock") {
				t.Fatal("socket mounted into proxy")
			}
		}
	}
	_, e = f.S.Proxy.Stop(context.Background(), J{"confirm": true}, nil)
	must(t, e)
	cs, e = f.S.Docker.Owned(context.Background(), current)
	must(t, e)
	for _, c := range cs {
		if !truth(obj(c)["running"]) {
			t.Fatal("proxy stop stopped app")
		}
	}
}
func TestWatchCancelNoPruneAndNoDuplicate(t *testing.T) {
	f := newFixture(t, false)
	a := f.app(t, "watch", "api", J{"services": J{"api": J{"image": "x", "develop": J{"watch": A{J{"action": "sync", "path": ".", "target": "/app"}}}}}})
	id := str(a["id"])
	f.trust(t, id, "dev")
	assertPass(t, f.act(t, id, "up", nil))
	op, e := f.S.StartWatch(id)
	must(t, e)
	assertPass(t, waitOp(t, f.S, op))
	_, e = f.S.StartWatch(id)
	expectCode(t, e, "WATCH_ACTIVE")
	time.Sleep(10 * time.Millisecond)
	f.S.StopWatch(id)
	seen := false
	for _, c := range f.F.History() {
		if contains(c.Args, "watch") && !contains(c.Args, "--help") {
			seen = true
			if !contains(c.Args, "--no-up") || !contains(c.Args, "--prune=false") {
				t.Fatal("unsafe watch flags")
			}
		}
	}
	if !seen {
		t.Fatal("watch did not run")
	}
}

func mustJSON(v any) []byte {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	return b
}
