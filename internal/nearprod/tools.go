package nearprod

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type ToolsManager struct {
	Runner   Runner
	Store    *Store
	Runtime  *Runtime
	Platform string
}

func (t *ToolsManager) platform() string {
	if t.Platform != "" {
		return t.Platform
	}
	if t.Runtime != nil {
		return t.Runtime.platform()
	}
	return runtime.GOOS
}
func locate(name string) J {
	p := findExecutable(name)
	if p == "" {
		return nil
	}
	rp, e := filepath.EvalSymlinks(p)
	if e != nil {
		return nil
	}
	return J{"path": p, "realPath": rp}
}
func nodeManager(p string) string {
	for _, v := range []struct {
		id    string
		parts []string
	}{{"fnm", []string{"/fnm/", "/fnm_multishells/"}}, {"nvm", []string{"/.nvm/versions/", "/nvm/versions/"}}, {"volta", []string{"/.volta/"}}, {"asdf", []string{"/.asdf/"}}, {"mise", []string{"/mise/"}}, {"n", []string{"/n/versions/"}}, {"homebrew", []string{"/Cellar/node"}}} {
		for _, part := range v.parts {
			if strings.Contains(p, part) {
				return v.id
			}
		}
	}
	return "unknown"
}
func definition(id string) (toolDefinition, error) {
	for _, d := range toolDefinitions {
		if id == d.ID {
			return d, nil
		}
	}
	return toolDefinition{}, fail("TOOL_INVALID", "Herramienta no administrada.", 400)
}
func formulaName(id string) string {
	if id == "compose" || id == "buildx" {
		return "docker-" + id
	}
	return id
}

var formulaRE = regexp.MustCompile(`^(docker|docker-compose|docker-buildx|colima)(@\d+(?:\.\d+)*)?$`)

