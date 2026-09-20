package nearprod

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Runtime struct {
	Runner   Runner
	Store    *Store
	Docker   *Docker
	Platform string
}

func (r *Runtime) platform() string {
	if r.Platform != "" {
		return r.Platform
	}
	return runtime.GOOS
}
func (r *Runtime) Capabilities() J {
	return platformCapabilitiesFor(r.Store.Get(), r.platform(), runtime.GOARCH, "")
}
func (r *Runtime) managedVirtualMachine() bool {
	return truth(at(r.Capabilities(), "runtime", "managedVirtualMachine"))
}
func (r *Runtime) supported() bool {
	return truth(at(r.Capabilities(), "runtime", "supported"))
}
func (r *Runtime) Prefix(args ...string) []string {
	return append([]string{"--profile", text(at(r.Store.Get(), "runtime", "profile"), "default")}, args...)
}
func (r *Runtime) Allocation() (J, error) {
	if !r.managedVirtualMachine() {
		return nil, nil
	}
	profile := text(at(r.Store.Get(), "runtime", "profile"), "default")
	if !validID(profile) {
		return nil, fail("PROFILE_INVALID", "Perfil Colima no válido.", 400)
	}
	base := os.Getenv("COLIMA_HOME")
	if base == "" {
		base = filepath.Join(userHome(), ".colima")
	}
	raw, e := readLimited(filepath.Join(base, profile, "colima.yaml"), 1<<20)
	if os.IsNotExist(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	values := J{}
	for _, line := range strings.Split(string(raw), "\n") {
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			values[strings.TrimSpace(parts[0])] = strings.Trim(strings.TrimSpace(strings.SplitN(parts[1], " #", 2)[0]), "\"'")
		}
	}
	return J{"memoryGiB": num(values["memory"]), "cpus": integer(values["cpu"]), "vmType": text(values["vmType"], "unknown"), "architecture": text(values["arch"], "default"), "mountType": text(values["mountType"], "default"), "runtime": text(values["runtime"], "docker"), "path": filepath.Join(base, profile, "colima.yaml")}, nil
}

var stoppedRE = regexp.MustCompile(`(?i)not running|is stopped|has not been started`)

func (r *Runtime) Status(ctx context.Context) J {
	if !r.supported() {
		return J{"state": "unsupported", "message": "El runtime guardado no está soportado en esta plataforma. Selecciona una opción disponible antes de usar Docker."}
	}
	if !r.managedVirtualMachine() {
		return J{"state": "not-applicable", "message": "Docker nativo sin máquina virtual administrada por NearProd."}
	}
	profile := at(r.Store.Get(), "runtime", "profile")
	a, e := r.Allocation()
	if e != nil {
		return J{"state": "unknown", "profile": profile, "message": "Configuración Colima inaccesible.", "error": publicError(e)}
	}
	if a == nil {
		return J{"state": "missing", "profile": profile, "message": "El perfil no existe. No se crea otra VM automáticamente."}
	}
	result, e := r.Runner.Run(ctx, "colima", r.Prefix("status"), RunOptions{Timeout: 15 * time.Second})
	if e == nil && result.Code == 0 {
		return J{"state": "running", "profile": profile, "message": "Colima está en ejecución."}
	}
	if e == nil && stoppedRE.MatchString(result.Stdout+result.Stderr) {
		return J{"state": "stopped", "profile": profile, "message": "Colima está detenido."}
	}
	return J{"state": "unknown", "profile": profile, "message": "No se pudo comprobar Colima; no se supondrá detenido."}
}

type toolDefinition struct {
	ID, Name, Command string
	Args              []string
	Fallback          string
	FallbackArgs      []string
	Required          bool
	Purpose, Hint     string
}

var toolDefinitions = []toolDefinition{
	{"docker", "Docker CLI", "docker", []string{"--version"}, "", nil, true, "Comunicación con Docker Engine y plugins.", "brew install docker"},
	{"compose", "Compose", "docker", []string{"compose", "version", "--short"}, "docker-compose", []string{"version", "--short"}, true, "Combina los archivos Compose y administra los servicios de cada aplicación.", "brew install docker-compose; si existe, revisa cliPluginsExtraDirs."},
	{"buildx", "Buildx", "docker", []string{"buildx", "version"}, "docker-buildx", []string{"version"}, false, "Construye imágenes con BuildKit. No necesario para consultar logs o detener.", "brew install docker-buildx; si existe, revisa cliPluginsExtraDirs."},
	{"colima", "Colima", "colima", []string{"version"}, "", nil, true, "Gestiona la VM Linux compartida; no una VM por proyecto.", "brew install colima"},
}

