package nearprod

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Compose struct {
	Runner Runner
	Store  *Store
	Docker *Docker
	Infra  *Infrastructure
}
type Resolution struct {
	Model, Preview, Routing, Infrastructure J
	Redact                                  *Redactor
}

func composeArgs(stack J, mode, context, command string, args []string, overlay string) []string {
	def := obj(at(stack, "modes", mode))
	flags := []string{"--context", context, "compose", "--project-directory", str(stack["path"]), "--project-name", str(stack["projectName"])}
	for _, p := range ss(def["files"]) {
		flags = append(flags, "-f", resolvePath(str(stack["path"]), p))
	}
	if overlay != "" {
		flags = append(flags, "-f", overlay)
	}
	for _, p := range ss(def["envFiles"]) {
		flags = append(flags, "--env-file", resolvePath(str(stack["path"]), p))
	}
	for _, p := range ss(def["profiles"]) {
		flags = append(flags, "--profile", p)
	}
	return append(append(flags, command), args...)
}
func resolvePath(root, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(root, p)
}
func activeServices(model J, profiles []string, selected string) ([]string, error) {
	svcs := obj(model["services"])
	if selected != "" && svcs[selected] == nil {
		return nil, fail("SERVICE_NOT_FOUND", "Servicio desconocido: "+selected, 404)
	}
	out := []string{}
	if selected != "" {
		out = append(out, selected)
	} else {
		for _, name := range keys(svcs) {
			p := ss(at(svcs, name, "profiles"))
			enabled := len(p) == 0 || contains(profiles, "*")
			for _, v := range p {
				enabled = enabled || contains(profiles, v)
			}
			if enabled {
				out = append(out, name)
			}
		}
	}
	for i := 0; i < len(out); i++ {
		deps := at(svcs, out[i], "depends_on")
		names := ss(deps)
		if len(obj(deps)) > 0 {
			names = keys(obj(deps))
		}
		for _, dep := range names {
			if svcs[dep] != nil && !contains(out, dep) {
				out = append(out, dep)
			}
		}
	}
	return out, nil
}

var devCommand = regexp.MustCompile(`--reload\b|\bvite\b|\bwebpack-dev-server\b|\bnodemon\b|\bartisan\s+serve\b|\bnpm\s+(?:run\s+)?(?:dev|start:dev)\b|\bpnpm\s+(?:run\s+)?dev\b|\bphp\s+-S\b|\bmix\s+phx\.server\b|\bng\s+serve\b|\bnext\s+dev\b`)

