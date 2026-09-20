package nearprod

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const Help = `NearProd 0.8 — controlador Go + panel React/TypeScript

Instalación y configuración (no requieren Docker):
  nearprod install [--configure-shell]
  nearprod update --check
  nearprod update                       solicita confirmación; --yes para automatización
  nearprod config paths
  nearprod config migrate --dry-run
  nearprod config migrate --yes
  nearprod config backup --yes          incluye credenciales, NO datos SQL
  nearprod startup enable|disable|status
  nearprod ui [--no-open]
  nearprod agent stop

Aplicaciones:
  nearprod init ~/Projects
  nearprod discover ~/Projects
  nearprod list
  nearprod status [grupo/aplicación]
  nearprod register --manifest definicion.json
  nearprod edit grupo/app --manifest definicion.json
  nearprod group-create --name "Mi grupo" [--id mi-grupo]
  nearprod review grupo/app [--mode dev|verify] [--approve --yes --allow-unsafe]
  nearprod adopt grupo/app [--yes]
  nearprod up|stop|restart|rebuild grupo[/app] [--mode dev|verify] [--wait]
               [--service api] [--confirm-mode] [--start-runtime] [--detach]
  nearprod logs grupo/app [--follow] [--service api] [--tail 100] [--since RFC3339]
  nearprod watch|watch-stop grupo/app
  nearprod remove grupo/app --yes        solo catálogo, conserva datos
  nearprod operation ID | nearprod cancel ID

Accesos locales / runtime:
  nearprod proxy status
  nearprod proxy start --port 80 [--yes]
  nearprod proxy stop --yes
  nearprod url grupo/app --host proyecto.localhost --service web --port 5173
  nearprod url-check grupo/app --host proyecto.localhost
  nearprod doctor
  nearprod runtime select --kind colima --context colima --profile default
  nearprod runtime status|start [--yes]
  nearprod runtime configure --memory 2 --cpus 2 [--yes]
  nearprod metrics

Infraestructura compartida:
  nearprod infra list
  nearprod infra ports --from 15432 --to 15442
  nearprod infra create --engine postgres|mysql|redis --id pg-main
    [--image postgres:18] [--memory 384] [--connections 30]
    [--persistence volume|folder|none] [--data-dir /ruta/dedicada] [--port 15432] [--yes]
  nearprod infra start --instance pg-main
  nearprod infra database --instance pg-main --name app_dev [--username app_user] --yes
  nearprod infra connection --database ID [--reveal]
  nearprod infra bind --target grupo/app --database ID --services api,worker
    [--mode dev] [--url-var DATABASE_URL] [--yes]
  nearprod infra check --database ID
  nearprod infra check-binding --binding ID
  nearprod infra unbind --binding ID --yes
  nearprod infra stop --instance pg-main [--yes --allow-active]
  nearprod infra backup --database ID [--directory /ruta/backups] --yes
  nearprod infra restore --database ID --file /ruta/archivo --trusted-backup --yes
  nearprod infra logs --instance pg-main [--follow]

Herramientas:
  nearprod tools list
  nearprod tools versions --tool docker|compose|buildx|colima
  nearprod tools install|upgrade|repair --tool compose [--formula docker-compose] [--yes]
  nearprod updates check

Aceptación real (crea/limpia SOLO recursos temporales de prueba):
  nearprod self-test --yes --context colima [--infra-only] [--folder]

--json emite JSON. NEARPROD_HOME selecciona un catálogo separado.
Actualizar el binario conserva ~/.nearprod/config y rutas/volúmenes existentes.
`

type cliArgs struct {
	Pos  []string
	Opts map[string][]string
}

