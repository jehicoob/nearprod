package nearprod

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestAgentUpdatePreservesPortAndSession(t *testing.T) {
	previous := Version
	Version = "1.0.0"
	t.Cleanup(func() { Version = previous })
	f := newFixture(t, true)
	auth := map[string]string{"Authorization": "Bearer " + str(f.A.Info["token"])}
	pair := f.A.HTTP.Pair()
	code, headers, _ := callHTTP(t, f, "POST", "/api/session", J{"code": pair["code"]}, nil)
	if code != 200 {
		t.Fatal(code)
	}
	cookie := map[string]string{"Cookie": strings.Split(headers.Get("Set-Cookie"), ";")[0]}
	url := str(f.A.Info["url"])
	oldBearer := str(f.A.Info["token"])
	code, _, body := callHTTP(t, f, "POST", "/api/agent/stop", J{"forUpdate": true, "version": Version}, auth)
	if code != 200 {
		t.Fatalf("stop: %d %s", code, body)
	}
	must(t, waitAgentStopped(context.Background(), f.Home))
	file := filepath.Join(f.Home, "agent-update.json")
	state, err := os.Stat(file)
	must(t, err)
	if state.Mode().Perm() != 0600 {
		t.Fatal("session handoff is not private")
	}
	if bytes.Contains(body, []byte(strings.TrimPrefix(cookie["Cookie"], "nearprod_session="))) {
		t.Fatal("session leaked through the stop response")
	}
	next, err := StartAgent(context.Background(), f.Home, f.A.HTTP.Assets, f.F, false, 0)
	must(t, err)
	f.A = next
	if str(next.Info["url"]) != url || str(next.Info["token"]) == oldBearer {
		t.Fatal("port changed or CLI bearer did not rotate")
	}
	code, _, _ = callHTTP(t, f, "GET", "/api/catalog", nil, cookie)
	if code != 200 {
		t.Fatal("the existing UI session did not reconnect", code)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("handoff was not consumed")
	}
	code, _, _ = callHTTP(t, f, "GET", "/api/catalog", nil, auth)
	if code != 401 {
		t.Fatal("old CLI bearer remained authorized")
	}
}

func TestAgentUpdateRefusesActiveWatchWithoutStopping(t *testing.T) {
	f := newFixture(t, true)
	done := make(chan struct{})
	close(done)
	f.S.watchMu.Lock()
	f.S.watches["test-watch"] = &watchEntry{Value: J{"state": "running"}, Cancel: func() {}, Done: done}
	f.S.watchMu.Unlock()
	auth := map[string]string{"Authorization": "Bearer " + str(f.A.Info["token"])}
	code, _, body := callHTTP(t, f, "POST", "/api/agent/stop", J{"forUpdate": true, "version": "1.0.0"}, auth)
	if code != 409 || !bytes.Contains(body, []byte("WATCH_ACTIVE")) {
		t.Fatalf("watch was interrupted: %d %s", code, body)
	}
	code, _, _ = callHTTP(t, f, "POST", "/api/pair", J{}, auth)
	if code != 200 || !f.S.HasWatch() {
		t.Fatal("blocked update changed the running agent")
	}
}

func TestAgentStopClosesAdmissionBeforeAcknowledgement(t *testing.T) {
	f := newFixture(t, true)
	f.A.HTTP.OnShutdown = func() {}
	auth := map[string]string{"Authorization": "Bearer " + str(f.A.Info["token"])}
	code, _, _ := callHTTP(t, f, "POST", "/api/agent/stop", J{}, auth)
	if code != 200 {
		t.Fatal(code)
	}
	_, err := f.S.Ops.Submit("late-operation", nil, true, false, func(context.Context, func(string, string)) (any, error) { return J{}, nil })
	expectCode(t, err, "AGENT_STOPPING")
	code, _, _ = callHTTP(t, f, "POST", "/api/pair", J{}, auth)
	if code != 409 {
		t.Fatal("a mutation entered the shutdown window", code)
	}
}