func (t *ToolsManager) Inventory(ctx context.Context) J {
	capabilities := platformCapabilitiesFor(t.Store.Get(), t.platform(), runtime.GOARCH, "")
	managers := A{}
	for _, pair := range [][2]string{{"brew", "system"}, {"fnm", "node"}, {"volta", "node"}, {"asdf", "runtimes"}, {"mise", "runtimes"}, {"n", "node"}, {"npm", "node-packages"}, {"pnpm", "node-packages"}, {"port", "system"}, {"nix", "system"}, {"apt-get", "system"}} {
		if f := locate(pair[0]); f != nil {
			managers = append(managers, merge(f, J{"id": pair[0], "scope": pair[1], "evidence": "executable", "managed": pair[0] == "brew" && t.platform() == "darwin"}))
		}
	}
	nvm := os.Getenv("NVM_DIR")
	if nvm == "" {
		nvm = filepath.Join(userHome(), ".nvm")
	}
	if st, e := os.Lstat(filepath.Join(nvm, "nvm.sh")); e == nil && st.Mode().IsRegular() {
		managers = append(managers, J{"id": "nvm", "scope": "node", "path": filepath.Join(nvm, "nvm.sh"), "evidence": "shell-file", "managed": false})
	}
	tools := A{}
	statuses := t.Runtime.toolStatus(ctx, t.platform())
	for n, d := range toolDefinitions {
		v := obj(statuses[n])
		selected := platformToolPath(t.Store.Get(), t.platform(), d.Command)
		f := locate(text(selected, d.Command))
		provider := "none"
		if f != nil {
			provider = "existing"
			if strings.Contains(str(f["realPath"]), "/Cellar/") {
				provider = "homebrew"
			}
		}
		var standalone any
		if !truth(v["available"]) && d.Fallback != "" {
			standalone = locate(d.Fallback)
		}
		tools = append(tools, merge(v, J{"formula": formulaName(d.ID), "command": d.Command, "path": f["path"], "realPath": f["realPath"], "provider": provider, "selected": selected != "", "standalone": standalone}))
	}
	var node any
	if f := locate("node"); f != nil {
		node = merge(f, J{"manager": nodeManager(str(f["realPath"])), "required": false, "note": "Node solo para tus proyectos/desarrollar la UI; no ejecuta NearProd."})
	}
	recommendation := "NearProd es un binario Go, independiente de fnm/nvm. Conserva tus gestores; en Linux/WSL2 NearProd detecta herramientas, pero no instala ni actualiza paquetes del sistema."
	if t.platform() == "darwin" {
		recommendation = "NearProd es un binario Go, independiente de fnm/nvm. Conserva tus gestores; la instalación guiada de herramientas usa Homebrew en macOS."
	}
	return J{"platform": t.platform(), "packageManagement": capabilities["packageManagement"], "managers": managers, "binary": J{"language": "Go", "version": runtime.Version(), "path": binaryPath(), "requiresNode": false}, "projectNode": node, "tools": tools, "traefik": J{"category": "core-proxy", "managedBy": "nearprod", "message": "Traefik se administra desde Accesos locales, dentro de Docker."}, "recommendation": recommendation, "checkedAt": now()}
}
func (t *ToolsManager) Brew() (J, error) {
	if t.platform() != "darwin" {
		return nil, fail("PACKAGE_PLATFORM", "Instalación guiada: Homebrew en macOS. Diagnóstico disponible en Linux.", 400)
	}
	f := locate("brew")
	if f == nil {
		return nil, fail("BREW_MISSING", "Homebrew no encontrado. No se instalará automáticamente.", 409)
	}
	return f, nil
}
func (t *ToolsManager) formulaInfo(ctx context.Context, formula, brew string) (J, error) {
	if !formulaRE.MatchString(formula) {
		return nil, fail("FORMULA_DENIED", "Fórmula fuera del catálogo permitido.", 403)
	}
	res, e := checked(ctx, t.Runner, brew, []string{"info", "--json=v2", "--formula", formula}, RunOptions{Timeout: 45 * time.Second, Exact: true, Limit: 2 << 20})
	if e != nil {
		return nil, e
	}
	v, e := decodeObject([]byte(res.Stdout))
	if e != nil {
		return nil, e
	}
	fs := arr(v["formulae"])
	if len(fs) != 1 || str(obj(fs[0])["name"]) != formula || str(at(fs[0], "versions", "stable")) == "" {
		return nil, fail("BREW_RESPONSE", "Homebrew no devolvió la fórmula exacta esperada.", 422)
	}
	return obj(fs[0]), nil
}
func (t *ToolsManager) Versions(ctx context.Context, id string) (J, error) {
	if _, e := definition(id); e != nil {
		return nil, e
	}
	brew, e := t.Brew()
	if e != nil {
		return nil, e
	}
	main, e := t.formulaInfo(ctx, formulaName(id), str(brew["path"]))
	if e != nil {
		return nil, e
	}
	candidates := A{main}
	for n, v := range ss(main["versioned_formulae"]) {
		if n >= 8 {
			break
		}
		if formulaRE.MatchString(v) && strings.HasPrefix(v, formulaName(id)+"@") {
			if f, e := t.formulaInfo(ctx, v, str(brew["path"])); e == nil {
				candidates = append(candidates, f)
			}
		}
	}
	options := A{}
	for _, raw := range candidates {
		f := obj(raw)
		if truth(f["disabled"]) {
			continue
		}
		installed := A{}
		for _, v := range arr(f["installed"]) {
			installed = append(installed, obj(v)["version"])
		}
		options = append(options, J{"formula": f["name"], "version": at(f, "versions", "stable"), "installed": installed, "pinned": truth(f["pinned"]), "kegOnly": truth(f["keg_only"]), "dependencies": list(f["dependencies"]), "deprecated": truth(f["deprecated"])})
	}
	return J{"tool": id, "provider": "homebrew", "options": options, "checkedAt": now(), "note": "Solo opciones reales ofrecidas por Homebrew; no todas las versiones históricas. Conservar tu instalación no ejecuta cambios."}, nil
}
func dockerConfigPath() string {
	d := os.Getenv("DOCKER_CONFIG")
	if d == "" {
		d = filepath.Join(userHome(), ".docker")
	}
	return filepath.Join(d, "config.json")
}
func dockerConfig(file string) (J, string, error) {
	st, e := regularOrMissing(file)
	if e != nil {
		return nil, "", e
	}
	if st == nil {
		return J{}, hash(""), nil
	}
	if st.Size() > 1<<20 {
		return nil, "", fail("DOCKER_CONFIG_SIZE", "config.json demasiado grande.", 413)
	}
	b, e := readLimited(file, 1<<20)
	if e != nil {
		return nil, "", e
	}
	v, e := decodeObject(b)
	if e != nil {
		return nil, "", fail("DOCKER_CONFIG_JSON", "config.json inválido; no se reemplazará.", 409)
	}
	if dirs, ok := v["cliPluginsExtraDirs"]; ok {
		a, ok := dirs.([]any)
		if !ok {
			return nil, "", fail("DOCKER_CONFIG_JSON", "cliPluginsExtraDirs debe ser una lista de rutas.", 409)
		}
		for _, p := range a {
			if _, ok := p.(string); !ok {
				return nil, "", fail("DOCKER_CONFIG_JSON", "La lista de plugins contiene valores inválidos.", 409)
			}
		}
	}
	return v, hash(string(b)), nil
}
func (t *ToolsManager) Preview(ctx context.Context, req J) (J, error) {
	id := str(req["tool"])
	d, e := definition(id)
	if e != nil {
		return nil, e
	}
	action := text(req["action"], "install")
	if !contains([]string{"install", "upgrade", "repair"}, action) {
		return nil, fail("TOOL_ACTION", "Usa install, upgrade o repair.", 400)
	}
	if action == "repair" && !contains([]string{"compose", "buildx"}, id) {
		return nil, fail("REPAIR_UNSUPPORTED", "Reparación de plugins Compose/Buildx únicamente.", 400)
	}
	brew, e := t.Brew()
	if e != nil {
		return nil, e
	}
	versions, e := t.Versions(ctx, id)
	if e != nil {
		return nil, e
	}
	formula := text(req["formula"], formulaName(id))
	var choice J
	for _, v := range arr(versions["options"]) {
		if str(obj(v)["formula"]) == formula {
			choice = obj(v)
		}
	}
	if choice == nil {
		return nil, fail("VERSION_UNAVAILABLE", "La fórmula no está disponible; consulta las opciones.", 409)
	}
	if action == "upgrade" && truth(choice["pinned"]) {
		return nil, fail("FORMULA_PINNED", "Fórmula fijada; no se ejecuta unpin.", 409)
	}
	res, e := checked(ctx, t.Runner, str(brew["path"]), []string{"--prefix"}, RunOptions{Timeout: 15 * time.Second})
	if e != nil {
		return nil, e
	}
	prefix := strings.TrimSpace(res.Stdout)
	if !filepath.IsAbs(prefix) || strings.ContainsAny(prefix, "\r\n\x00") {
		return nil, fail("BREW_PREFIX", "Prefijo no válido.", 422)
	}
	fprefix := prefix
	if truth(choice["kegOnly"]) {
		res, e = checked(ctx, t.Runner, str(brew["path"]), []string{"--prefix", formula}, RunOptions{})
		if e != nil {
			return nil, e
		}
		fprefix = strings.TrimSpace(res.Stdout)
	}
	if !filepath.IsAbs(fprefix) || strings.ContainsAny(fprefix, "\r\n\x00") {
		return nil, fail("BREW_PREFIX", "Prefijo versionado inválido.", 422)
	}
	var plugin, config, configHash any
	if id == "compose" || id == "buildx" {
		plugin = filepath.Join(fprefix, "lib", "docker", "cli-plugins")
		file := dockerConfigPath()
		_, h, e := dockerConfig(file)
		if e != nil {
			return nil, e
		}
		config, configHash = file, h
	}
	warnings := A{}
	current := locate(text(platformToolPath(t.Store.Get(), t.platform(), d.Command), d.Command))
	if current != nil && !strings.Contains(str(current["realPath"]), "/Cellar/") {
		warnings = append(warnings, "Existe una instalación ajena: no se elimina ni cambia el PATH global. Se seleccionará Homebrew para NearProd.")
	}
	if truth(choice["deprecated"]) {
		warnings = append(warnings, "Fórmula marcada obsoleta por Homebrew.")
	}
	if len(arr(choice["dependencies"])) > 0 {
		warnings = append(warnings, "Homebrew puede instalar/actualizar dependencias necesarias; no se ejecuta actualización global.")
	}
	var command any
	if action != "repair" {
		command = []string{str(brew["path"]), action, "--formula", "--force-bottle", formula}
	}
	p := J{"tool": id, "formula": formula, "version": choice["version"], "installed": choice["installed"], "action": action, "provider": "homebrew", "brewPath": brew["path"], "brewRealPath": brew["realPath"], "prefix": prefix, "command": command, "pluginDir": plugin, "configFile": config, "configHash": configHash, "dependencies": choice["dependencies"], "warnings": warnings}
	p["fingerprint"] = hash(p)
	p["note"] = "Revisa y confirma. No cambia Node, no ejecuta sudo y no instala paquetes del proyecto."
	return p, nil
}
func (t *ToolsManager) Apply(ctx context.Context, req J, line func(string, string)) (J, error) {
	if !truth(req["confirm"]) {
		return nil, fail("CONFIRM_REQUIRED", "Confirma instalar/reparar la herramienta.", 409)
	}
	p, e := t.Preview(ctx, req)
	if e != nil {
		return nil, e
	}
	if str(p["fingerprint"]) != str(req["fingerprint"]) {
		return nil, fail("PREVIEW_CHANGED", "Plan o metadatos cambiaron; vuelve a revisar.", 409)
	}
	state := str(t.Runtime.Status(ctx)["state"])
	if str(req["tool"]) == "colima" && state != "stopped" && !(str(p["action"]) == "install" && state == "missing") {
		return nil, fail("COLIMA_MAINTENANCE", "Para actualizar Colima el perfil debe estar detenido y comprobado.", 409)
	}
	if str(p["action"]) != "repair" {
		args := ss(p["command"])
		_, e = checked(ctx, t.Runner, args[0], args[1:], RunOptions{Timeout: 30 * time.Minute, Line: line, Env: map[string]string{"HOMEBREW_NO_AUTO_UPDATE": "1", "HOMEBREW_NO_INSTALL_CLEANUP": "1", "HOMEBREW_NO_INSTALL_UPGRADE": "1", "HOMEBREW_NO_INSTALLED_DEPENDENTS_CHECK": "1", "HOMEBREW_NO_ANALYTICS": "1", "HOMEBREW_NO_ASK": "1", "NONINTERACTIVE": "1"}})
		if e != nil {
			return nil, e
		}
	}
	var backup any
	if dir := str(p["pluginDir"]); dir != "" {
		config, fileHash, e := dockerConfig(str(p["configFile"]))
		if e != nil {
			return nil, e
		}
		if fileHash != str(p["configHash"]) {
			return nil, fail("DOCKER_CONFIG_CHANGED", "Docker config cambió durante la instalación. No se sobrescribió. Ejecuta Reparar otra vez.", 409)
		}
		dirs := ss(config["cliPluginsExtraDirs"])
		if !contains(dirs, dir) {
			file := str(p["configFile"])
			if st, _ := os.Lstat(file); st != nil {
				b, e := readLimited(file, 1<<20)
				if e != nil {
					return nil, e
				}
				dest := file + ".nearprod-" + token(8) + ".bak"
				if e = atomicBytes(dest, b, 0600); e != nil {
					return nil, e
				}
				backup = dest
			}
			config["cliPluginsExtraDirs"] = append(dirs, dir)
			if e = writeJSON(file, config); e != nil {
				return nil, e
			}
		}
	}
	d, _ := definition(str(req["tool"]))
	if d.ID == "docker" || d.ID == "colima" {
		res, e := checked(ctx, t.Runner, str(p["brewPath"]), []string{"--prefix", str(p["formula"])}, RunOptions{})
		if e != nil {
			return nil, e
		}
		pre := strings.TrimSpace(res.Stdout)
		if !filepath.IsAbs(pre) || strings.ContainsAny(pre, "\r\n\x00") {
			return nil, fail("BREW_PREFIX", "Prefijo no válido.", 422)
		}
		exe := locate(filepath.Join(pre, "bin", d.Command))
		if exe == nil {
			return nil, fail("TOOL_VERIFY", "No se encontró el binario tras la instalación.", 503)
		}
		if _, e = checked(ctx, t.Runner, str(exe["path"]), d.Args, RunOptions{}); e != nil {
			return nil, e
		}
		if e = t.Store.Update(func(v J) error { obj(v["toolPaths"])[d.Command] = exe["path"]; return nil }); e != nil {
			return nil, e
		}
	} else {
		if _, e = checked(ctx, t.Runner, "docker", d.Args, RunOptions{}); e != nil {
			return nil, e
		}
		if d.ID == "compose" {
			r, e := checked(ctx, t.Runner, "docker", []string{"compose", "up", "--help"}, RunOptions{})
			if e != nil {
				return nil, e
			}
			if !strings.Contains(r.Stdout, "--wait-timeout") {
				return nil, fail("COMPOSE_CAPABILITY", "Compose no ofrece las capacidades necesarias.", 422)
			}
		}
	}
	return J{"tool": d.ID, "verified": true, "configBackup": backup, "inventory": t.Inventory(ctx), "note": "Comando verificado. Ningún repositorio ni gestor Node fue modificado."}, nil
}

var _ = json.Valid
