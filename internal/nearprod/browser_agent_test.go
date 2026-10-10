package nearprod

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// This entry point exists ONLY inside a go test binary, never a release executable.
func TestBrowserAgent(t *testing.T) {
	if os.Getenv("NEARPROD_BROWSER_TEST") != "1" {
		t.Skip("interactive browser fixture")
	}
	f := newFixture(t, true)
	must(t, f.S.Store.Update(func(v J) error {
		if runtime.GOOS == "darwin" {
			v["runtime"] = J{"kind": "colima", "context": "colima", "profile": "default"}
		} else {
			v["runtime"] = J{"kind": "native", "context": "default", "profile": "default"}
		}
		return nil
	}))
	f.F.Set(func() {
		f.F.Connected = false
		if runtime.GOOS == "darwin" {
			f.F.ColimaRunning = false
			f.F.Endpoint = "unix://" + filepath.Join(userHome(), ".colima", "default", "docker.sock")
		} else {
			f.F.Endpoint = "unix:///var/run/docker.sock"
		}
	})
	for _, name := range []string{"backend", "frontend"} {
		p := filepath.Join(f.Root, "Tienda", name)
		os.MkdirAll(p, 0755)
		model := J{"services": J{"frontend": J{"image": "fixture:web", "expose": A{"80"}}}}
		if name == "backend" {
			model = J{"services": J{"api": J{"image": "fixture:api", "expose": A{"8000"}, "healthcheck": J{"test": A{"CMD", "true"}}}, "db": J{"image": "postgres:17", "expose": A{"5432"}}}}
		}
		writeJSON(filepath.Join(p, "compose.yaml"), model)
		writeJSON(filepath.Join(p, "compose.dev.yaml"), J{"services": J{"debug": J{"image": "fixture:debug", "profiles": A{"debug"}}}})
		os.WriteFile(filepath.Join(p, ".env.local"), []byte("PASSWORD=private123\n"), 0600)
		os.WriteFile(filepath.Join(p, ".env.example"), []byte("PASSWORD=example\n"), 0600)
	}
	if os.Getenv("NEARPROD_BROWSER_PURGE") == "1" {
		f.F.Set(func() { f.F.Connected = true; f.F.ColimaRunning = true })
		for _, name := range []string{"active", "archived", "retry"} {
			app := f.app(t, "delete-demo", name, nil)
			f.trust(t, str(app["id"]), "dev")
			assertPass(t, f.act(t, str(app["id"]), "up", J{"mode": "dev"}))
			f.F.Set(func() {
				volume := str(app["projectName"]) + "_data"
				f.F.Volumes[volume] = J{"Name": volume, "Driver": "local", "CreatedAt": "fixture", "Labels": J{LProject: app["projectName"], LOwner: f.S.Store.Get()["owner"], LStack: app["uid"]}}
				for _, c := range f.F.Containers {
					if str(at(c, "Config", "Labels", LStack)) == str(app["uid"]) {
						c["Mounts"] = A{J{"Type": "volume", "Name": volume}}
					}
				}
			})
			if name == "archived" {
				assertPass(t, f.act(t, str(app["id"]), "stop", J{}))
				f.S.Refresh(context.Background())
				p, e := f.S.ArchiveStackPreview(str(app["id"]))
				must(t, e)
				_, e = f.S.ArchiveStack(context.Background(), J{"target": app["id"], "confirm": true, "fingerprint": p["fingerprint"]})
				must(t, e)
			}
			if name == "retry" {
				f.F.Set(func() { f.F.FailCmd = "volume rm" })
				_, req := reviewedPurge(t, f, app)
				op := executePurge(t, f, req)
				if str(op["state"]) != "failed" {
					t.Fatal("purge failure fixture did not fail")
				}
				f.F.Set(func() { f.F.FailCmd = "" })
			}
		}
		f.S.Refresh(context.Background())
	}
	if os.Getenv("NEARPROD_BROWSER_ACTIONS") == "1" {
		f.F.Set(func() { f.F.Connected = true; f.F.ColimaRunning = true })
		for _, name := range []string{"running", "stopped", "no-check", "new", "starting", "mixed"} {
			var model J
			if name == "no-check" {
				model = J{"services": J{"api": J{"image": "fixture:test"}}}
			}
			if name == "mixed" {
				model = J{"services": J{"api": J{"image": "fixture:test", "healthcheck": J{"test": A{"CMD", "true"}}}, "worker": J{"image": "fixture:worker"}}}
			}
			app := f.app(t, "actions-demo", name, model)
			f.trust(t, str(app["id"]), "dev")
			f.trust(t, str(app["id"]), "verify")
			if name != "new" {
				assertPass(t, f.act(t, str(app["id"]), "up", J{"mode": "dev"}))
			}
			if name == "stopped" {
				assertPass(t, f.act(t, str(app["id"]), "stop", J{}))
			}
			if name == "starting" {
				f.F.Set(func() {
					for _, c := range f.F.Containers {
						if str(at(c, "Config", "Labels", LStack)) == str(app["uid"]) {
							obj(c["State"])["Health"] = J{"Status": "starting"}
						}
					}
				})
			}
		}
		app := f.app(t, "all-running", "ready", nil)
		f.trust(t, str(app["id"]), "dev")
		assertPass(t, f.act(t, str(app["id"]), "up", J{"mode": "dev"}))
		f.S.Refresh(context.Background())
	}
	ln, e := net.Listen("tcp4", "127.0.0.1:0")
	must(t, e)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := strings.Split(r.Host, ":")[0]
		for _, raw := range arr(f.S.Store.Get()["stacks"]) {
			st := obj(raw)
			for _, route := range arr(at(st, "proxyApplied", "routes")) {
				if str(obj(route)["host"]) == host {
					w.Header().Set("X-Nearprod-Route", routeKey(st, host))
					fmt.Fprint(w, `{"fixture":"simulated-traefik"}`)
					return
				}
			}
		}
		http.NotFound(w, r)
	})}
	go server.Serve(ln)
	defer server.Close()
	info := merge(f.A.Info, J{"code": f.A.HTTP.Pair()["code"], "root": f.Root, "home": f.Home, "proxyPort": ln.Addr().(*net.TCPAddr).Port, "host": f.S.Runtime.Capabilities()})
	json.NewEncoder(os.Stdout).Encode(info)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var request J
		if json.Unmarshal(scanner.Bytes(), &request) != nil {
			continue
		}
		switch str(request["action"]) {
		case "connect":
			f.F.Set(func() {
				f.F.Connected = true
				f.F.ColimaRunning = runtime.GOOS == "darwin"
			})
		case "engine-down":
			f.F.Set(func() {
				f.F.Connected = false
				f.F.ColimaRunning = runtime.GOOS == "darwin"
			})
		case "disconnect":
			f.F.Set(func() { f.F.Connected = false; f.F.ColimaRunning = false })
		case "unhealthy":
			f.F.Set(func() {
				for _, c := range f.F.Containers {
					if str(at(c, "Config", "Labels", LService)) == "api" {
						obj(c["State"])["Health"] = J{"Status": "unhealthy"}
					}
				}
			})
		case "break-compose":
			os.WriteFile(filepath.Join(f.Root, "Tienda", "backend", "compose.yaml"), []byte("invalid"), 0600)
		case "shutdown":
			return
		}
		f.S.Refresh(context.Background())
		f.S.emitCatalog()
	}
}