func TestAgentUpdateRefusesActiveOperationWithoutCancelling(t *testing.T) {
	f := newFixture(t, true)
	finish := make(chan struct{})
	t.Cleanup(func() { close(finish) })
	op, err := f.S.Ops.Submit("test-work", nil, true, false, func(ctx context.Context, _ func(string, string)) (any, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-finish:
			return J{}, nil
		}
	})
	must(t, err)
	auth := map[string]string{"Authorization": "Bearer " + str(f.A.Info["token"])}
	code, _, body := callHTTP(t, f, "POST", "/api/agent/stop", J{"forUpdate": true, "version": "1.0.0"}, auth)
	if code != 409 || !bytes.Contains(body, []byte("OPERATION_CONFLICT")) {
		t.Fatalf("operation was interrupted: %d %s", code, body)
	}
	current, err := f.S.Ops.Get(str(op["id"]))
	must(t, err)
	if str(current["state"]) != "running" {
		t.Fatal(current)
	}
}

func TestAgentUpdateDoesNotRestoreExpiredSessionHandoff(t *testing.T) {
	previous := Version
	Version = "1.0.0"
	t.Cleanup(func() { Version = previous })
	f := newFixture(t, true)
	port := integer(f.A.Info["port"])
	f.A.Close()
	session := token(32)
	must(t, writeJSON(filepath.Join(f.Home, "agent-update.json"), J{
		"version": Version, "port": port,
		"expiresAt": time.Now().Add(-time.Minute).Format(time.RFC3339Nano),
		"sessions":  J{session: time.Now().Add(time.Hour).Format(time.RFC3339Nano)},
	}))
	next, err := StartAgent(context.Background(), f.Home, f.A.HTTP.Assets, f.F, false, 0)
	must(t, err)
	f.A = next
	code, _, _ := callHTTP(t, f, "GET", "/api/catalog", nil, map[string]string{"Cookie": "nearprod_session=" + session})
	if code != http.StatusUnauthorized {
		t.Fatal("expired handoff restored a session", code)
	}
}

func newAgentUpdater(t *testing.T, candidate []byte) (*fixture, *Updater, updatePlan) {
	t.Helper()
	previous := Version
	Version = "1.0.0"
	t.Cleanup(func() { Version = previous })
	f := newFixture(t, true)
	home := tempHome(t)
	installed := filepath.Join(home, ".local", "bin", "nearprod")
	must(t, os.MkdirAll(filepath.Dir(installed), 0700))
	must(t, os.WriteFile(installed, updateTestBinary(Version, runtimePlatform()), 0755))
	archive := updateTestArchive(t, map[string][]byte{"nearprod": candidate})
	server, _ := updateTestServer(t, "1.1.0", runtimePlatform(), archive, updateTestDigest(archive))
	t.Cleanup(server.Close)
	u := &Updater{Client: server.Client(), APIURL: server.URL + "/latest", AssetBase: server.URL + "/download", Executable: installed, Home: home, CatalogHome: f.Home, OS: runtimeOS(), Arch: runtimeArch(), AllowedOrigins: map[string]bool{server.URL: true}}
	plan, err := u.Check(context.Background())
	must(t, err)
	return f, u, plan
}

