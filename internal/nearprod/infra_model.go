package nearprod

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var engineOptions = J{
	"postgres": J{"name": "PostgreSQL", "versions": A{"18", "17"}, "defaultVersion": "18", "port": 5432, "suggestedPort": 15432, "memoryMiB": 384, "minMemoryMiB": 256, "user": "postgres"},
	"mysql":    J{"name": "MySQL", "versions": A{"8.4"}, "defaultVersion": "8.4", "port": 3306, "suggestedPort": 13306, "memoryMiB": 768, "minMemoryMiB": 512, "user": "mysql"},
	"redis":    J{"name": "Redis", "versions": A{"7.4", "8"}, "defaultVersion": "7.4", "port": 6379, "suggestedPort": 16379, "memoryMiB": 192, "minMemoryMiB": 128, "user": "redis"},
}
var imagesRE = map[string]*regexp.Regexp{
	"postgres": regexp.MustCompile(`^postgres:(17|18)(?:\.\d+)?(?:-alpine|-bookworm)?$`),
	"mysql":    regexp.MustCompile(`^mysql:8\.4(?:\.\d+)?$`),
	"redis":    regexp.MustCompile(`^redis:(?:7\.4(?:\.\d+)?|8(?:\.\d+){0,2})(?:-alpine)?$`),
}

func imageMajor(image string) int {
	_, tag, ok := strings.Cut(image, ":")
	if !ok {
		return 0
	}
	return integer(regexp.MustCompile(`^\d+`).FindString(tag))
}

func validateFolderPlatform(state, def J) error {
	if runtime.GOOS == "linux" && str(at(state, "runtime", "kind")) == "native" &&
		str(def["engine"]) == "postgres" && imageMajor(str(def["requestedImage"])) >= 18 &&
		str(at(def, "persistence", "kind")) == "folder" {
		return fail("FOLDER_PLATFORM_UNSUPPORTED", "PostgreSQL 18 con Docker nativo en Linux/WSL2 requiere persistencia volume en esta versión. No se modificaron datos.", 409)
	}
	return nil
}

func folderUnsupportedImages(state J) A {
	if runtime.GOOS == "linux" && str(at(state, "runtime", "kind")) == "native" {
		return A{"postgres:18"}
	}
	return A{}
}

func folderRestrictions(state J) A {
	if len(folderUnsupportedImages(state)) == 0 {
		return A{}
	}
	return A{J{"image": "postgres:18", "code": "FOLDER_PLATFORM_UNSUPPORTED", "message": "PostgreSQL 18 con Docker nativo en Linux/WSL2 requiere un volumen administrado por Docker para evitar permisos incompatibles con el UID del motor."}}
}

func normalizeInstance(v J, home string) (J, error) {
	engine := str(v["engine"])
	def := obj(engineOptions[engine])
	if len(def) == 0 {
		return nil, fail("ENGINE_INVALID", "Elige PostgreSQL, MySQL o Redis. Traefik está en Accesos locales.", 400)
	}
	id := text(v["id"], slug(text(v["name"], engine+"-principal")))
	if !validID(id) {
		return nil, fail("INSTANCE_ID", "ID no válido.", 400)
	}
	name, e := requireText(text(v["name"], id), "Nombre", 120)
	if e != nil {
		return nil, e
	}
	image := text(v["image"], text(v["requestedImage"], engine+":"+str(def["defaultVersion"])))
	if !imagesRE[engine].MatchString(image) {
		return nil, fail("IMAGE_UNSUPPORTED", "Usa una etiqueta oficial numérica de una familia admitida; no latest.", 400)
	}
	p := obj(v["persistence"])
	kind := text(p["kind"], "volume")
	if !contains([]string{"volume", "folder", "none"}, kind) || engine != "redis" && kind == "none" {
		return nil, fail("PERSISTENCE_INVALID", "PostgreSQL/MySQL requieren volumen o carpeta.", 400)
	}
	p = J{"kind": kind, "path": p["path"]}
	if kind == "folder" {
		p["path"] = text(p["path"], filepath.Join(home, "databases", engine, id))
	} else {
		delete(p, "path")
	}
	memory := v["memoryMiB"]
	if memory == nil {
		memory = def["memoryMiB"]
	}
	mem, e := intRange(memory, integer(def["minMemoryMiB"]), 32768, "RAM MiB")
	if e != nil {
		return nil, e
	}
	var port any
	if v["hostPort"] != nil && str(v["hostPort"]) != "" {
		n, e := intRange(v["hostPort"], 1024, 65535, "Puerto local")
		if e != nil {
			return nil, e
		}
		port = n
	}
	var conns any
	if engine != "redis" {
		raw := v["maxConnections"]
		if raw == nil {
			raw = 32
		}
		n, e := intRange(raw, 8, 300, "Conexiones")
		if e != nil {
			return nil, e
		}
		conns = n
	}
	return J{"id": id, "name": name, "engine": engine, "requestedImage": image, "hostPort": port, "memoryMiB": mem, "maxConnections": conns, "persistence": p}, nil
}

