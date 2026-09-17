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
		v["runtime"] = J{"kind": "colima", "context": "colima", "profile": "default"}
		return nil
	}))
	f.F.Set(func() {
		f.F.Connected = false
		f.F.ColimaRunning = false
		f.F.Endpoint = "unix://" + filepath.Join(userHome(), ".colima", "default", "docker.sock")
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
	info := merge(f.A.Info, J{"code": f.A.HTTP.Pair()["code"], "root": f.Root, "home": f.Home, "proxyPort": ln.Addr().(*net.TCPAddr).Port})
	json.NewEncoder(os.Stdout).Encode(info)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var request J
		if json.Unmarshal(scanner.Bytes(), &request) != nil {
			continue
		}
		switch str(request["action"]) {
		case "connect":
			f.F.Set(func() { f.F.Connected = true; f.F.ColimaRunning = true })
		case "engine-down":
			f.F.Set(func() { f.F.Connected = false; f.F.ColimaRunning = true })
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