func TestUpdaterRestartsVerifiedBinaryOnSamePort(t *testing.T) {
	f, u, plan := newAgentUpdater(t, updateTestBinary("1.1.0", runtimePlatform()))
	oldURL := str(f.A.Info["url"])
	oldCatalog := hash(f.S.Store.Get())
	calls := 0
	u.StartAgent = func(ctx context.Context, home, executable, version string, port int) (J, error) {
		calls++
		select {
		case <-f.A.Done():
		default:
			t.Fatal("the old agent still owns the catalog")
		}
		if home != f.Home || executable != u.Executable || port != integer(f.A.Info["port"]) || version != plan.Latest {
			t.Fatal("the updater did not select the installed binary and original endpoint")
		}
		_, err := verifyUpdateBinary(ctx, executable, version, runtimePlatform())
		must(t, err)
		Version = version
		next, err := StartAgent(ctx, home, f.A.HTTP.Assets, f.F, false, port)
		must(t, err)
		f.A = next
		return readLive(ctx, home, true)
	}
	result, err := u.Apply(context.Background(), plan)
	must(t, err)
	if calls != 1 || !truth(result["agentRestarted"]) || str(result["agentVersion"]) != plan.Latest || str(result["url"]) != oldURL || !truth(result["uiSessionsPreserved"]) {
		t.Fatal(result)
	}
	if hash(f.A.Service.Store.Get()) != oldCatalog {
		t.Fatal("update changed the catalog")
	}
}

func TestUpdaterRetriesFailedActivationWithoutDownloading(t *testing.T) {
	f, u, plan := newAgentUpdater(t, updateTestBinary("1.1.0", runtimePlatform()))
	u.StartAgent = func(context.Context, string, string, string, int) (J, error) {
		return nil, fail("AGENT_START_FAILED", "test startup failure", 503)
	}
	result, err := u.Apply(context.Background(), plan)
	if err != nil && str(publicError(err)["code"]) != "UPDATE_AGENT_RESTART" {
		t.Logf("unexpected activation failure details: %#v", publicError(err))
	}
	expectCode(t, err, "UPDATE_AGENT_RESTART")
	if str(at(publicError(err), "details", "update", "status")) != "activation-pending" {
		t.Fatal("CLI error details lost the installed-but-not-activated state")
	}
	if str(result["status"]) != "activation-pending" || !truth(result["installed"]) {
		t.Fatal(result)
	}
	Version = plan.Latest
	plan, err = u.Check(context.Background())
	must(t, err)
	if plan.Available || str(plan.Summary["status"]) != "up-to-date" {
		t.Fatal(plan.Summary)
	}
	u.Client = nil // Activation must not download another release.
	u.StartAgent = func(ctx context.Context, home, executable, version string, port int) (J, error) {
		next, err := StartAgent(ctx, home, f.A.HTTP.Assets, f.F, false, port)
		must(t, err)
		f.A = next
		return readLive(ctx, home, true)
	}
	result, err = u.Apply(context.Background(), plan)
	must(t, err)
	if !truth(result["agentRestarted"]) || str(result["agentVersion"]) != plan.Latest {
		t.Fatal(result)
	}
}

func TestUpdaterRestartsOriginalAfterFailedInstall(t *testing.T) {
	f, u, plan := newAgentUpdater(t, updatePathDependentBinary("1.1.0", runtimePlatform()))
	before, err := streamHash(u.Executable)
	must(t, err)
	calls := 0
	u.StartAgent = func(ctx context.Context, home, executable, version string, port int) (J, error) {
		calls++
		after, err := streamHash(executable)
		must(t, err)
		if before != after || version != plan.Current {
			t.Fatal("recovery attempted to start an unverified binary")
		}
		next, err := StartAgent(context.Background(), home, f.A.HTTP.Assets, f.F, false, port)
		must(t, err)
		f.A = next
		return readLive(ctx, home, true)
	}
	_, err = u.Apply(context.Background(), plan)
	expectCode(t, err, "UPDATE_BINARY")
	if calls != 1 {
		t.Fatal("the previous agent was not restarted")
	}
	info, err := readLive(context.Background(), f.Home, true)
	must(t, err)
	if str(info["version"]) != plan.Current {
		t.Fatal("recovery did not restore availability")
	}
}