var boolFlags = map[string]bool{"help": true, "version": true, "identity": true, "json": true, "yes": true, "check": true, "configure-shell": true, "foreground": true, "launch-agent": true, "no-open": true, "approve": true, "allow-unsafe": true, "wait": true, "detach": true, "confirm-mode": true, "start-runtime": true, "follow": true, "reveal": true, "allow-active": true, "trusted-backup": true, "dry-run": true, "infra-only": true, "folder": true}
var valueFlags = map[string]bool{"home": true, "port": true, "target": true, "mode": true, "service": true, "tail": true, "since": true, "container": true, "manifest": true, "name": true, "id": true, "product": true, "stack": true, "slug": true, "path": true, "project-name": true, "file": true, "env-file": true, "profile": true, "verify-file": true, "verify-env-file": true, "verify-profile": true, "kind": true, "context": true, "memory": true, "cpus": true, "cpu": true, "formula": true, "tool": true, "engine": true, "instance": true, "database": true, "binding": true, "image": true, "connections": true, "persistence": true, "data-dir": true, "services": true, "url-var": true, "host-var": true, "port-var": true, "database-var": true, "user-var": true, "password-var": true, "directory": true, "output": true, "username": true, "host": true, "verify-service": true, "verify-port": true, "from": true, "to": true, "depth": true, "max-entries": true, "ignore": true}

