package nearprod

// Opt-in acceptance against a REAL Engine. No runner injection or mock fallback.
import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"nearprod/internal/acceptanceassets"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type acceptance struct {
	ctx                    context.Context
	s                      *Service
	out                    io.Writer
	result                 J
	root, engine, endpoint string
	port                   int
	prefix                 string
}

func (a *acceptance) step(name string, fn func() error) error {
	start := time.Now()
	fmt.Fprintln(a.out, "Comprobando:", name)
	e := fn()
	v := J{"name": name, "durationMs": time.Since(start).Milliseconds(), "state": "passed"}
	if e != nil {
		v["state"] = "failed"
		v["error"] = publicError(e)
	}
	a.result["checks"] = append(arr(a.result["checks"]), v)
	return e
}
func SelfTest(ctx context.Context, args cliArgs, out io.Writer) (any, error) {
	if !args.B("yes") {
		return J{"confirmationRequired": true, "effect": "Crea proyectos temporales, descarga/construye imágenes y borra SOLO sus contenedores/redes/volúmenes/carpetas de prueba. No cambia Colima ni el catálogo habitual. Imágenes/caché permanecen.", "command": "nearprod self-test --yes --context colima [--infra-only] [--folder]"}, nil
	}
	contextName := args.S("context")
	if !validID(contextName) {
		return nil, fail("USAGE", "Selecciona un contexto local explícito con --context.", 400)
	}
	output := args.S("output")
	if output == "" {
		output = "nearprod-acceptance-" + time.Now().UTC().Format("20060102-150405") + "-" + token(3)
	}
	output, e := filepath.Abs(output)
	if e != nil {
		return nil, e
	}
	if e = privateDir(output); e != nil {
		return nil, e
	}
	report := J{"version": Version, "runtime": "Go", "state": "running", "startedAt": now(), "context": contextName, "checks": A{}, "folder": args.B("folder"), "infraOnly": args.B("infra-only"), "evidence": "Docker real; no simulación. No prueba launchd, Homebrew ni navegador macOS."}
	reportFile := filepath.Join(output, "result.json")
	if _, e = regularOrMissing(reportFile); e != nil {
		return nil, e
	}
	if _, e = os.Stat(reportFile); e == nil {
		return nil, fail("REPORT_EXISTS", "Elige un directorio nuevo para no sobrescribir evidencia.", 409)
	}
	finish := func(err error) (any, error) {
		report["finishedAt"] = now()
		if err != nil {
			report["state"] = "failed"
			report["error"] = publicError(err)
			if str(publicError(err)["code"]) == "TOOL_MISSING" {
				report["state"] = "blocked"
			}
		} else {
			report["state"] = "passed"
		}
		we := writeJSON(reportFile, report)
		if we != nil && err == nil {
			err = we
		}
		return J{"report": reportFile, "state": report["state"], "checks": len(arr(report["checks"])), "version": Version}, err
	}
	if findExecutable("docker") == "" {
		return finish(fail("TOOL_MISSING", "Docker no está instalado. Cero pruebas reales ejecutadas.", 503))
	}
	root, e := os.MkdirTemp(userHome(), ".np-check-")
	if e != nil {
		return finish(e)
	}
	root, _ = filepath.EvalSymlinks(root)
	home := filepath.Join(root, "catalog")
	st, e := OpenStore(home)
	if e != nil {
		os.RemoveAll(root)
		return finish(e)
	}
	e = st.Update(func(v J) error {
		v["runtime"] = J{"kind": "native", "context": contextName, "profile": "default"}
		return nil
	})
	if e != nil {
		os.RemoveAll(root)
		return finish(e)
	}
	s := NewService(ctx, st, nil)
	defer s.Close()
	a := &acceptance{ctx: ctx, s: s, out: out, result: report, root: root, prefix: "np-" + hash(token(8))[:8]}
	report["temporaryDirectory"] = root
	e = a.step("Engine Linux local identificado", func() error {
		info, e := s.Docker.Info(ctx)
		if e != nil {
			return e
		}
		a.engine = str(info["id"])
		a.endpoint = str(info["endpoint"])
		report["engine"] = info
		return nil
	})
	if e != nil {
		os.RemoveAll(root)
		return finish(e)
	}
	e = a.step("Compose con --wait y --wait-timeout", func() error {
		if !s.Compose.Supports(ctx, "up", "--wait-timeout") {
			return fail("COMPOSE_CAPABILITY", "Actualiza/instala Compose.", 409)
		}
		return nil
	})
	if e == nil && !args.B("infra-only") {
		e = a.webAcceptance()
	}
	if e == nil {
		for _, engine := range []string{"postgres", "mysql", "redis"} {
			if e = a.databaseAcceptance(engine, args.B("folder")); e != nil {
				break
			}
		}
	}
	s.Close()
	// Cleanup is part of acceptance. Never hide an error or touch another Engine.
	clean := a.step("Limpieza de recursos temporales con propietario y Engine verificados", a.cleanup)
	if clean != nil {
		report["cleanupError"] = publicError(clean)
		if e == nil {
			e = clean
		}
	} else {
		if de := os.RemoveAll(root); de != nil {
			report["cleanupDirectoryError"] = de.Error()
			if e == nil {
				e = fail("CLEANUP_DIRECTORY", "Recursos Docker retirados; no se pudo borrar la carpeta temporal. Revisa el informe.", 422)
			}
		}
	}
	return finish(e)
}
func (a *acceptance) approve(id, mode string) error {
	p, e := a.s.Preview(a.ctx, id, mode)
	if e != nil {
		return e
	}
	_, e = a.s.Trust(a.ctx, id, J{"mode": mode, "fingerprint": p["fingerprint"], "allowUnsafe": true})
	return e
}
func (a *acceptance) action(id, action string, req J) error {
	op, e := a.s.Action(id, action, req)
	if e != nil {
		return e
	}
	for {
		v, e := a.s.Ops.Get(str(op["id"]))
		if e != nil {
			return e
		}
		switch str(v["state"]) {
		case "succeeded":
			return nil
		case "failed", "cancelled", "interrupted":
			return detailed("ACCEPTANCE_OPERATION", "Falló una operación de aceptación.", 422, J{"operation": v})
		}
		select {
		case <-a.ctx.Done():
			return a.ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
	}
}
func availableEphemeral() (int, error) {
	ln, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		return 0, e
	}
	p := ln.Addr().(*net.TCPAddr).Port
	return p, ln.Close()
}
func (a *acceptance) copyExample(name string) (string, error) {
	dest := filepath.Join(a.root, "projects", name)
	source := "examples/" + name
	e := fs.WalkDir(acceptanceassets.Files, source, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(p, source), "/")
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		b, e := fs.ReadFile(acceptanceassets.Files, p)
		if e != nil {
			return e
		}
		return os.WriteFile(target, b, 0644)
	})
	return dest, e
}
func (a *acceptance) register(name, path string, routes A, devFiles A) (J, error) {
	if _, e := a.s.AddRoot(filepath.Join(a.root, "projects")); e != nil {
		return nil, e
	}
	if len(devFiles) == 0 {
		devFiles = A{"compose.yaml"}
	}
	return a.s.Register(J{"product": a.prefix, "slug": name, "projectName": a.prefix + "-" + name, "path": path, "modes": J{"dev": J{"files": devFiles}, "verify": J{"files": A{"compose.yaml"}}}, "routes": routes}, false)
}
func (a *acceptance) request(host, path, origin string) (int, http.Header, string, error) {
	req, e := http.NewRequestWithContext(a.ctx, "GET", fmt.Sprintf("http://127.0.0.1:%d%s", a.port, path), nil)
	if e != nil {
		return 0, nil, "", e
	}
	req.Host = fmt.Sprintf("%s:%d", host, a.port)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	res, e := localClient(5 * time.Second).Do(req)
	if e != nil {
		return 0, nil, "", e
	}
	defer res.Body.Close()
	b, e := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	return res.StatusCode, res.Header, string(b), e
}
func (a *acceptance) waitHTTP(host, path, body string) error {
	end := time.Now().Add(60 * time.Second)
	for time.Now().Before(end) {
		code, _, b, e := a.request(host, path, "")
		if e == nil && code == 200 && strings.Contains(b, body) {
			return nil
		}
		select {
		case <-a.ctx.Done():
			return a.ctx.Err()
		case <-time.After(700 * time.Millisecond):
		}
	}
	return fail("ACCEPTANCE_HTTP", "No respondió HTTP 200 con el contenido esperado: "+host+path, 422)
}
func (a *acceptance) webAcceptance() error {
	var e error
	a.port, e = availableEphemeral()
	if e != nil {
		return e
	}
	if e = a.step("Traefik real compartido en puerto temporal loopback", func() error {
		r := J{"port": a.port}
		p, e := a.s.Proxy.Preview(a.ctx, r)
		if e != nil {
			return e
		}
		r["confirm"] = true
		r["fingerprint"] = p["fingerprint"]
		_, e = a.s.Proxy.Start(a.ctx, r, nil)
		return e
	}); e != nil {
		return e
	}
	path, e := a.copyExample("fullstack")
	if e != nil {
		return e
	}
	front := a.prefix + ".localhost"
	api := "api-" + front
	// Fixture-local override, not modifications to any user project.
	extras := J{"services": J{"frontend": J{"environment": J{"API_URL": routeURL(api, a.port)}}, "api": J{"environment": J{"FRONTEND_URL": routeURL(front, a.port)}}}}
	if e = writeCompose(filepath.Join(path, "acceptance.json"), extras); e != nil {
		return e
	}
	app, e := a.register("fullstack", path, A{J{"host": front, "service": "frontend", "port": 3000}, J{"host": api, "service": "api", "port": 3000}}, A{"compose.yaml", "acceptance.json"})
	if e != nil {
		return e
	}
	id := str(app["id"])
	if e = a.step("Frontend y API: construcción, aprobación, arranque y dos rutas", func() error {
		if e := a.approve(id, "dev"); e != nil {
			return e
		}
		if e := a.action(id, "up", J{"wait": true}); e != nil {
			return e
		}
		if e := a.waitHTTP(front, "/", "Frontend de prueba"); e != nil {
			return e
		}
		return a.waitHTTP(api, "/api/message", "nearprod-api")
	}); e != nil {
		return e
	}
	if e = a.step("Origen CORS preciso y host ajeno no publicado", func() error {
		code, h, _, e := a.request(api, "/api/message", routeURL(front, a.port))
		if e != nil {
			return e
		}
		if code != 200 || h.Get("Access-Control-Allow-Origin") != routeURL(front, a.port) {
			return fail("ACCEPTANCE_CORS", "La API no devolvió el origen preciso.", 422)
		}
		code, _, _, e = a.request("unregistered-"+front, "/", "")
		if e != nil {
			return e
		}
		if code != 404 {
			return fail("ACCEPTANCE_UNKNOWN_HOST", "Un host ajeno obtuvo ruta.", 422)
		}
		return nil
	}); e != nil {
		return e
	}
	if e = a.step("Logs de contenedores reales, stop externo, reconciliación, restart sin YAML", func() error {
		lines := 0
		if e := a.s.Logs(a.ctx, id, J{"tail": 30}, func(J) { lines++ }); e != nil {
			return e
		}
		if lines == 0 {
			return fail("ACCEPTANCE_LOGS", "No hubo logs.", 422)
		}
		current, _ := a.s.Store.Stack(id)
		cs, e := a.s.Docker.Owned(a.ctx, current)
		if e != nil {
			return e
		}
		if len(cs) == 0 {
			return fail("ACCEPTANCE_CONTAINERS", "Faltan contenedores.", 422)
		}
		if _, e = a.s.Docker.Call(a.ctx, []string{"stop", str(obj(cs[0])["id"])}, RunOptions{}); e != nil {
			return e
		}
		obs := a.s.Refresh(a.ctx)
		_ = obs
		bad := filepath.Join(path, "compose.yaml")
		if e = os.Rename(bad, bad+".saved"); e != nil {
			return e
		}
		defer os.Rename(bad+".saved", bad)
		if e = a.action(id, "restart", J{}); e != nil {
			return e
		}
		return a.waitHTTP(front, "/", "Frontend de prueba")
	}); e != nil {
		return e
	}
	if e = a.action(id, "stop", J{}); e != nil {
		return e
	}
	for _, spec := range []struct {
		name, service    string
		port             int
		source, old, new string
	}{{"python-api", "api", 8000, "app/main.py", "nearprod-python-v1", "nearprod-python-v2"}, {"php-web", "web", 80, "src/index.php", "nearprod-php-v1", "nearprod-php-v2"}, {"elixir", "web", 4000, "", "", ""}} {
		path, e := a.copyExample(spec.name)
		if e != nil {
			return e
		}
		// Remove fixed demo host publication only in this private fixture copy.
		if spec.name != "elixir" {
			b, e := os.ReadFile(filepath.Join(path, "compose.yaml"))
			if e != nil {
				return e
			}
			lines := strings.Split(string(b), "\n")
			keep := []string{}
			skip := false
			for _, l := range lines {
				if strings.TrimSpace(l) == "ports:" {
					skip = true
					continue
				}
				if skip && strings.HasPrefix(l, "      -") {
					continue
				}
				skip = false
				keep = append(keep, l)
			}
			if e = os.WriteFile(filepath.Join(path, "compose.yaml"), []byte(strings.Join(keep, "\n")), 0600); e != nil {
				return e
			}
		}
		dev := A{"compose.yaml"}
		if spec.name != "elixir" {
			dev = append(dev, "compose.dev.yaml")
		}
		host := spec.name + "-" + front
		st, e := a.register(spec.name, path, A{J{"host": host, "service": spec.service, "port": spec.port}}, dev)
		if e != nil {
			return e
		}
		sid := str(st["id"])
		if e = a.step(spec.name+": imagen, arranque y acceso HTTP por Traefik", func() error {
			if e := a.approve(sid, "dev"); e != nil {
				return e
			}
			if e := a.action(sid, "up", J{"wait": true}); e != nil {
				return e
			}
			return a.waitHTTP(host, "/", "")
		}); e != nil {
			return e
		}
		if spec.source != "" {
			if e = a.step(spec.name+": recarga dev y código empaquetado aislado en verify", func() error {
				file := filepath.Join(path, spec.source)
				b, e := os.ReadFile(file)
				if e != nil {
					return e
				}
				if !strings.Contains(string(b), spec.old) {
					return fail("FIXTURE_CHANGED", "El fixture no contiene la marca de recarga esperada.", 422)
				}
				if e = os.WriteFile(file, []byte(strings.ReplaceAll(string(b), spec.old, spec.new)), 0600); e != nil {
					return e
				}
				if e = a.waitHTTP(host, "/", spec.new); e != nil {
					return e
				}
				if e = os.WriteFile(file, b, 0600); e != nil {
					return e
				}
				if e = a.approve(sid, "verify"); e != nil {
					return e
				}
				if e = a.action(sid, "rebuild", J{"mode": "verify", "wait": true, "confirmMode": true}); e != nil {
					return e
				}
				if e = a.waitHTTP(host, "/", spec.old); e != nil {
					return e
				}
				if e = os.WriteFile(file, []byte(strings.ReplaceAll(string(b), spec.old, spec.new)), 0600); e != nil {
					return e
				}
				return a.waitHTTP(host, "/", spec.old)
			}); e != nil {
				return e
			}
		}
		if e = a.action(sid, "stop", J{}); e != nil {
			return e
		}
	}
	return nil
}
func (a *acceptance) databaseAcceptance(engine string, folder bool) error {
	id := "test-" + engine
	kind := "volume"
	if folder {
		kind = "folder"
	}
	var r, db, other J
	if e := a.step(engine+": crear instancia persistente y cuenta limitada", func() error {
		req := J{"id": id, "engine": engine, "persistence": J{"kind": kind}}
		p, e := a.s.Infra.Preview(a.ctx, req)
		if e != nil {
			return e
		}
		req["confirm"] = true
		req["fingerprint"] = p["fingerprint"]
		if _, e = a.s.Infra.Create(a.ctx, req, nil); e != nil {
			return e
		}
		r, e = a.s.Infra.Instance(id)
		if e != nil {
			return e
		}
		v, e := a.s.Infra.CreateDatabase(a.ctx, J{"instance": id, "name": "app_a", "confirm": true}, nil)
		db = obj(v["database"])
		return e
	}); e != nil {
		return e
	}
	query := "CREATE TABLE acceptance_persistence (value varchar(64)); INSERT INTO acceptance_persistence VALUES ('nearprod-persisted');"
	if engine == "redis" {
		_, e := a.s.Infra.Client(a.ctx, db, r, "", []string{"SET", "np-acceptance-persistence", "nearprod-persisted"})
		if e != nil {
			return e
		}
	} else {
		if _, e := a.s.Infra.Client(a.ctx, db, r, query, nil); e != nil {
			return e
		}
	}
	if engine != "redis" {
		if e := a.step(engine+": añadir segunda base después de iniciar y denegar cuenta A en B", func() error {
			v, e := a.s.Infra.CreateDatabase(a.ctx, J{"instance": id, "name": "app_b", "confirm": true}, nil)
			if e != nil {
				return e
			}
			other = obj(v["database"])
			probe, e := a.s.Infra.Probe(a.ctx, str(other["id"]))
			if e != nil || !truth(probe["authenticated"]) {
				return fail("ACCEPTANCE_SECOND_DATABASE", "No se confirmó la segunda base.", 422)
			}
			wrong := copyJ(db)
			wrong["name"] = other["name"]
			res, e := a.s.Infra.Client(a.ctx, wrong, r, "SELECT 1;", nil)
			msg := strings.ToLower(res.Stderr + res.Stdout + fmt.Sprint(e))
			if e == nil || !(strings.Contains(msg, "permission denied") || strings.Contains(msg, "access denied")) {
				return fail("ACCEPTANCE_ISOLATION", "No se verificó una denegación de permisos (un fallo de red no es prueba de aislamiento).", 422)
			}
			return nil
		}); e != nil {
			return e
		}
	}
	if e := a.step(engine+": conservar dato tras stop, recreación e inicio", func() error {
		cs, e := a.s.Infra.Containers(a.ctx, r)
		if e != nil {
			return e
		}
		for _, v := range cs {
			c := obj(v)
			if str(at(c, "labels", LOwner)) != str(a.s.Store.Get()["owner"]) {
				return fail("ACCEPTANCE_OWNER", "Identidad inesperada.", 409)
			}
			if _, e = a.s.Docker.Call(a.ctx, []string{"stop", "--time", "30", str(c["id"])}, RunOptions{}); e != nil {
				return e
			}
			if _, e = a.s.Docker.Call(a.ctx, []string{"rm", str(c["id"])}, RunOptions{}); e != nil {
				return e
			}
		}
		if _, e = a.s.Infra.Start(a.ctx, id, nil); e != nil {
			return e
		}
		var res Result
		if engine == "redis" {
			res, e = a.s.Infra.Client(a.ctx, db, r, "", []string{"GET", "np-acceptance-persistence"})
		} else {
			res, e = a.s.Infra.Client(a.ctx, db, r, "SELECT value FROM acceptance_persistence;", nil)
		}
		if e != nil {
			return e
		}
		if !strings.Contains(res.Stdout, "nearprod-persisted") {
			return fail("ACCEPTANCE_PERSISTENCE", "Se perdió el dato después de recrear.", 422)
		}
		return nil
	}); e != nil {
		return e
	}
	if engine != "redis" {
		if e := a.step(engine+": backup lógico y restauración limitada en base nueva", func() error {
			backup, e := a.s.Infra.Backup(a.ctx, J{"database": db["id"], "confirm": true})
			if e != nil {
				return e
			}
			if _, e = a.s.Infra.Restore(a.ctx, J{"database": other["id"], "file": backup["file"], "trustedBackup": true, "confirm": true}); e != nil {
				return e
			}
			res, e := a.s.Infra.Client(a.ctx, other, r, "SELECT value FROM acceptance_persistence;", nil)
			if e != nil {
				return e
			}
			if !strings.Contains(res.Stdout, "nearprod-persisted") {
				return fail("ACCEPTANCE_RESTORE", "No se recuperó el dato.", 422)
			}
			return nil
		}); e != nil {
			return e
		}
	}
	// A lightweight real consumer proves overlay/environment/network, without installing anything on the host.
	if e := a.step(engine+": vincular consumidor y comprobar red/variables/autenticación", func() error {
		path := filepath.Join(a.root, "projects", engine+"-consumer")
		if e := os.MkdirAll(path, 0700); e != nil {
			return e
		}
		model := J{"services": J{"worker": J{"image": r["resolvedImage"], "entrypoint": A{"sh", "-c", "while :; do sleep 5; done"}, "mem_limit": "64m"}}}
		if e := writeCompose(filepath.Join(path, "compose.yaml"), model); e != nil {
			return e
		}
		app, e := a.register(engine+"-consumer", path, nil, nil)
		if e != nil {
			return e
		}
		req := J{"target": app["id"], "database": db["id"], "services": A{"worker"}}
		p, e := a.s.Infra.BindingPreview(a.ctx, req)
		if e != nil {
			return e
		}
		req["confirm"] = true
		req["fingerprint"] = p["fingerprint"]
		binding, e := a.s.Infra.Bind(a.ctx, req)
		if e != nil {
			return e
		}
		if e = a.approve(str(app["id"]), "dev"); e != nil {
			return e
		}
		if e = a.action(str(app["id"]), "up", J{"wait": true}); e != nil {
			return e
		}
		if _, e = a.s.Infra.BindingCheck(a.ctx, str(at(binding, "binding", "id"))); e != nil {
			return e
		}
		return a.action(str(app["id"]), "stop", J{})
	}); e != nil {
		return e
	}
	p, e := a.s.Infra.StopPreview(a.ctx, id)
	if e != nil {
		return e
	}
	_, e = a.s.Infra.Stop(a.ctx, J{"instance": id, "fingerprint": p["fingerprint"], "confirm": true}, nil)
	return e
}
func (a *acceptance) cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	s := a.s
	info, e := s.Docker.Info(ctx)
	if e != nil {
		return e
	}
	if str(info["id"]) != a.engine || str(info["endpoint"]) != a.endpoint {
		return fail("CLEANUP_ENGINE", "El Engine cambió: no se borrará nada.", 409)
	}
	owner := str(s.Store.Get()["owner"])
	cs, e := s.Docker.Containers(ctx, false)
	if e != nil {
		return e
	}
	for _, v := range cs {
		c := obj(v)
		if str(at(c, "labels", LOwner)) == owner {
			if _, e = s.Docker.Call(ctx, []string{"rm", "-f", str(c["id"])}, RunOptions{}); e != nil {
				return e
			}
		}
	}
	// Only resources with this random acceptance owner are eligible for removal.
	for _, kind := range []string{"network", "volume"} {
		res, e := s.Docker.Call(ctx, []string{kind, "ls", "--filter", "label=" + LOwner + "=" + owner, "--format", "{{.Name}}"}, RunOptions{Exact: true, Limit: 1 << 20})
		if e != nil {
			return e
		}
		for _, name := range strings.Fields(res.Stdout) {
			v, e := s.Docker.InspectNamed(ctx, kind, name)
			if e != nil {
				return e
			}
			if str(at(v, "Labels", LOwner)) != owner {
				return fail("CLEANUP_OWNER", "El recurso no es propio: no se elimina.", 409)
			}
			if _, e = s.Docker.Call(ctx, []string{kind, "rm", name}, RunOptions{}); e != nil {
				return e
			}
		}
	}
	for _, v := range arr(s.Infra.State()["instances"]) {
		r := obj(v)
		if str(at(r, "persistence", "kind")) != "folder" {
			continue
		}
		p := str(at(r, "persistence", "path"))
		if !within(a.root, p) {
			return fail("CLEANUP_PATH", "Carpeta fuera de aceptación: no se elimina.", 409)
		}
		marker, e := readJSON(filepath.Join(p, ".nearprod-resource.json"), 4096)
		if e != nil || str(marker["owner"]) != owner || str(marker["uid"]) != str(r["uid"]) {
			return fail("CLEANUP_OWNER", "Marcador de datos no coincide.", 409)
		}
		// Own isolated fixture data only; required on Mac where engine files use another UID.
		_, e = s.Infra.Helper(ctx, r, []string{"sh", "-ec", `find /data -mindepth 1 -maxdepth 1 -exec rm -rf -- {} +`}, HelperOptions{RunOptions: RunOptions{Timeout: time.Minute}, Network: "none", Extra: []string{"--user", "0:0", "--mount", "type=bind,source=" + filepath.Join(p, "data") + ",target=/data"}})
		if e != nil {
			return e
		}
	}
	return nil
}
