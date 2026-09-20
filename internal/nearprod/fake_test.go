package nearprod

// Contract double ONLY: this does not implement Docker/Compose/SQL semantics.
// Real Engine acceptance is separate and never falls back to this double.
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"nearprod/internal/webui"
)

type fakeCall struct {
	Name       string
	Args       []string
	Dir, Input string
	Env        map[string]string
}
type fakeRunner struct {
	mu                                     sync.Mutex
	Calls                                  []fakeCall
	Connected                              bool
	EngineID, Endpoint, Arch               string
	Containers, Networks, Volumes          map[string]J
	Images                                 map[string]J
	Roles, DBs                             map[string]string
	Redis                                  map[string]string
	Counter                                int
	Streams                                int
	FailProject, Help, FailCmd, SQLFailure string
	SQLFailContains                        string
	ColimaRunning                          bool
	BeforeUp                               func()
	AfterDropDatabase, AfterDropAccount    func()
	BlockUp                                chan struct{}
	FailAfterUp                            bool
}

func newFake() *fakeRunner {
	return &fakeRunner{Connected: true, EngineID: "fixture-engine", Endpoint: "unix:///var/run/docker.sock", Arch: runtime.GOARCH, Containers: map[string]J{}, Networks: map[string]J{}, Volumes: map[string]J{}, Images: map[string]J{}, Roles: map[string]string{}, DBs: map[string]string{}, Redis: map[string]string{}, ColimaRunning: true, Help: "--wait --wait-timeout --no-up --prune --activate --cpu --memory"}
}
func fakeOK(v any) (Result, error) {
	if v == nil {
		return Result{}, nil
	}
	if s, ok := v.(string); ok {
		return Result{Stdout: s}, nil
	}
	b, _ := json.Marshal(v)
	return Result{Stdout: string(b)}, nil
}
func fakeFail(msg string) (Result, error) { return Result{Code: 1, Stderr: msg}, nil }
func flag(args []string, name string) string {
	for n, s := range args {
		if s == name && n+1 < len(args) {
			return args[n+1]
		}
	}
	return ""
}
func flags(args []string, name string) []string {
	out := []string{}
	for n, s := range args {
		if s == name && n+1 < len(args) {
			out = append(out, args[n+1])
		}
	}
	return out
}
func deepMerge(a, b J) J {
	out := copyJ(a)
	for k, v := range b {
		switch v.(type) {
		case J, map[string]any:
			out[k] = deepMerge(obj(out[k]), obj(v))
		default:
			out[k] = v
		}
	}
	return out
}
func unescape(v any) any {
	switch x := v.(type) {
	case string:
		return strings.ReplaceAll(x, "$$", "$")
	case J:
		r := J{}
		for k, v := range x {
			r[k] = unescape(v)
		}
		return r
	case map[string]any:
		return unescape(J(x))
	case []any:
		r := A{}
		for _, v := range x {
			r = append(r, unescape(v))
		}
		return r
	}
	return v
}
func fakeModel(args []string) (J, error) {
	model := J{}
	for _, f := range flags(args, "-f") {
		b, e := os.ReadFile(f)
		if e != nil {
			return nil, e
		}
		v, e := decodeObject(b)
		if e != nil {
			return nil, fmt.Errorf("test fixtures must be JSON Compose: %s", f)
		}
		model = deepMerge(model, obj(unescape(v)))
	}
	return model, nil
}
func (f *fakeRunner) add(project, service, path string, labels J, running bool, health string) J {
	f.Counter++
	id := fmt.Sprintf("%064x", f.Counter)
	label := merge(J{LProject: project, LService: service, LWorking: path}, labels)
	st := J{"Status": "exited", "Running": running, "ExitCode": 0, "OOMKilled": false}
	if running {
		st["Status"] = "running"
	}
	if health != "" {
		st["Health"] = J{"Status": health}
	}
	v := J{"Id": id, "Name": "/" + project + "-" + service + "-1", "Image": "sha256:" + strings.Repeat("a", 64), "Platform": "linux", "Config": J{"Image": "fixture:test", "Labels": label, "Env": A{}}, "State": st, "NetworkSettings": J{"Ports": J{}, "Networks": J{}}, "RestartCount": 0}
	f.Containers[id] = v
	return v
}
func (f *fakeRunner) Add(project, service, path string, labels J, running bool, health string) J {
	f.mu.Lock()
	defer f.mu.Unlock()
	return copyJ(f.add(project, service, path, labels, running, health))
}
func (f *fakeRunner) Set(fn func()) { f.mu.Lock(); defer f.mu.Unlock(); fn() }
func (f *fakeRunner) History() []fakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeCall{}, f.Calls...)
}
func (f *fakeRunner) Run(ctx context.Context, name string, args []string, o RunOptions) (Result, error) {
	if ctx.Err() != nil {
		return Result{}, fail("CANCELLED", "Cancelado.", 409)
	}
	input := ""
	if o.Input != nil {
		b, e := io.ReadAll(io.LimitReader(o.Input, 4<<20))
		if e != nil {
			return Result{}, e
		}
		input = string(b)
	}
	f.mu.Lock()
	f.Calls = append(f.Calls, fakeCall{name, append([]string{}, args...), o.Dir, input, o.Env})
	// Output to callbacks must not hold the state lock: real runner callbacks can query status.
	if o.Stream && (contains(args, "logs") || contains(args, "events") || contains(args, "watch")) {
		f.Streams++
		f.mu.Unlock()
		defer func() { f.mu.Lock(); f.Streams--; f.mu.Unlock() }()
		line := "2026-09-16T01:02:03.000000000Z fixture log PASSWORD=private123"
		if contains(args, "events") {
			line = `{"Type":"container","Action":"start"}`
		}
		if contains(args, "watch") {
			line = "Watch enabled"
		}
		if o.Redact != nil {
			line = o.Redact.Text(line)
		}
		if o.Line != nil {
			o.Line(line, "stdout")
		}
		<-ctx.Done()
		return Result{}, fail("CANCELLED", "Cancelado.", 409)
	}
	if contains(args, "up") && f.BlockUp != nil {
		ch := f.BlockUp
		f.mu.Unlock()
		select {
		case <-ch:
		case <-ctx.Done():
			return Result{}, fail("CANCELLED", "Cancelado.", 409)
		}
		f.mu.Lock()
	}
	if contains(args, "up") && f.BeforeUp != nil {
		fn := f.BeforeUp
		f.BeforeUp = nil
		f.mu.Unlock()
		fn()
		f.mu.Lock()
	}
	defer f.mu.Unlock()
	if f.FailCmd != "" && strings.Contains(name+" "+strings.Join(args, " "), f.FailCmd) {
		return fakeFail("forced failure")
	}
	if filepath.Base(name) == "colima" {
		switch {
		case contains(args, "--help"):
			return fakeOK(f.Help)
		case contains(args, "version"):
			return fakeOK("colima fixture")
		case contains(args, "status"):
			if f.ColimaRunning {
				return fakeOK("colima is running")
			}
			return fakeFail("colima is not running")
		case contains(args, "stop"):
			f.ColimaRunning = false
			f.Connected = false
			return fakeOK(nil)
		case contains(args, "start"):
			f.ColimaRunning = true
			f.Connected = true
			return fakeOK(nil)
		case contains(args, "ssh"):
			return fakeOK("MemTotal: 2097152 kB\nMemAvailable: 1048576 kB\nSwapTotal: 0 kB\nSwapFree: 0 kB")
		}
	}
	if filepath.Base(name) != "docker" {
		return fakeFail("unexpected executable " + name)
	}
	if len(args) > 1 && args[0] == "context" {
		return fakeOK(A{J{"Name": args[len(args)-1], "Endpoints": J{"docker": J{"Host": f.Endpoint}}}})
	}
	if contains(args, "--help") {
		return fakeOK(f.Help)
	}
	if contains(args, "--version") {
		return fakeOK("Docker test")
	}
	if contains(args, "buildx") {
		return fakeOK("buildx test")
	}
	if contains(args, "compose") && contains(args, "version") {
		return fakeOK("5.5.1")
	}
	rest := args
	if len(rest) > 1 && rest[0] == "--context" {
		rest = rest[2:]
	}
	if len(rest) == 0 {
		return fakeFail("empty docker")
	}
	if rest[0] == "compose" && contains(rest, "config") {
		model, e := fakeModel(rest)
		if e != nil {
			return fakeFail(e.Error())
		}
		return fakeOK(model)
	}
	if !f.Connected {
		return fakeFail("Cannot connect to Docker daemon")
	}
	switch rest[0] {
	case "info":
		return fakeOK(J{"ID": f.EngineID, "OSType": "linux", "ServerVersion": "fixture", "NCPU": 2, "MemTotal": 2147483648, "Architecture": f.Arch})
	case "ps":
		out := []string{}
		for _, id := range keysJMap(f.Containers) {
			c := f.Containers[id]
			include := true
			for _, filter := range flags(rest, "--filter") {
				if strings.HasPrefix(filter, "label=") {
					key, value, eq := strings.Cut(strings.TrimPrefix(filter, "label="), "=")
					if at(c, "Config", "Labels", key) == nil || eq && str(at(c, "Config", "Labels", key)) != value {
						include = false
					}
				}
			}
			if include {
				out = append(out, id)
			}
		}
		return fakeOK(strings.Join(out, "\n"))
	case "inspect":
		out := A{}
		for _, c := range f.Containers {
			if contains(rest, str(c["Id"])) || contains(rest, strings.TrimPrefix(str(c["Name"]), "/")) {
				out = append(out, copyJ(c))
			}
		}
		if len(out) == 0 {
			return fakeFail("No such container")
		}
		return fakeOK(out)
	case "network", "volume":
		if len(rest) < 3 {
			return fakeFail("bad resource command")
		}
		coll := f.Networks
		if rest[0] == "volume" {
			coll = f.Volumes
		}
		switch rest[1] {
		case "inspect":
			v := coll[rest[2]]
			if v == nil {
				return fakeFail("No such " + rest[0])
			}
			return fakeOK(A{v})
		case "create":
			n := rest[len(rest)-1]
			ls := J{}
			for _, l := range flags(rest, "--label") {
				k, v, _ := strings.Cut(l, "=")
				ls[k] = v
			}
			coll[n] = J{"Id": "resource-" + n, "Name": n, "Labels": ls, "Driver": "bridge"}
			return fakeOK(n)
		case "rm":
			delete(coll, rest[len(rest)-1])
			return fakeOK(nil)
		}
	case "image":
		id := rest[len(rest)-1]
		if f.Images[id] != nil {
			return fakeOK(A{f.Images[id]})
		}
		return fakeOK(A{J{"Id": "sha256:" + strings.Repeat("a", 64), "Os": "linux", "Architecture": f.Arch, "RepoDigests": A{strings.Split(id, ":")[0] + "@sha256:" + strings.Repeat("b", 64)}}})
	case "pull":
		return fakeOK("pulled")
	case "stop", "restart":
		for _, c := range f.Containers {
			if contains(rest, str(c["Id"])) {
				st := obj(c["State"])
				st["Running"] = rest[0] == "restart"
				st["Status"] = "exited"
				if rest[0] == "restart" {
					st["Status"] = "running"
				}
			}
		}
		return fakeOK(nil)
	case "rm":
		for id, c := range f.Containers {
			if contains(rest, id) || contains(rest, strings.TrimPrefix(str(c["Name"]), "/")) {
				delete(f.Containers, id)
			}
		}
		return fakeOK(nil)
	case "stats":
		rows := []string{}
		for _, c := range f.Containers {
			if truth(at(c, "State", "Running")) {
				b, _ := json.Marshal(J{"ID": c["Id"], "Name": strings.TrimPrefix(str(c["Name"]), "/"), "CPUPerc": "0.10%", "MemUsage": "32MiB / 2GiB", "MemPerc": "1.5%"})
				rows = append(rows, string(b))
			}
		}
		return fakeOK(strings.Join(rows, "\n"))
	case "logs":
		line := "2026-09-16T01:02:03.000000000Z fixture log PASSWORD=private123"
		if o.Redact != nil {
			line = o.Redact.Text(line)
		}
		if o.Line != nil {
			o.Line(line, "stdout")
		}
		return fakeOK(line)
	case "exec":
		if o.Output != nil {
			_, e := o.Output.Write([]byte("FAKE DUMP CONTENT\x00\xff"))
			return Result{}, e
		}
		if f.SQLFailure != "" {
			return fakeFail(f.SQLFailure)
		}
		if f.SQLFailContains != "" && strings.Contains(input, f.SQLFailContains) {
			f.SQLFailContains = ""
			return fakeFail("forced SQL failure")
		}
		if contains(rest, "ACL") {
			return fakeOK("OK")
		}
		if strings.Contains(input, "rolcanlogin") {
			m := regexp.MustCompile(`rolname='([^']+)'`).FindStringSubmatch(input)
			if len(m) > 1 && f.Roles[m[1]] != "" {
				return fakeOK("1")
			}
			return fakeOK("0")
		}
		if strings.Contains(input, "pg_database WHERE datdba=") {
			userMatch := regexp.MustCompile(`rolname='([^']+)'`).FindStringSubmatch(input)
			nameMatch := regexp.MustCompile(`datname<>'([^']+)'`).FindStringSubmatch(input)
			count := 0
			if len(userMatch) > 1 && len(nameMatch) > 1 {
				for name, owner := range f.DBs {
					if owner == userMatch[1] && name != nameMatch[1] {
						count++
					}
				}
			}
			return fakeOK(str(count))
		}
		if strings.Contains(input, "SELECT count(*)") || strings.Contains(input, "SELECT COUNT(*)") {
			return fakeOK("0")
		}
		if strings.Contains(input, "SELECT SCHEMA_NAME") {
			m := regexp.MustCompile(`SCHEMA_NAME='([^']+)'`).FindStringSubmatch(input)
			if len(m) > 1 {
				if _, ok := f.DBs[m[1]]; ok {
					return fakeOK(m[1])
				}
			}
			return fakeOK("")
		}
		if strings.Contains(input, "SELECT CONCAT(User,'@',Host)") {
			m := regexp.MustCompile(`User='([^']+)'`).FindStringSubmatch(input)
			if len(m) > 1 && f.Roles[m[1]] != "" {
				return fakeOK(m[1] + "@%")
			}
			return fakeOK("")
		}
		if strings.Contains(input, "SHOW GRANTS FOR") {
			m := regexp.MustCompile(`SHOW GRANTS FOR '([^']+)'@'%'`).FindStringSubmatch(input)
			if len(m) > 1 && f.Roles[m[1]] != "" {
				for name, owner := range f.DBs {
					if owner == m[1] {
						return fakeOK("GRANT USAGE ON *.* TO '" + m[1] + "'@'%'\nGRANT ALL PRIVILEGES ON `" + name + "`.* TO '" + m[1] + "'@'%'")
					}
				}
			}
			return fakeOK("")
		}
		if strings.Contains(input, "SELECT rolname") {
			m := regexp.MustCompile(`rolname='([^']+)'`).FindStringSubmatch(input)
			if len(m) > 1 {
				return fakeOK(f.Roles[m[1]])
			}
		}
		if strings.Contains(input, "pg_get_userbyid") {
			m := regexp.MustCompile(`datname='([^']+)'`).FindStringSubmatch(input)
			if len(m) > 1 {
				return fakeOK(f.DBs[m[1]])
			}
		}
		for _, m := range regexp.MustCompile(`(?:CREATE|ALTER) ROLE "([^"]+)"`).FindAllStringSubmatch(input, -1) {
			f.Roles[m[1]] = m[1]
		}
		for _, m := range regexp.MustCompile(`CREATE DATABASE "([^"]+)" OWNER "([^"]+)"`).FindAllStringSubmatch(input, -1) {
			f.DBs[m[1]] = m[2]
		}
		mysqlUser := regexp.MustCompile(`CREATE USER IF NOT EXISTS '([^']+)'@'%'`).FindStringSubmatch(input)
		for _, m := range regexp.MustCompile("CREATE DATABASE IF NOT EXISTS `([^`]+)`").FindAllStringSubmatch(input, -1) {
			owner := ""
			if len(mysqlUser) > 1 {
				owner = mysqlUser[1]
			}
			f.DBs[m[1]] = owner
		}
		if len(mysqlUser) > 1 {
			f.Roles[mysqlUser[1]] = mysqlUser[1]
		}
		for _, m := range regexp.MustCompile(`DROP DATABASE IF EXISTS "([^"]+)"`).FindAllStringSubmatch(input, -1) {
			delete(f.DBs, m[1])
			if f.AfterDropDatabase != nil {
				f.AfterDropDatabase()
			}
		}
		for _, m := range regexp.MustCompile(`DROP ROLE IF EXISTS "([^"]+)"`).FindAllStringSubmatch(input, -1) {
			delete(f.Roles, m[1])
			if f.AfterDropAccount != nil {
				f.AfterDropAccount()
			}
		}
		for _, m := range regexp.MustCompile("DROP DATABASE IF EXISTS `([^`]+)`").FindAllStringSubmatch(input, -1) {
			delete(f.DBs, m[1])
			if f.AfterDropDatabase != nil {
				f.AfterDropDatabase()
			}
		}
		for _, m := range regexp.MustCompile(`DROP USER IF EXISTS '([^']+)'@'%'`).FindAllStringSubmatch(input, -1) {
			delete(f.Roles, m[1])
			if f.AfterDropAccount != nil {
				f.AfterDropAccount()
			}
		}
		return fakeOK("")
	case "run":
		if f.SQLFailure != "" {
			return fakeFail(f.SQLFailure)
		}
		if o.Output != nil {
			_, e := o.Output.Write([]byte("TEST"))
			return Result{}, e
		}
		if contains(rest, "SET") {
			key, value := flag(rest, "SET"), ""
			for n, x := range rest {
				if x == "SET" && n+2 < len(rest) {
					value = rest[n+2]
				}
			}
			f.Redis[key] = value
			return fakeOK("OK")
		}
		if contains(rest, "GET") {
			return fakeOK(f.Redis[flag(rest, "GET")])
		}
		if contains(rest, "DEL") {
			delete(f.Redis, flag(rest, "DEL"))
			return fakeOK("1")
		}
		return fakeOK("1")
	case "compose":
		if contains(rest, "up") {
			project := flag(rest, "--project-name")
			if project == f.FailProject && !f.FailAfterUp {
				return fakeFail("fixture build failed")
			}
			model, e := fakeModel(rest)
			if e != nil {
				return fakeFail(e.Error())
			}
			selected := rest[len(rest)-1]
			if at(model, "services", selected) == nil {
				selected = ""
			}
			names, e := activeServices(model, flags(rest, "--profile"), selected)
			if e != nil {
				return Result{}, e
			}
			for _, n := range names {
				spec := obj(at(model, "services", n))
				var c J
				for _, v := range f.Containers {
					if str(at(v, "Config", "Labels", LProject)) == project && str(at(v, "Config", "Labels", LService)) == n {
						c = v
						break
					}
				}
				health := ""
				if spec["healthcheck"] != nil {
					health = "healthy"
				}
				if c == nil {
					c = f.add(project, n, flag(rest, "--project-directory"), obj(spec["labels"]), true, health)
				} else {
					obj(c["Config"])["Labels"] = merge(obj(at(c, "Config", "Labels")), obj(spec["labels"]))
					obj(c["State"])["Running"] = true
					obj(c["State"])["Status"] = "running"
				}
				nets := J{}
				for key, v := range networksOf(spec) {
					netname := text(at(model, "networks", key, "name"), project+"_"+key)
					nets[netname] = J{"Aliases": append(A{n}, arr(at(v, "aliases"))...)}
				}
				ports := J{}
				for _, v := range arr(spec["ports"]) {
					p := obj(v)
					if len(p) > 0 {
						ports[str(p["target"])+"/"+text(p["protocol"], "tcp")] = A{J{"HostIp": text(p["host_ip"], "0.0.0.0"), "HostPort": str(p["published"])}}
					}
				}
				c["NetworkSettings"] = J{"Networks": nets, "Ports": ports}
				env := A{}
				for _, k := range keys(obj(spec["environment"])) {
					env = append(env, k+"="+str(at(spec, "environment", k)))
				}
				obj(c["Config"])["Env"] = env
				obj(c["Config"])["Image"] = spec["image"]
			}
			if project == f.FailProject && f.FailAfterUp {
				return fakeFail("healthcheck failed after creation")
			}
			return fakeOK("Started " + project)
		}
	}
	return fakeFail("unhandled test command " + strings.Join(args, " "))
}
func keysJMap(v map[string]J) []string {
	j := J{}
	for k := range v {
		j[k] = nil
	}
	return keys(j)
}