func parseCLI(args []string) (cliArgs, error) {
	a := cliArgs{Opts: map[string][]string{}}
	for n := 0; n < len(args); n++ {
		x := args[n]
		if x == "-h" {
			x = "--help"
		}
		if x == "-v" {
			x = "--version"
		}
		if !strings.HasPrefix(x, "--") {
			if strings.HasPrefix(x, "-") {
				return a, fail("OPTION_INVALID", "Opción desconocida: "+x, 400)
			}
			a.Pos = append(a.Pos, x)
			continue
		}
		k, v, equal := strings.Cut(strings.TrimPrefix(x, "--"), "=")
		if boolFlags[k] {
			if !equal {
				v = "true"
			}
			if v != "true" && v != "false" {
				return a, fail("OPTION_INVALID", "Usa true/false para --"+k, 400)
			}
		} else if valueFlags[k] {
			if !equal {
				n++
				if n >= len(args) || strings.HasPrefix(args[n], "--") {
					return a, fail("OPTION_VALUE", "Falta valor de --"+k, 400)
				}
				v = args[n]
			}
		} else {
			return a, fail("OPTION_INVALID", "Opción desconocida --"+k, 400)
		}
		a.Opts[k] = append(a.Opts[k], v)
	}
	return a, nil
}
func (a cliArgs) S(k string) string {
	xs := a.Opts[k]
	if len(xs) == 0 {
		return ""
	}
	return xs[len(xs)-1]
}
func (a cliArgs) B(k string) bool { return a.S(k) == "true" }
func (a cliArgs) P(n int) string {
	if len(a.Pos) > n {
		return a.Pos[n]
	}
	return ""
}
func (a cliArgs) N(k string) any {
	if a.S(k) == "" {
		return nil
	}
	n, e := strconv.ParseFloat(a.S(k), 64)
	if e != nil {
		return a.S(k)
	}
	return n
}
func outputJSON(out io.Writer, v any) {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// RunCLI returns an exit status, without os.Exit, to support real CLI contract tests.
func RunCLI(ctx context.Context, args []string, assets fs.FS, out, errout io.Writer) int {
	a, e := parseCLI(args)
	if e != nil {
		fmt.Fprintln(errout, e)
		return 2
	}
	if a.S("home") != "" {
		_ = os.Setenv("NEARPROD_HOME", expandHome(a.S("home")))
	}
	if a.B("identity") {
		outputJSON(out, J{"service": "nearprod", "version": Version, "marker": BinaryMarker, "language": "Go", "toolchain": runtime.Version(), "platform": runtime.GOOS + "/" + runtime.GOARCH})
		return 0
	}
	if a.B("version") || a.P(0) == "version" {
		fmt.Fprintln(out, Version)
		return 0
	}
	if a.B("help") || a.P(0) == "" || a.P(0) == "help" {
		fmt.Fprint(out, Help)
		return 0
	}
	home := homeDir()
	cmd, sub := a.P(0), a.P(1)
	var result any
	execute := func() (any, error) {
		switch cmd {
		case "serve":
			port := 0
			if a.S("port") != "" {
				p, e := intRange(a.N("port"), 0, 65535, "Puerto del agente")
				if e != nil {
					return nil, e
				}
				port = p
			}
			agent, e := StartAgent(ctx, home, assets, nil, true, port)
			if e != nil {
				if str(publicError(e)["code"]) == "AGENT_RUNNING" && a.B("launch-agent") {
					return J{"alreadyRunning": true}, nil
				}
				return nil, e
			}
			fmt.Fprintln(out, "NearProd", Version, "Go —", str(agent.Info["url"]))
			select {
			case <-ctx.Done():
				agent.Close()
			case <-agent.Done():
			}
			return nil, nil
		case "install":
			return InstallBinary(userHome(), binaryPath(), a.B("configure-shell"))
		case "update":
			if sub != "" {
				return nil, fail("USAGE", "Usa nearprod update [--check] [--yes].", 400)
			}
			updater := newUpdater()
			plan, e := updater.Check(ctx)
			if e != nil {
				return nil, e
			}
			if a.B("check") || !plan.Available {
				return plan.Summary, nil
			}
			if plan.Provider != "manual" {
				return updater.Apply(ctx, plan)
			}
			if !a.B("yes") {
				confirmed, e := confirmUpdate(os.Stdin, errout, plan.Current, plan.Latest)
				if e != nil {
					return nil, e
				}
				if !confirmed {
					return merge(plan.Summary, J{"status": "cancelled", "cancelled": true}), nil
				}
			}
			return updater.Apply(ctx, plan)
		case "startup":
			manager := NewStartup(home)
			switch sub {
			case "enable":
				return manager.Enable(ctx)
			case "disable":
				return manager.Disable(ctx)
			case "status":
				return manager.Status(ctx)
			}
			return nil, fail("USAGE", "Usa startup enable|disable|status.", 400)
		case "config":
			switch sub {
			case "paths":
				return configPaths(home), nil
			case "migrate":
				if !a.B("yes") || a.B("dry-run") {
					return MigrationPreview(home)
				}
				release, e := AcquireAgentLock(home)
				if e != nil {
					return nil, e
				}
				defer release()
				st, e := OpenStore(home)
				if e != nil {
					return nil, e
				}
				return J{"migrated": true, "paths": configPaths(home), "stacks": len(arr(st.Get()["stacks"])), "note": "No se inició Docker ni se movieron datos físicos."}, nil
			case "backup":
				if !a.B("yes") {
					return J{"preview": true, "containsSecrets": true, "containsDatabaseData": false, "instruction": "nearprod config backup --yes"}, nil
				}
			default:
				return nil, fail("USAGE", "Usa config paths|migrate|backup.", 400)
			}
		case "self-test":
			return SelfTest(ctx, a, out)
		}
		start := true
		allowMismatch := false
		if cmd == "agent" && sub == "stop" {
			start = false
			allowMismatch = true
		}
		info, e := EnsureAgent(ctx, home, start, allowMismatch)
		if e != nil {
			return nil, e
		}
		api := func(route string, body any) (J, error) { return requestAPI(ctx, info, route, body, 60*time.Second) }
		operation := func(route string, body J) (any, error) {
			op, e := api(route, body)
			if e != nil {
				return nil, e
			}
			if a.B("detach") {
				return op, nil
			}
			emit := func(v string) {
				if !a.B("json") {
					fmt.Fprintln(errout, v)
				}
			}
			fmt.Fprintln(errout, "Operación:", str(op["id"]))
			op, e = waitOperation(ctx, info, op, emit)
			if e != nil {
				return nil, e
			}
			if str(op["state"]) != "succeeded" {
				return op, detailed("OPERATION_FAILED", "La operación falló; no se deshicieron los efectos de Docker.", 422, op)
			}
			return op, nil
		}
		previewApply := func(preRoute, actionRoute string, body J) (any, error) {
			p, e := api(preRoute, body)
			if e != nil {
				return nil, e
			}
			if !a.B("yes") {
				return p, nil
			}
			body["fingerprint"] = p["fingerprint"]
			body["confirm"] = true
			return operation(actionRoute, body)
		}
		switch cmd {
		case "ui":
			pair, e := api("/pair", J{})
			if e != nil {
				return nil, e
			}
			if a.B("json") {
				return merge(pair, J{"url": info["url"], "version": Version}), nil
			}
			fmt.Fprintf(out, "NearProd %s\nPanel: %s\nCódigo temporal (120 s): %s\n", Version, str(info["url"]), str(pair["code"]))
			if !a.B("no-open") {
				openBrowser(str(info["url"]))
			}
			return nil, nil
		case "agent":
			if sub != "stop" {
				return nil, fail("USAGE", "Usa agent stop.", 400)
			}
			result, err := api("/agent/stop", J{})
			if err != nil {
				return nil, err
			}
			if err = waitAgentStopped(ctx, home); err != nil {
				return nil, err
			}
			return result, nil
		case "config":
			return api("/config/backup", J{"confirm": true})
		case "doctor":
			return api("/doctor", nil)
		case "metrics":
			return api("/metrics", nil)
		case "init":
			return api("/roots", J{"root": sub})
		case "discover":
			p := J{"root": sub}
			if a.N("depth") != nil {
				p["depth"] = a.N("depth")
			}
			if a.N("max-entries") != nil {
				p["maxEntries"] = a.N("max-entries")
			}
			p["ignore"] = a.Opts["ignore"]
			return api("/discover", p)
		case "list":
			return api("/catalog", nil)
		case "groups":
			v, e := api("/catalog", nil)
			if e != nil {
				return nil, e
			}
			return J{"groups": v["groups"]}, nil
		case "group-create":
			return api("/groups", J{"name": a.S("name"), "id": a.S("id")})
		case "status":
			v, e := api("/status?refresh=1", nil)
			if e != nil {
				return nil, e
			}
			if sub != "" {
				selected := A{}
				for _, r := range arr(v["stacks"]) {
					id := str(obj(r)["id"])
					if id == sub || strings.HasPrefix(id, sub+"/") {
						selected = append(selected, r)
					}
				}
				v["stacks"] = selected
			}
			return v, nil
		case "register", "edit":
			var d J
			if file := a.S("manifest"); file != "" {
				raw, e := readLimited(expandHome(file), 1<<20)
				if e != nil {
					return nil, e
				}
				d, e = decodeObject(raw)
				if e != nil {
					return nil, e
				}
			} else {
				d = J{"product": a.S("product"), "slug": text(a.S("stack"), a.S("slug")), "name": a.S("name"), "path": expandHome(a.S("path")), "projectName": a.S("project-name"), "modes": J{"dev": J{"files": a.Opts["file"], "envFiles": a.Opts["env-file"], "profiles": a.Opts["profile"]}}}
				if len(a.Opts["verify-file"]) > 0 {
					obj(d["modes"])["verify"] = J{"files": a.Opts["verify-file"], "envFiles": a.Opts["verify-env-file"], "profiles": a.Opts["verify-profile"]}
				}
			}
			if cmd == "edit" {
				return api("/stacks/edit", J{"target": sub, "definition": d})
			}
			return api("/stacks", d)
		case "review":
			body := J{"target": sub, "mode": text(a.S("mode"), "dev")}
			p, e := api("/preview", body)
			if e != nil {
				return nil, e
			}
			if !a.B("approve") {
				return p, nil
			}
			if !a.B("yes") {
				return nil, fail("CONFIRM_REQUIRED", "Revisa primero el Compose. Aprobar requiere --approve --yes; riesgos requieren --allow-unsafe.", 409)
			}
			body["fingerprint"] = p["fingerprint"]
			body["allowUnsafe"] = a.B("allow-unsafe")
			return api("/trust", body)
		case "adopt":
			body := J{"target": sub}
			p, e := api("/adoption", body)
			if e != nil {
				return nil, e
			}
			if !a.B("yes") {
				return p, nil
			}
			return api("/adopt", merge(body, J{"confirm": true, "fingerprint": p["fingerprint"]}))
		case "up", "stop", "restart", "rebuild":
			return operation("/actions", J{"target": sub, "action": cmd, "mode": a.S("mode"), "service": a.S("service"), "wait": a.B("wait"), "confirmMode": a.B("confirm-mode"), "startRuntime": a.B("start-runtime")})
		case "logs":
			query := url.Values{"target": {sub}, "follow": {strconv.FormatBool(a.B("follow"))}, "tail": {text(a.S("tail"), "100")}, "service": {a.S("service")}, "since": {a.S("since")}, "container": {a.S("container")}}
			return nil, streamAPI(ctx, info, "/logs", query, func(event string, v J) {
				if a.B("json") {
					outputJSON(out, J{"event": event, "data": v})
				} else if event == "line" {
					fmt.Fprintf(out, "[%s] %s\n", str(v["service"]), str(v["text"]))
				} else if event == "log-error" {
					fmt.Fprintln(errout, str(v["message"]))
				}
			})
		case "watch":
			return operation("/watch", J{"target": sub, "action": "start"})
		case "watch-stop":
			return api("/watch", J{"target": sub, "action": "stop"})
		case "remove":
			return api("/stacks/remove", J{"target": sub, "confirm": a.B("yes")})
		case "operation":
			return api("/operations/"+url.PathEscape(sub), nil)
		case "cancel":
			return api("/cancel", J{"id": sub})
		case "runtime":
			switch sub {
			case "select":
				return api("/runtime/settings", J{"kind": a.S("kind"), "context": a.S("context"), "profile": text(a.S("profile"), "default")})
			case "status":
				return api("/doctor", nil)
			case "start":
				return operation("/runtime/actions", J{"action": "start", "confirm": a.B("yes")})
			case "configure":
				return previewApply("/runtime/preview", "/runtime/actions", J{"action": "configure", "memory": a.N("memory"), "cpus": a.N("cpus")})
			}
			return nil, fail("USAGE", "Usa runtime status|select|start|configure.", 400)
		case "proxy":
			switch sub {
			case "status":
				return api("/proxy", nil)
			case "start":
				body := J{"action": "start"}
				if a.N("port") != nil {
					body["port"] = a.N("port")
				}
				return previewApply("/proxy/preview", "/proxy/actions", body)
			case "stop":
				return operation("/proxy/actions", J{"action": "stop", "confirm": a.B("yes")})
			}
			return nil, fail("USAGE", "Usa proxy status|start|stop.", 400)
		case "url-check":
			return api("/proxy/check", J{"target": sub, "host": a.S("host")})
		case "url":
			cat, e := api("/catalog", nil)
			if e != nil {
				return nil, e
			}
			var st J
			for _, v := range arr(cat["stacks"]) {
				if str(obj(v)["id"]) == sub {
					st = obj(v)
				}
			}
			if st == nil {
				return nil, fail("STACK_NOT_FOUND", "Aplicación no encontrada.", 404)
			}
			route := J{"host": a.S("host"), "service": a.S("service"), "port": a.N("port")}
			if a.N("verify-port") != nil {
				route["verify"] = J{"service": text(a.S("verify-service"), a.S("service")), "port": a.N("verify-port")}
			}
			routes := A{}
			for _, v := range arr(st["routes"]) {
				if str(obj(v)["host"]) != a.S("host") {
					routes = append(routes, v)
				}
			}
			st["routes"] = append(routes, route)
			return api("/stacks/edit", J{"target": sub, "definition": st})
		case "tools":
			if sub == "list" {
				return api("/tools", nil)
			}
			if sub == "versions" {
				return api("/tools/versions", J{"tool": a.S("tool")})
			}
			return previewApply("/tools/preview", "/tools/actions", J{"tool": a.S("tool"), "formula": a.S("formula"), "action": sub})
		case "updates":
			return api("/updates", J{})
		case "infra":
			return cliInfra(ctx, a, info, api, operation, previewApply, out, errout)
		}
		return nil, fail("COMMAND_UNKNOWN", "Comando desconocido. Usa nearprod --help.", 400)
	}
	result, e = execute()
	if e != nil {
		if result != nil {
			outputJSON(out, result)
		}
		if a.B("json") {
			outputJSON(errout, J{"error": publicError(e)})
		} else {
			fmt.Fprintln(errout, e)
		}
		if cmd == "self-test" && str(publicError(e)["code"]) == "TOOL_MISSING" {
			return 77
		}
		if statusCode(e) == 400 {
			return 2
		}
		return 1
	}
	if result != nil {
		outputJSON(out, result)
	}
	return 0
}
func openBrowser(address string) {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "/usr/bin/open"
	}
	p := findExecutable(name)
	if p == "" {
		return
	}
	cmd := exec.Command(p, address)
	if cmd.Start() == nil {
		go func() { _ = cmd.Wait() }()
	}
}
func cliInfra(ctx context.Context, a cliArgs, info J, api func(string, any) (J, error), operation func(string, J) (any, error), previewApply func(string, string, J) (any, error), out, errout io.Writer) (any, error) {
	sub := a.P(1)
	req := J{"action": sub, "instance": a.S("instance"), "database": a.S("database"), "binding": a.S("binding"), "confirm": a.B("yes")}
	switch sub {
	case "list":
		return api("/infra", nil)
	case "ports":
		return api("/infra/ports", J{"start": a.N("from"), "end": a.N("to")})
	case "create":
		for _, key := range []string{"engine", "id", "name", "image"} {
			if a.S(key) != "" {
				req[key] = a.S(key)
			}
		}
		for flag, key := range map[string]string{"memory": "memoryMiB", "connections": "maxConnections", "port": "hostPort"} {
			if a.N(flag) != nil {
				req[key] = a.N(flag)
			}
		}
		req["persistence"] = J{"kind": text(a.S("persistence"), "volume")}
		if a.S("data-dir") != "" {
			obj(req["persistence"])["path"] = expandHome(a.S("data-dir"))
		}
		return previewApply("/infra/preview", "/infra/actions", req)
	case "stop":
		req["allowActive"] = a.B("allow-active")
		return previewApply("/infra/stop-preview", "/infra/actions", req)
	case "start", "check", "check-binding", "unbind":
		return operation("/infra/actions", req)
	case "database":
		req["name"] = a.S("name")
		if a.S("username") != "" {
			req["username"] = a.S("username")
		}
		return operation("/infra/actions", req)
	case "connection":
		return api("/infra/connection", J{"database": a.S("database"), "reveal": a.B("reveal")})
	case "bind":
		req["target"] = a.S("target")
		req["mode"] = text(a.S("mode"), "dev")
		req["services"] = strings.Split(a.S("services"), ",")
		mapping := J{}
		for _, field := range []string{"url", "host", "port", "database", "user", "password"} {
			if a.S(field+"-var") != "" {
				mapping[field] = a.S(field + "-var")
			}
		}
		req["mapping"] = mapping
		return previewApply("/infra/binding-preview", "/infra/actions", req)
	case "backup":
		if a.S("directory") != "" {
			req["directory"] = expandHome(a.S("directory"))
		}
		return operation("/infra/actions", req)
	case "restore":
		req["file"] = expandHome(a.S("file"))
		req["trustedBackup"] = a.B("trusted-backup")
		return operation("/infra/actions", req)
	case "logs":
		query := url.Values{"instance": {a.S("instance")}, "follow": {strconv.FormatBool(a.B("follow"))}, "tail": {text(a.S("tail"), "200")}}
		return nil, streamAPI(ctx, info, "/infra/logs", query, func(event string, v J) {
			if a.B("json") {
				outputJSON(out, J{"event": event, "data": v})
			} else if event == "line" {
				fmt.Fprintln(out, str(v["text"]))
			} else if event == "log-error" {
				fmt.Fprintln(errout, str(v["message"]))
			}
		})
	}
	return nil, fail("USAGE", "Subcomando infra desconocido; consulta nearprod --help.", 400)
}

var _ = filepath.Separator