func (r *Runtime) ToolStatus(ctx context.Context) A {
	return r.toolStatus(ctx, r.platform())
}
func (r *Runtime) toolStatus(ctx context.Context, platform string) A {
	out := make(A, len(toolDefinitions))
	var wg sync.WaitGroup
	for index, def := range toolDefinitions {
		wg.Add(1)
		go func(n int, d toolDefinition) {
			defer wg.Done()
			supported := d.ID != "colima" || platform == "darwin" && str(at(r.Store.Get(), "runtime", "kind")) == "colima"
			hint := d.Hint
			if platform != "darwin" {
				hint = map[string]string{
					"docker":  "Instala Docker CLI con el mecanismo aprobado para tu distribución.",
					"compose": "Instala o habilita el plugin Docker Compose para tu Docker CLI.",
					"buildx":  "Instala o habilita el plugin Docker Buildx para tu Docker CLI.",
					"colima":  "Colima solo aplica al runtime administrado en macOS.",
				}[d.ID]
			}
			v := J{"id": d.ID, "name": d.Name, "required": d.Required && supported, "supported": supported, "purpose": d.Purpose, "hint": hint, "available": false, "status": "unavailable", "version": nil}
			if !supported {
				v["status"] = "not-applicable"
				out[n] = v
				return
			}
			res, e := r.Runner.Run(ctx, d.Command, d.Args, RunOptions{Timeout: 15 * time.Second})
			if e == nil && res.Code == 0 {
				v["available"] = true
				v["status"] = "available"
				v["version"] = bounded(strings.TrimSpace(res.Stdout+res.Stderr), 350)
			} else if d.Fallback != "" {
				res, e = r.Runner.Run(ctx, d.Fallback, d.FallbackArgs, RunOptions{Timeout: 10 * time.Second})
				if e == nil && res.Code == 0 {
					v["status"] = "plugin-not-registered"
					v["standaloneVersion"] = bounded(strings.TrimSpace(res.Stdout+res.Stderr), 200)
					v["hint"] = "El plugin existe, pero Docker no lo encuentra. Herramientas → Reparar conserva config.json."
				}
			}
			out[n] = v
		}(index, def)
	}
	wg.Wait()
	return out
}
func (r *Runtime) Doctor(ctx context.Context) J {
	info, e := r.Docker.Info(ctx)
	allocation, ae := r.Allocation()
	j := J{"nearprod": Version, "runtimeLanguage": "Go", "goVersion": runtime.Version(), "binary": binaryPath(), "platform": r.platform() + "/" + runtime.GOARCH, "host": r.Capabilities(), "tools": r.ToolStatus(ctx), "runtime": r.Store.Get()["runtime"], "colima": r.Status(ctx), "allocation": allocation, "engine": info, "hostMemoryBytes": hostMemory(), "agentRssBytes": processRSS(), "checkedAt": now()}
	if e != nil {
		j["error"] = publicError(e)
	}
	if ae != nil {
		j["allocationError"] = publicError(ae)
	}
	return j
}
func (r *Runtime) Start(ctx context.Context, line func(string, string)) (J, error) {
	if !r.supported() {
		return nil, fail("RUNTIME_PLATFORM", "El runtime guardado no está soportado en esta plataforma.", 409)
	}
	if !r.managedVirtualMachine() {
		return nil, fail("RUNTIME_NATIVE", "Inicia Docker con tu administrador del sistema; no se ejecuta sudo.", 409)
	}
	a, e := r.Allocation()
	if e != nil {
		return nil, e
	}
	if a == nil {
		return nil, fail("COLIMA_PROFILE_MISSING", "Crea explícitamente el perfil Colima antes de administrarlo.", 409)
	}
	if str(a["runtime"]) != "docker" {
		return nil, fail("COLIMA_RUNTIME", "El perfil seleccionado no usa Docker.", 409)
	}
	status := r.Status(ctx)
	if str(status["state"]) == "running" {
		info, e := r.Docker.Info(ctx)
		return merge(info, J{"noOp": true}), e
	}
	if str(status["state"]) != "stopped" {
		return nil, fail("COLIMA_STATUS_UNKNOWN", str(status["message"]), 503)
	}
	if e = r.activateCapability(ctx); e != nil {
		return nil, e
	}
	if _, e = checked(ctx, r.Runner, "colima", r.Prefix("start", "--activate=false"), RunOptions{Timeout: 10 * time.Minute, Line: line}); e != nil {
		return nil, e
	}
	return r.Docker.Info(ctx)
}
func (r *Runtime) activateCapability(ctx context.Context) error {
	v, e := checked(ctx, r.Runner, "colima", []string{"start", "--help"}, RunOptions{})
	if e != nil {
		return e
	}
	if !strings.Contains(v.Stdout, "--activate") {
		return fail("COLIMA_CAPABILITY", "Colima debe admitir --activate=false para no cambiar tu contexto global.", 409)
	}
	return nil
}
func (r *Runtime) Preview(ctx context.Context, req J) (J, error) {
	if !r.supported() {
		return nil, fail("RUNTIME_PLATFORM", "El runtime guardado no está soportado en esta plataforma.", 409)
	}
	if !r.managedVirtualMachine() {
		return nil, fail("RUNTIME_NATIVE", "La asignación de VM pertenece a Colima.", 400)
	}
	mem := num(req["memory"])
	total := hostMemory()
	if mem < 0.5 || mem > float64(total)/1073741824 {
		return nil, fail("INVALID_MEMORY", "Usa RAM en GiB desde 0.5 hasta la memoria física disponible del host.", 400)
	}
	cpus, e := intRange(req["cpus"], 1, runtime.NumCPU(), "CPU")
	if e != nil {
		return nil, e
	}
	allocation, e := r.Allocation()
	if e != nil {
		return nil, e
	}
	if allocation == nil {
		return nil, fail("COLIMA_PROFILE_MISSING", "Perfil no encontrado.", 409)
	}
	status := r.Status(ctx)
	running := str(status["state"]) == "running"
	if !running && str(status["state"]) != "stopped" {
		return nil, fail("COLIMA_STATUS_UNKNOWN", str(status["message"]), 503)
	}
	affected, ids := A{}, []string{}
	var info J
	if running {
		info, e = r.Docker.Info(ctx)
		if e != nil {
			return nil, e
		}
		cs, e := r.Docker.Containers(ctx, true)
		if e != nil {
			return nil, e
		}
		for _, v := range cs {
			c := obj(v)
			if truth(c["running"]) {
				affected = append(affected, J{"id": c["id"], "name": c["name"], "project": c["project"]})
				ids = append(ids, str(c["id"]))
			}
		}
	}
	ids = sortedStrings(ids)
	p := J{"memory": mem, "cpus": cpus, "allocation": allocation, "affected": affected, "running": running, "warning": "Reinicia TODO Colima, incluidos contenedores ajenos. No se promete reinicio automático de los proyectos."}
	p["fingerprint"] = hash(J{"runtime": r.Store.Get()["runtime"], "allocation": allocation, "memory": mem, "cpus": cpus, "engine": info["id"], "affected": ids, "running": running})
	return p, nil
}
func (r *Runtime) Configure(ctx context.Context, req J, line func(string, string)) (J, error) {
	if !truth(req["confirm"]) {
		return nil, fail("CONFIRM_REQUIRED", "Confirma el reinicio global del runtime.", 409)
	}
	p, e := r.Preview(ctx, req)
	if e != nil {
		return nil, e
	}
	if str(p["fingerprint"]) != str(req["fingerprint"]) {
		return nil, fail("PREVIEW_CHANGED", "El alcance del reinicio cambió.", 409)
	}
	if e = r.activateCapability(ctx); e != nil {
		return nil, e
	}
	if truth(p["running"]) {
		if _, e = checked(ctx, r.Runner, "colima", r.Prefix("stop"), RunOptions{Timeout: 5 * time.Minute, Line: line}); e != nil {
			return nil, e
		}
	}
	if _, e = checked(ctx, r.Runner, "colima", r.Prefix("start", "--activate=false", "--cpu", str(req["cpus"]), "--memory", str(req["memory"])), RunOptions{Timeout: 10 * time.Minute, Line: line}); e != nil {
		return nil, e
	}
	info, e := r.Docker.Info(ctx)
	if e != nil {
		return nil, e
	}
	allocation, e := r.Allocation()
	return J{"engine": info, "allocation": allocation, "affected": p["affected"], "note": "Revisa qué aplicaciones se recuperaron. No se restauran procesos automáticamente."}, e
}
func (r *Runtime) Metrics(ctx context.Context) J {
	info, e := r.Docker.Info(ctx)
	containers := A{}
	var guest any
	if e == nil {
		containers, e = r.Docker.Stats(ctx)
	}
	if info != nil && r.managedVirtualMachine() {
		res, err := checked(ctx, r.Runner, "colima", r.Prefix("ssh", "--", "cat", "/proc/meminfo"), RunOptions{Timeout: 10 * time.Second})
		if err == nil {
			v := parseMeminfo(res.Stdout)
			if v["MemTotal"] > 0 {
				guest = J{"totalBytes": v["MemTotal"], "availableBytes": v["MemAvailable"], "swapTotalBytes": v["SwapTotal"], "swapFreeBytes": v["SwapFree"]}
			}
		}
	}
	allocation, _ := r.Allocation()
	hostNote := "Consulta la presión de memoria con las herramientas del sistema operativo."
	if r.platform() == "darwin" {
		hostNote = "Consulta la presión de memoria en Monitor de Actividad."
	} else if detectHostEnvironment(r.platform()) == "wsl2" {
		hostNote = "Consulta la presión de memoria de WSL2 y Windows; el límite global no lo administra NearProd."
	}
	note := "La memoria del Engine y los contenedores es consumo relacionado, no valores para sumar. La pestaña del navegador se mide aparte."
	if r.managedVirtualMachine() {
		note = "No sumes memoria de los contenedores a la VM: consumos anidados. La pestaña de navegador se mide aparte."
	}
	return J{"host": J{"totalBytes": hostMemory(), "freeBytes": nil, "note": hostNote}, "agent": J{"rssBytes": processRSS(), "runtime": "Go"}, "allocation": allocation, "guest": guest, "engine": info, "containers": containers, "error": publicError(e), "checkedAt": now(), "note": note}
}
func parseMeminfo(s string) map[string]int64 {
	v := map[string]int64{}
	for _, l := range strings.Split(s, "\n") {
		f := strings.Fields(l)
		if len(f) >= 2 {
			n, _ := strconv.ParseInt(f[1], 10, 64)
			v[strings.TrimSuffix(f[0], ":")] = n * 1024
		}
	}
	return v
}
func (r *Runtime) Updates(ctx context.Context) (J, error) {
	if r.platform() != "darwin" {
		return J{"status": "unsupported", "items": A{}, "checkedAt": now(), "message": "NearProd no administra actualizaciones de herramientas en Linux/WSL2; usa el mecanismo aprobado de tu distribución."}, nil
	}
	res, e := r.Runner.Run(ctx, "brew", []string{"outdated", "--json=v2", "--formula"}, RunOptions{Exact: true, Limit: 2 << 20, Timeout: 45 * time.Second})
	if e != nil {
		return J{"status": "unavailable", "items": A{}, "checkedAt": now(), "message": "Homebrew no disponible; no se cambió ningún paquete."}, nil
	}
	if res.Code != 0 {
		return J{"status": "error", "items": A{}, "checkedAt": now(), "message": "No se pudo leer Homebrew. No se actualizaron paquetes."}, nil
	}
	var data J
	if e = json.Unmarshal([]byte(res.Stdout), &data); e != nil {
		return nil, fail("BREW_RESPONSE", "Respuesta Homebrew no válida.", 422)
	}
	items := A{}
	for _, f := range arr(data["formulae"]) {
		v := obj(f)
		if contains([]string{"docker", "colima", "docker-compose", "docker-buildx"}, str(v["name"])) {
			items = append(items, J{"name": v["name"], "installed": v["installed_versions"], "latestKnown": v["current_version"], "command": "brew upgrade " + str(v["name"])})
		}
	}
	return J{"status": "cached-metadata", "checkedAt": now(), "message": "Metadatos locales; no prueba que sean los últimos de Internet. No se instaló ninguna actualización.", "items": items, "engine": "Engine y Docker CLI tienen versiones diferentes."}, nil
}
func binaryPath() string {
	p, e := os.Executable()
	if e != nil {
		return "unknown"
	}
	if real, e := filepath.EvalSymlinks(p); e == nil {
		return real
	}
	return p
}

var _ = fmt.Sprintf