func TestUpdaterKeepsActiveWorkAndInstalledBinary(t *testing.T) {
	for _, watch := range []bool{false, true} {
		t.Run(map[bool]string{false: "operation", true: "watch"}[watch], func(t *testing.T) {
			f, u, plan := newAgentUpdater(t, updateTestBinary("1.1.0", runtimePlatform()))
			before, err := streamHash(u.Executable)
			must(t, err)
			if watch {
				done := make(chan struct{})
				close(done)
				f.S.watchMu.Lock()
				f.S.watches["update-test"] = &watchEntry{Value: J{"state": "running"}, Done: done, Cancel: func() {}}
				f.S.watchMu.Unlock()
			} else {
				finish := make(chan struct{})
				t.Cleanup(func() { close(finish) })
				_, err = f.S.Ops.Submit("test-work", nil, true, false, func(ctx context.Context, _ func(string, string)) (any, error) {
					select {
					case <-finish:
						return J{}, nil
					case <-ctx.Done():
						return nil, ctx.Err()
					}
				})
				must(t, err)
			}
			u.StartAgent = func(context.Context, string, string, string, int) (J, error) {
				t.Fatal("busy agent was restarted")
				return nil, nil
			}
			_, err = u.Apply(context.Background(), plan)
			expectCode(t, err, map[bool]string{false: "OPERATION_CONFLICT", true: "WATCH_ACTIVE"}[watch])
			after, err := streamHash(u.Executable)
			must(t, err)
			if before != after {
				t.Fatal("busy update changed the installed binary")
			}
			select {
			case <-f.A.Done():
				t.Fatal("busy agent was stopped")
			default:
			}
		})
	}
}

func TestAgentUpdateInvalidHandoffKeepsAgentUsable(t *testing.T) {
	f := newFixture(t, true)
	file := filepath.Join(f.Home, agentRestartFile)
	must(t, os.Symlink(filepath.Join(f.Home, "unrelated"), file))
	auth := map[string]string{"Authorization": "Bearer " + str(f.A.Info["token"])}
	code, _, _ := callHTTP(t, f, "POST", "/api/agent/stop", J{"forUpdate": true, "version": "1.1.0"}, auth)
	if code < 400 {
		t.Fatal("unsafe handoff accepted")
	}
	code, _, _ = callHTTP(t, f, "POST", "/api/pair", J{}, auth)
	if code != 200 {
		t.Fatal("failed preparation left the agent unusable")
	}
}

func TestAgentUpdateRejectsWrongVersionBeforeOpeningCatalog(t *testing.T) {
	previous := Version
	Version = "1.0.0"
	t.Cleanup(func() { Version = previous })
	f := newFixture(t, true)
	port := integer(f.A.Info["port"])
	f.A.Close()
	must(t, writeJSON(filepath.Join(f.Home, agentRestartFile), J{
		"version": "1.1.0", "port": port, "sessions": J{},
		"expiresAt": time.Now().Add(time.Minute).Format(time.RFC3339Nano),
	}))
	_, err := StartAgent(context.Background(), f.Home, f.A.HTTP.Assets, f.F, false, 0)
	expectCode(t, err, "UPDATE_AGENT_VERSION")
	release, err := AcquireAgentLock(f.Home)
	must(t, err)
	release()
}

// Closing the legacy socket and releasing the flock are separate system calls.
// A waiter can observe this state during the normal Agent lock release.
func TestStopWaitsForGuardAfterSocketCloses(t *testing.T) {
	home := tempHome(t)
	guard, err := os.OpenFile(filepath.Join(home, "agent.guard"), os.O_CREATE|os.O_RDWR, 0600)
	must(t, err)
	defer guard.Close()
	must(t, syscall.Flock(int(guard.Fd()), syscall.LOCK_EX|syscall.LOCK_NB))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err = waitAgentStopped(ctx, home)
	if err == nil || str(publicError(err)["code"]) != "AGENT_STOP_PENDING" {
		t.Fatalf("shutdown must remain pending while the catalog guard is held, got %#v", publicError(err))
	}
	must(t, guard.Close())
	must(t, waitAgentStopped(context.Background(), home))
	release, err := AcquireAgentLock(home)
	must(t, err)
	release()
}