func stringCommand(v any) string {
	if _, ok := v.(string); ok {
		return str(v)
	}
	return strings.Join(ss(v), " ")
}
func analyzeModel(model J, mode string, roots []string) (J, error) {
	svcs := obj(model["services"])
	if len(svcs) == 0 {
		return nil, fail("COMPOSE_EMPTY", "Compose no tiene servicios; revisa los perfiles.", 422)
	}
	risks, warnings, blockers, services := A{}, A{}, A{}, A{}
	for _, name := range keys(svcs) {
		s := obj(svcs[name])
		cmd := stringCommand(s["entrypoint"]) + " " + stringCommand(s["command"])
		dev := devCommand.MatchString(cmd)
		mounts, ports := A{}, A{}
		binds := 0
		for _, raw := range arr(s["volumes"]) {
			v := obj(raw)
			if str(v["type"]) == "bind" {
				binds++
				allowed := false
				for _, r := range roots {
					allowed = allowed || within(r, str(v["source"]))
				}
				if !allowed {
					risks = append(risks, name+": montaje fuera de las raíces autorizadas.")
				}
				src := str(v["source"])
				if strings.Contains(src, "docker.sock") || strings.Contains(src, ".ssh") || strings.Contains(src, "/var/run") {
					risks = append(risks, name+": montaje sensible o socket de control.")
				}
			}
			mounts = append(mounts, J{"type": v["type"], "source": v["source"], "target": v["target"], "readOnly": truth(v["read_only"])})
		}
		if truth(s["privileged"]) {
			risks = append(risks, name+": privileged=true.")
		}
		for _, k := range []string{"network_mode", "pid", "ipc"} {
			if str(s[k]) == "host" {
				risks = append(risks, name+": comparte namespace del host.")
			}
		}
		if len(arr(s["cap_add"])) > 0 || len(arr(s["devices"])) > 0 {
			risks = append(risks, name+": capacidades/dispositivos adicionales.")
		}
		if mode == "verify" && binds > 0 {
			blockers = append(blockers, name+": Prueba de imagen no admite bind mounts.")
		}
		if mode == "verify" && (dev || s["develop"] != nil) {
			blockers = append(blockers, name+": contiene recarga, servidor de desarrollo o develop/watch.")
		}
		health := len(obj(s["healthcheck"])) > 0 && !truth(at(s, "healthcheck", "disable"))
		if !health {
			warnings = append(warnings, name+": sin healthcheck; running no significa listo.")
		}
		if img := str(s["image"]); img != "" && !strings.Contains(img, "@sha256:") {
			warnings = append(warnings, name+": imagen con etiqueta mutable; no demuestra igualdad con producción.")
		}
		for _, raw := range arr(s["ports"]) {
			p := obj(raw)
			host := text(p["host_ip"], "0.0.0.0")
			if host != "127.0.0.1" && host != "::1" {
				risks = append(risks, name+": publica puertos en interfaces no limitadas a loopback.")
			}
			ports = append(ports, J{"target": p["target"], "published": p["published"], "host": host, "protocol": text(p["protocol"], "tcp")})
		}
		if s["post_start"] != nil || s["pre_start"] != nil || s["pre_stop"] != nil {
			risks = append(risks, name+": hooks de ciclo de vida; revisa sus efectos.")
		}
		services = append(services, J{"name": name, "image": s["image"], "build": s["build"] != nil, "platform": s["platform"], "hasHealthcheck": health, "watch": len(arr(at(s, "develop", "watch"))) > 0, "devCommand": dev, "ports": ports, "mounts": mounts})
	}
	return J{"services": services, "risks": risks, "warnings": warnings, "blockers": blockers}, nil
}

var transitiveRE = regexp.MustCompile(`(?m)(?:^|[\s{,])(?:["']?)(?:include|extends)["']?\s*:`)
var escapedKeyRE = regexp.MustCompile(`(?m)["'][^\r\n"']*\\[ux][^\r\n"']*["']\s*:`)