type fixture struct {
	Home, Root, Dir string
	S               *Service
	F               *fakeRunner
	A               *Agent
}

func newFixture(t *testing.T, agent bool) *fixture {
	t.Helper()
	dir, e := os.MkdirTemp("", "np-test-")
	if e != nil {
		t.Fatal(e)
	}
	dir, _ = filepath.EvalSymlinks(dir)
	f := &fixture{Dir: dir, Home: filepath.Join(dir, "home"), Root: filepath.Join(dir, "Projects"), F: newFake()}
	os.MkdirAll(f.Root, 0700)
	if agent {
		assets, _ := fs.Sub(webui.Files, "dist")
		f.A, e = StartAgent(context.Background(), f.Home, assets, f.F, false, 0)
		if e == nil {
			f.S = f.A.Service
		}
	} else {
		var st *Store
		st, e = OpenStore(f.Home)
		if e == nil {
			f.S = NewService(context.Background(), st, f.F)
		}
	}
	if e != nil {
		os.RemoveAll(dir)
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if f.A != nil {
			f.A.Close()
		} else {
			f.S.Close()
		}
		os.RemoveAll(dir)
	})
	must(t, f.S.Store.Update(func(v J) error {
		v["runtime"] = J{"kind": "native", "context": "default", "profile": "default"}
		return nil
	}))
	_, e = f.S.AddRoot(f.Root)
	must(t, e)
	f.S.Proxy.PortCheck = func(int) J { return J{"available": true} }
	f.S.Infra.PortCheck = f.S.Proxy.PortCheck
	return f
}
func must(t *testing.T, e error) {
	t.Helper()
	if e != nil {
		t.Fatal(e)
	}
}
func expectCode(t *testing.T, e error, code string) {
	t.Helper()
	if e == nil || str(publicError(e)["code"]) != code {
		t.Fatalf("want %s, got %v", code, e)
	}
}
func (f *fixture) app(t *testing.T, group, name string, model J) J {
	t.Helper()
	if model == nil {
		model = J{"services": J{"api": J{"image": "fixture:test", "healthcheck": J{"test": A{"CMD", "true"}}}}}
	}
	path := filepath.Join(f.Root, group, name)
	must(t, os.MkdirAll(path, 0700))
	must(t, writeJSON(filepath.Join(path, "compose.yaml"), model))
	s, e := f.S.Register(J{"product": group, "slug": name, "projectName": group + "-" + name, "path": path, "modes": J{"dev": J{"files": A{"compose.yaml"}}, "verify": J{"files": A{"compose.yaml"}}}}, false)
	must(t, e)
	return s
}
func (f *fixture) trust(t *testing.T, id, mode string) {
	t.Helper()
	p, e := f.S.Preview(context.Background(), id, mode)
	must(t, e)
	_, e = f.S.Trust(context.Background(), id, J{"mode": mode, "fingerprint": p["fingerprint"], "allowUnsafe": true})
	must(t, e)
}
func waitOp(t *testing.T, s *Service, op J) J {
	t.Helper()
	end := time.Now().Add(40 * time.Second)
	for time.Now().Before(end) {
		v, e := s.Ops.Get(str(op["id"]))
		must(t, e)
		if !contains([]string{"running", "queued"}, str(v["state"])) {
			return v
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("operation timed out")
	return nil
}
func (f *fixture) act(t *testing.T, id, action string, req J) J {
	t.Helper()
	if req == nil {
		req = J{}
	}
	o, e := f.S.Action(id, action, req)
	must(t, e)
	return waitOp(t, f.S, o)
}
func assertPass(t *testing.T, op J) {
	t.Helper()
	if str(op["state"]) != "succeeded" {
		b, _ := json.MarshalIndent(op, "", "  ")
		t.Fatal(string(b))
	}
}
func (f *fixture) createInstance(t *testing.T, engine string) J {
	t.Helper()
	req := J{"engine": engine, "id": engine + "-main", "persistence": J{"kind": "volume"}}
	p, e := f.S.Infra.Preview(context.Background(), req)
	must(t, e)
	req["confirm"] = true
	req["fingerprint"] = p["fingerprint"]
	_, e = f.S.Infra.Create(context.Background(), req, nil)
	must(t, e)
	r, e := f.S.Infra.Instance(engine + "-main")
	must(t, e)
	return r
}