var dbRE = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)

func validDB(s string) bool {
	return dbRE.MatchString(s) && !strings.HasPrefix(s, "pg_") && !contains([]string{"postgres", "template0", "template1", "mysql", "sys", "information_schema", "performance_schema", "default"}, s)
}
func validUser(s string) bool { return validDB(s) && !contains([]string{"root", "npadmin"}, s) }

var envRE = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,79}$`)
var sensitiveEnvRE = regexp.MustCompile(`^(?:PATH|HOME|SHELL|ENV|BASH_ENV|NODE_OPTIONS|DOCKER_HOST|DOCKER_CONTEXT|LD_.*|DYLD_.*|PYTHONPATH|RUBYOPT|PERL5OPT|COMPOSE_.*)$`)

func normalizedMapping(v J) (J, error) {
	out := J{}
	seen := map[string]bool{}
	for k, raw := range v {
		if !contains([]string{"url", "host", "port", "database", "user", "password"}, k) {
			return nil, fail("BINDING_MAPPING", "Campo de conexión desconocido.", 400)
		}
		value := str(raw)
		if value == "" {
			continue
		}
		if !envRE.MatchString(value) || sensitiveEnvRE.MatchString(value) || seen[value] {
			return nil, fail("BINDING_MAPPING", "Variable repetida, sensible o no válida.", 400)
		}
		seen[value] = true
		out[k] = value
	}
	if out["url"] == nil {
		for _, k := range []string{"host", "port", "user", "password"} {
			if out[k] == nil {
				return nil, fail("BINDING_MAPPING", "Elige URL o host/puerto/usuario/contraseña.", 400)
			}
		}
	}
	return out, nil
}
func validateInfra(v J) error {
	if len(v) == 0 {
		return nil
	}
	for _, k := range []string{"instances", "databases", "bindings"} {
		if _, ok := v[k].([]any); !ok {
			return fail("INFRA_STATE", "Falta una lista de infraestructura: "+k, 409)
		}
	}
	ids, uids, ports := map[string]bool{}, map[string]bool{}, map[int]bool{}
	paths := []string{}
	for _, raw := range arr(v["instances"]) {
		r := obj(raw)
		if _, e := normalizeInstance(r, "/nearprod"); e != nil {
			return e
		}
		uid := str(r["uid"])
		id := str(r["id"])
		if !regexp.MustCompile(`^[a-f0-9]{16}$`).MatchString(uid) || ids[id] || uids[uid] {
			return fail("INFRA_STATE", "Instancia duplicada o UID no válido.", 409)
		}
		ids[id], uids[uid] = true, true
		if dir := str(r["managedDir"]); dir != "" && dir != "config/resources/"+uid {
			return fail("INFRA_PATH", "Directorio interno inválido.", 409)
		}
		for _, k := range []string{"projectName", "network", "hostname"} {
			if !validID(str(r[k])) {
				return fail("INFRA_STATE", "Identidad del recurso inválida: "+k, 409)
			}
		}
		if str(at(r, "persistence", "kind")) == "volume" && !validID(str(r["volume"])) {
			return fail("INFRA_STATE", "Volumen inválido.", 409)
		}
		if hp := integer(r["hostPort"]); hp > 0 {
			if ports[hp] {
				return fail("INFRA_PORT_DUPLICATE", "Puerto local reservado dos veces.", 409)
			}
			ports[hp] = true
		}
		if str(at(r, "persistence", "kind")) == "folder" {
			p := str(at(r, "persistence", "path"))
			if !filepath.IsAbs(p) {
				return fail("INFRA_PATH", "Persistencia requiere ruta absoluta.", 409)
			}
			for _, other := range paths {
				if within(other, p) || within(p, other) {
					return fail("DATA_OVERLAP", "Carpetas de instancias solapadas.", 409)
				}
			}
			paths = append(paths, p)
		}
	}
	dbids, names, users := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, raw := range arr(v["databases"]) {
		d := obj(raw)
		id := str(d["id"])
		uid := str(d["instanceUid"])
		if !validID(id) || !validDB(str(d["name"])) || !validUser(str(d["username"])) || !uids[uid] || dbids[id] || names[uid+":"+str(d["name"])] || users[uid+":"+str(d["username"])] {
			return fail("INFRA_STATE", "Base/cuenta inválida, duplicada o sin instancia.", 409)
		}
		dbids[id], names[uid+":"+str(d["name"])], users[uid+":"+str(d["username"])] = true, true, true
	}
	bIDs := map[string]bool{}
	for _, raw := range arr(v["bindings"]) {
		b := obj(raw)
		if !dbids[str(b["databaseId"])] || str(b["stackUid"]) == "" || !contains([]string{"dev", "verify"}, str(b["mode"])) || len(arr(b["services"])) == 0 || bIDs[str(b["id"])] {
			return fail("INFRA_STATE", "Vinculación inválida.", 409)
		}
		bIDs[str(b["id"])] = true
		if _, e := normalizedMapping(obj(b["mapping"])); e != nil {
			return e
		}
		for _, s := range ss(b["services"]) {
			if !svcRE.MatchString(s) {
				return fail("INFRA_STATE", "Servicio consumidor inválido.", 409)
			}
		}
	}
	return nil
}

func dataDirectory(candidate, home string, empty bool) (string, error) {
	p, e := filepath.Abs(expandHome(candidate))
	if e != nil || candidate == "" || strings.ContainsAny(p, "\x00\r\n,") {
		return "", fail("DATA_PATH", "Elige una carpeta local dedicada.", 400)
	}
	for _, q := range []string{"/Library/CloudStorage/", "/Library/Mobile Documents/"} {
		if strings.Contains(p, q) {
			return "", fail("DATA_CLOUD", "No guardes datos activos de motores en carpetas sincronizadas.", 409)
		}
	}
	for _, q := range []string{"/", userHome(), filepath.Join(userHome(), "Desktop"), filepath.Join(userHome(), "Documents"), filepath.Join(userHome(), "Downloads"), filepath.Join(userHome(), "Library"), filepath.Join(userHome(), ".ssh"), "/Users", "/Volumes", "/tmp", "/var", "/etc", "/opt", home, filepath.Join(home, "databases")} {
		if p == q {
			return "", fail("DATA_PATH_BROAD", "Usa una subcarpeta dedicada, no un directorio general.", 409)
		}
	}
	if within(home, p) && !within(filepath.Join(home, "databases"), p) {
		return "", fail("DATA_HOME", "Dentro de NEARPROD_HOME los datos solo pueden estar bajo databases/<motor>/<instancia>.", 409)
	}
	if within(p, home) {
		return "", fail("DATA_HOME", "La carpeta no puede contener el catálogo de NearProd.", 409)
	}
	ancestor := p
	for {
		st, err := os.Lstat(ancestor)
		if err == nil {
			if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
				return "", fail("DATA_SYMLINK", "Carpeta o padre no puede ser un enlace simbólico.", 409)
			}
			real, err := filepath.EvalSymlinks(ancestor)
			if err != nil {
				return "", err
			}
			if real != ancestor {
				return "", fail("DATA_SYMLINK", "Usa la ruta física de persistencia, sin symlinks.", 409)
			}
			break
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", fail("DATA_PATH", "No se encuentra un padre válido.", 409)
		}
		ancestor = parent
	}
	if empty {
		entries, e := os.ReadDir(p)
		if e == nil && len(entries) > 0 {
			return "", fail("DATA_NOT_EMPTY", "La carpeta debe estar vacía o ser nueva; no se adoptan datos ajenos.", 409)
		}
		if e != nil && !os.IsNotExist(e) {
			return "", e
		}
	}
	return p, nil
}
func sqlString(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
func pgID(s string) string      { return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\"" }
func myID(s string) string      { return "`" + strings.ReplaceAll(s, "`", "``") + "`" }