func validateSource(data []byte) error {
	if len(data) > 2<<20 {
		return fail("CONFIG_LIMIT", "Compose demasiado grande.", 413)
	}
	if transitiveRE.Match(data) || escapedKeyRE.Match(data) {
		return fail("COMPOSE_TRANSITIVE", "Usa archivos -f explícitos: include/extends o claves escapadas no se admiten para una revisión completa.", 422)
	}
	return nil
}
func (c *Compose) Supports(ctx context.Context, command, flag string) bool {
	r, e := c.Runner.Run(ctx, "docker", []string{"compose", command, "--help"}, RunOptions{})
	return e == nil && r.Code == 0 && strings.Contains(r.Stdout, flag)
}
func (c *Compose) Resolve(ctx context.Context, stack J, mode string) (*Resolution, error) {
	state := c.Store.Get()
	definition := obj(at(stack, "modes", mode))
	if len(definition) == 0 {
		return nil, fail("MODE_MISSING", "No existe el modo "+mode+".", 400)
	}
	roots := ss(state["roots"])
	checkout, e := canonical(str(stack["path"]), roots, "directory")
	if e != nil {
		return nil, e
	}
	redact := NewRedactor()
	files := A{}
	for _, p := range ss(definition["files"]) {
		full, e := canonical(resolvePath(checkout, p), roots, "file")
		if e != nil {
			return nil, e
		}
		raw, e := readLimited(full, 2<<20)
		if e != nil {
			return nil, e
		}
		if e = validateSource(raw); e != nil {
			return nil, e
		}
		files = append(files, A{full, string(raw)})
	}
	for _, p := range ss(definition["envFiles"]) {
		full, e := canonical(resolvePath(checkout, p), roots, "file")
		if e != nil {
			return nil, e
		}
		raw, e := readLimited(full, 2<<20)
		if e != nil {
			return nil, e
		}
		redact.Dotenv(string(raw))
		files = append(files, A{full, string(raw)})
	}
	// Default .env can influence interpolation; include even if no explicit files selected.
	if raw, e := readLimited(filepath.Join(checkout, ".env"), 2<<20); e == nil {
		redact.Dotenv(string(raw))
		files = append(files, A{".env", string(raw)})
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	if _, e = c.Docker.Endpoint(ctx); e != nil {
		return nil, e
	}
	ver, e := checked(ctx, c.Runner, "docker", []string{"compose", "version", "--short"}, RunOptions{})
	if e != nil {
		return nil, e
	}
	m := regexp.MustCompile(`\d+`).FindString(ver.Stdout)
	if integer(m) < 2 {
		return nil, fail("COMPOSE_VERSION", "Se requiere docker compose V2 o posterior.", 422)
	}
	response, e := checked(ctx, c.Runner, "docker", composeArgs(stack, mode, str(at(state, "runtime", "context")), "config", []string{"--format", "json"}, ""), RunOptions{Dir: checkout, Exact: true, Limit: 4 << 20, Redact: redact})
	if e != nil {
		return nil, e
	}
	model, e := decodeObject([]byte(response.Stdout))
	if e != nil {
		return nil, fail("COMPOSE_RESPONSE", "Compose no devolvió un modelo JSON válido.", 503)
	}
	for _, raw := range obj(model["services"]) {
		s := obj(raw)
		redact.Env(obj(s["environment"]))
		build := obj(s["build"])
		if bctx := str(build["context"]); bctx != "" {
			if strings.Contains(bctx, "://") || strings.HasPrefix(bctx, "git@") {
				continue
			}
			if build["dockerfile_inline"] == nil {
				df := resolvePath(bctx, text(build["dockerfile"], "Dockerfile"))
				if raw, e := readLimited(df, 2<<20); e == nil {
					files = append(files, A{df, string(raw)})
				}
			}
		}
	}
	analysis, e := analyzeModel(model, mode, roots)
	if e != nil {
		return nil, e
	}
	active, e := activeServices(model, ss(definition["profiles"]), "")
	if e != nil {
		return nil, e
	}
	analysis["activeServices"] = cloneStrings(active)
	routing, e := routingPlan(stack, mode, model, str(state["owner"]), active)
	if e != nil {
		return nil, e
	}
	infra := J{"services": J{}, "networks": J{}, "summary": A{}, "risks": A{}, "blockers": A{}, "fingerprint": nil}
	if c.Infra != nil {
		infra, e = c.Infra.Plan(ctx, stack, mode, model, redact)
		if e != nil {
			return nil, e
		}
	}
	for _, k := range []string{"risks", "warnings", "blockers"} {
		analysis[k] = append(append(list(analysis[k]), arr(routing[k])...), arr(infra[k])...)
	}
	fp := hash(J{"reviewVersion": "go-v7-1", "salt": state["owner"], "path": stack["path"], "project": stack["projectName"], "mode": mode, "definition": definition, "files": files, "model": model, "routes": list(stack["routes"]), "infra": infra["fingerprint"]})
	preview := merge(analysis, J{"stack": stack["id"], "mode": mode, "fingerprint": fp, "approved": str(at(stack, "trust", mode, "fingerprint")) == fp, "files": definition["files"], "envFiles": definition["envFiles"], "profiles": definition["profiles"], "routes": routing["targets"], "infrastructure": infra["summary"], "modeChange": str(stack["activeMode"]) != "" && str(stack["activeMode"]) != mode, "command": composeArgs(stack, mode, str(at(state, "runtime", "context")), "up", []string{"--detach"}, "<NearProd override>"), "note": "Revisar no ejecuta el proyecto. Aprobar autoriza su configuración Docker; no constituye un sandbox. Cambiar de modo comparte datos salvo configuración explícita."})
	return &Resolution{Model: model, Preview: preview, Routing: routing, Infrastructure: infra, Redact: redact}, nil
}
func readLimited(file string, limit int64) ([]byte, error) {
	f, e := os.Open(file)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !st.Mode().IsRegular() || st.Size() > limit {
		return nil, fail("FILE_LIMIT", "Archivo especial o demasiado grande: "+filepath.Base(file), 413)
	}
	data, e := io.ReadAll(io.LimitReader(f, limit+1))
	if e != nil {
		return nil, e
	}
	if int64(len(data)) > limit {
		return nil, fail("FILE_LIMIT", "Archivo excede el límite durante la lectura.", 413)
	}
	return data, nil
}
func (c *Compose) Approved(ctx context.Context, stack J, mode string) (*Resolution, error) {
	r, e := c.Resolve(ctx, stack, mode)
	if e != nil {
		return nil, e
	}
	if len(arr(r.Preview["blockers"])) > 0 {
		return nil, detailed("CONFIG_BLOCKED", "Hay bloqueos en la configuración.", 422, J{"blockers": r.Preview["blockers"]})
	}
	if !truth(r.Preview["approved"]) {
		return nil, fail("REVIEW_REQUIRED", "Revisa y aprueba esta configuración. Después de migrar a Go se requiere una revisión, no registrar de nuevo.", 409)
	}
	if len(arr(r.Preview["risks"])) > 0 && !truth(at(stack, "trust", mode, "allowUnsafe")) {
		return nil, fail("RISK_APPROVAL_REQUIRED", "Confirma las capacidades sensibles de la configuración.", 409)
	}
	return r, nil
}
func escapedCompose(v any) any {
	switch x := v.(type) {
	case string:
		return strings.ReplaceAll(x, "$", "$$")
	case J:
		r := J{}
		for k, v := range x {
			r[k] = escapedCompose(v)
		}
		return r
	case map[string]any:
		return escapedCompose(J(x))
	case []any:
		r := A{}
		for _, v := range x {
			r = append(r, escapedCompose(v))
		}
		return r
	case []string:
		r := A{}
		for _, v := range x {
			r = append(r, escapedCompose(v))
		}
		return r
	}
	return v
}
func writeCompose(file string, v J) error { return writeJSON(file, escapedCompose(v)) }
func (c *Compose) Overlay(stack J, r *Resolution) (string, error) {
	state := c.Store.Get()
	services, networks := J{}, merge(obj(r.Routing["networks"]), obj(r.Infrastructure["networks"]))
	for _, name := range keys(obj(r.Model["services"])) {
		svc := J{"labels": J{LOwner: state["owner"], LStack: stack["uid"]}}
		route, data := obj(at(r.Routing, "services", name)), obj(at(r.Infrastructure, "services", name))
		if route["networks"] != nil || data["networks"] != nil {
			nets := merge(obj(route["networks"]), obj(data["networks"]))
			svc["networks"] = nets
			if _, ok := nets["default"]; ok {
				networks["default"] = obj(at(r.Model, "networks", "default"))
			}
		}
		if data["environment"] != nil {
			svc["environment"] = data["environment"]
		}
		services[name] = svc
	}
	doc := J{"services": services}
	if len(networks) > 0 {
		doc["networks"] = networks
	}
	file := filepath.Join(c.Store.Home, "config", "generated", str(stack["uid"])+".json")
	return file, writeCompose(file, doc)
}
func (c *Compose) Execute(ctx context.Context, stack J, mode, action, service string, wait bool, line func(string, string)) (J, error) {
	r, e := c.Approved(ctx, stack, mode)
	if e != nil {
		return nil, e
	}
	file, e := c.Overlay(stack, r)
	if e != nil {
		return nil, e
	}
	args := []string{"--detach"}
	if action == "rebuild" {
		args = append(args, "--build", "--force-recreate")
	}
	if wait {
		if !c.Supports(ctx, "up", "--wait-timeout") {
			return nil, fail("COMPOSE_CAPABILITY", "Actualiza Compose para usar --wait.", 422)
		}
		args = append(args, "--wait", "--wait-timeout", "90")
	}
	if service != "" {
		if !svcRE.MatchString(service) || at(r.Model, "services", service) == nil {
			return nil, fail("SERVICE_NOT_FOUND", "Servicio no encontrado.", 404)
		}
		args = append(args, service)
	}
	_, e = checked(ctx, c.Runner, "docker", composeArgs(stack, mode, str(at(c.Store.Get(), "runtime", "context")), "up", args, file), RunOptions{Dir: str(stack["path"]), Timeout: 30 * time.Minute, Line: line, Redact: r.Redact})
	return J{"services": keys(obj(r.Model["services"])), "mode": mode, "labelFile": file}, e
}

// keep encoding/json referenced for generated schema tests using RawMessage.
var _ = json.Valid
