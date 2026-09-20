package nearprod

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Store has one writer in the agent; its path never includes an app release.
// Docker metadata and physical data paths are preserved verbatim during upgrade.
type Store struct {
	Home, File string
	mu         sync.RWMutex
	value      J
}

func initialState() J {
	kind, context := "native", "default"
	if runtime.GOOS == "darwin" {
		kind, context = "colima", "colima"
	}
	return J{"version": SchemaVersion, "owner": token(24), "roots": A{}, "runtime": J{"kind": kind, "context": context, "profile": "default"}, "groups": A{}, "stacks": A{}, "archivedStacks": A{}, "operations": A{}, "toolPaths": J{}, "infra": J{"instances": A{}, "databases": A{}, "archivedDatabases": A{}, "bindings": A{}, "archivedInstances": A{}}}
}
func validateState(v J) error {
	if num(v["version"]) != float64(SchemaVersion) || len(str(v["owner"])) < 8 {
		return fail("INVALID_STATE", "Catálogo inválido o de una versión no compatible. No se sobrescribirá.", 409)
	}
	for _, k := range []string{"roots", "stacks", "operations", "groups"} {
		if _, ok := v[k].([]any); !ok {
			return fail("INVALID_STATE", "Falta la lista "+k+" del catálogo.", 409)
		}
	}
	if v["archivedStacks"] != nil {
		if _, ok := v["archivedStacks"].([]any); !ok {
			return fail("INVALID_STATE", "La lista de aplicaciones archivadas no es válida.", 409)
		}
	}
	rt := obj(v["runtime"])
	if !contains([]string{"colima", "native"}, str(rt["kind"])) || !validID(str(rt["context"])) || !validID(str(rt["profile"])) {
		return fail("INVALID_STATE", "Runtime inválido.", 409)
	}
	for _, root := range arr(v["roots"]) {
		if !filepath.IsAbs(str(root)) || strings.ContainsAny(str(root), "\x00\r\n") {
			return fail("INVALID_STATE", "Raíz no válida.", 409)
		}
	}
	groupIDs := map[string]bool{}
	for _, raw := range arr(v["groups"]) {
		g := obj(raw)
		if !validID(str(g["id"])) || groupIDs[str(g["id"])] || str(g["name"]) == "" {
			return fail("INVALID_STATE", "Grupo inválido o repetido.", 409)
		}
		groupIDs[str(g["id"])] = true
	}
	ids, projects, uids, hosts := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	validateStack := func(s J) error {
		id := str(s["id"])
		uid := str(s["uid"])
		project := str(s["projectName"])
		if !validID(str(s["product"])) || !validID(str(s["slug"])) || id != str(s["product"])+"/"+str(s["slug"]) || !validID(project) || ids[id] || projects[project] || !safeTokenRE.MatchString(uid) || uids[uid] || !filepath.IsAbs(str(s["path"])) {
			return fail("INVALID_STATE", "Identidad de aplicación inválida o duplicada.", 409)
		}
		ids[id], projects[project], uids[uid] = true, true, true
		if len(obj(at(s, "modes", "dev"))) == 0 {
			return fail("INVALID_STATE", "Modo de desarrollo ausente.", 409)
		}
		for mode, m := range obj(s["modes"]) {
			if !contains([]string{"dev", "verify"}, mode) {
				return fail("INVALID_STATE", "Modo no válido.", 409)
			}
			if len(arr(at(m, "files"))) == 0 {
				return fail("INVALID_STATE", "Modo sin archivos Compose.", 409)
			}
		}
		routes, e := normalizeRoutes(s["routes"])
		if e != nil {
			return e
		}
		for _, r := range routes {
			h := str(obj(r)["host"])
			if hosts[h] {
				return fail("ROUTE_DUPLICATE", "Dominio duplicado: "+h, 409)
			}
			hosts[h] = true
		}
		return nil
	}
	activeStackUIDs := map[string]bool{}
	for _, raw := range arr(v["stacks"]) {
		s := obj(raw)
		if e := validateStack(s); e != nil {
			return e
		}
		activeStackUIDs[str(s["uid"])] = true
	}
	archivedBindings := A{}
	for _, raw := range arr(v["archivedStacks"]) {
		snapshot := obj(raw)
		for k := range snapshot {
			if !contains([]string{"lifecycle", "archivedAt", "stack", "bindings", "snapshotDigest"}, k) {
				return fail("STACK_ARCHIVE", "El snapshot de aplicación contiene campos desconocidos.", 409)
			}
		}
		if str(snapshot["lifecycle"]) != "archived" {
			return fail("STACK_ARCHIVE", "Estado archivado de aplicación inválido.", 409)
		}
		if _, e := time.Parse(time.RFC3339Nano, str(snapshot["archivedAt"])); e != nil {
			return fail("STACK_ARCHIVE", "Fecha de archivo de aplicación inválida.", 409)
		}
		if _, ok := snapshot["bindings"].([]any); !ok {
			return fail("STACK_ARCHIVE", "Las vinculaciones archivadas no son válidas.", 409)
		}
		stack := obj(snapshot["stack"])
		if e := validateStack(stack); e != nil {
			return e
		}
		for _, bindingRaw := range arr(snapshot["bindings"]) {
			if str(obj(bindingRaw)["stackUid"]) != str(stack["uid"]) {
				return fail("STACK_ARCHIVE", "Vinculación archivada asignada a otra aplicación.", 409)
			}
		}
		if str(snapshot["snapshotDigest"]) != archivedStackDigest(snapshot) {
			return fail("STACK_ARCHIVE_DIGEST", "El snapshot de aplicación no coincide con su digest.", 409)
		}
		archivedBindings = append(archivedBindings, arr(snapshot["bindings"])...)
	}
	for _, raw := range allInfraInstances(obj(v["infra"])) {
		project := str(obj(raw)["projectName"])
		if projects[project] {
			return fail("PROJECT_RESERVED", "Una aplicación y una instancia de infraestructura comparten identidad Compose.", 409)
		}
	}
	for name, p := range obj(v["toolPaths"]) {
		if !contains([]string{"docker", "colima"}, name) || !filepath.IsAbs(str(p)) || strings.ContainsAny(str(p), "\x00\r\n") {
			return fail("TOOL_PATH", "Ruta de herramienta inválida.", 409)
		}
	}
	if p := obj(v["proxy"]); len(p) > 0 {
		if _, e := intRange(p["port"], 1, 65535, "Puerto proxy"); e != nil {
			return e
		}
		if str(p["image"]) != TraefikImage {
			return fail("PROXY_IMAGE", "Imagen de proxy no reconocida; se preservó el catálogo.", 409)
		}
	}
	if e := validateInfra(obj(v["infra"])); e != nil {
		return e
	}
	activeDBIDs, bindingIDs := map[string]bool{}, map[string]bool{}
	for _, raw := range arr(at(v, "infra", "databases")) {
		activeDBIDs[str(obj(raw)["id"])] = true
	}
	for _, raw := range arr(at(v, "infra", "bindings")) {
		binding := obj(raw)
		if !activeStackUIDs[str(binding["stackUid"])] {
			return fail("INFRA_STATE", "Vinculación sin aplicación activa.", 409)
		}
		if e := validateInfraBinding(binding, activeDBIDs, bindingIDs); e != nil {
			return e
		}
	}
	for _, raw := range archivedBindings {
		binding := obj(raw)
		if !archivedStackUID(v, str(binding["stackUid"])) {
			return fail("STACK_ARCHIVE", "Vinculación archivada inválida o sin base activa.", 409)
		}
		if e := validateInfraBinding(binding, activeDBIDs, bindingIDs); e != nil {
			return e
		}
	}
	return nil
}

func MigrationPreview(home string) (J, error) {
	canonical := filepath.Join(home, "config", "catalog.json")
	legacy := filepath.Join(home, "catalog.json")
	report := J{"home": home, "catalog": canonical, "legacyCatalog": legacy, "requiresMigration": false, "schemaVersion": SchemaVersion, "movesDatabaseFiles": false, "note": "Se conserva owner, IDs, proyectos Compose, rutas, credenciales y volúmenes. No se ejecuta Docker."}
	if v, e := readJSON(canonical, 16<<20); e == nil {
		if err := validateState(v); err != nil {
			return nil, err
		}
		report["state"] = "current"
		report["projects"] = len(arr(v["stacks"]))
		return report, nil
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	if v, e := readJSON(legacy, 16<<20); e == nil {
		if num(v["version"]) != 3 {
			return nil, fail("SCHEMA_UNSUPPORTED", "El catálogo previo no es schema 3 o ya fue migrado. No se crea uno vacío.", 409)
		}
		migrated := copyJ(v)
		migrated["version"] = SchemaVersion
		defaults(migrated)
		if err := validateState(migrated); err != nil {
			return nil, err
		}
		report["state"] = "legacy"
		report["requiresMigration"] = true
		report["projects"] = len(arr(v["stacks"]))
		report["instances"] = len(arr(at(v, "infra", "instances")))
		report["sourceHash"] = hashFile(legacy)
		return report, nil
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	// Protect against a missing catalog in a home that still has data/secrets.
	for _, name := range []string{"infra", "proxy", "databases", "config/secrets", "generated", "config/resources", "config/generated"} {
		if entries, e := os.ReadDir(filepath.Join(home, name)); e == nil && len(entries) > 0 {
			return nil, fail("CATALOG_MISSING", "Hay recursos existentes pero falta el catálogo. Restaura la configuración; no se regenerarán identidades.", 409)
		}
	}
	report["state"] = "new"
	return report, nil
}
func hashFile(file string) string {
	b, e := os.ReadFile(file)
	if e != nil {
		return ""
	}
	return hash(string(b))
}
func defaults(v J) {
	for _, k := range []string{"groups", "operations", "roots", "stacks", "archivedStacks"} {
		if v[k] == nil {
			v[k] = A{}
		}
	}
	if v["toolPaths"] == nil {
		v["toolPaths"] = J{}
	}
	if v["infra"] == nil {
		v["infra"] = J{}
	}
	i := obj(v["infra"])
	for _, k := range []string{"instances", "databases", "archivedDatabases", "bindings", "archivedInstances"} {
		if i[k] == nil {
			i[k] = A{}
		}
	}
	v["infra"] = i
}

// Init must only be called after the root-level legacy-compatible agent.sock lock.
// The journal makes migration resumable after interruption. The old path becomes
// a fail-closed marker, preventing 0.6.x from creating a competing catalog.
func OpenStore(home string) (*Store, error) {
	if e := privateDir(home); e != nil {
		return nil, e
	}
	if e := privateDir(filepath.Join(home, "config")); e != nil {
		return nil, e
	}
	file := filepath.Join(home, "config", "catalog.json")
	legacy := filepath.Join(home, "catalog.json")
	journal := filepath.Join(home, "config", "migration.json")
	s := &Store{Home: home, File: file}
	v, e := readJSON(file, 16<<20)
	if e == nil {
		if e = validateState(v); e != nil {
			return nil, e
		}
		// Recover only a journaled migration whose copied target still matches.
		if j, je := readJSON(journal, 1<<20); je == nil && str(j["phase"]) != "committed" {
			if hashFile(file) != str(j["targetHash"]) {
				return nil, fail("MIGRATION_CONFLICT", "El catálogo cambió durante una migración incompleta. Conserva ambas copias y revisa el informe.", 409)
			}
			if old, oe := readJSON(legacy, 16<<20); oe == nil && integer(old["version"]) == 3 && hashFile(legacy) != str(j["sourceHash"]) {
				return nil, fail("MIGRATION_CONFLICT", "El catálogo antiguo cambió después del backup; no se sobrescribe.", 409)
			}
			if e = writeLegacyMarker(legacy, file, str(j["backup"])); e != nil {
				return nil, e
			}
			j["phase"] = "committed"
			if e = writeJSON(journal, j); e != nil {
				return nil, e
			}
		}
		if old, oe := readJSON(legacy, 16<<20); oe == nil && integer(old["version"]) == 3 {
			return nil, fail("DUAL_CATALOG", "Hay dos catálogos activos. No se elegirán ni combinarán silenciosamente.", 409)
		} else if oe != nil && !os.IsNotExist(oe) {
			return nil, oe
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	} else {
		preview, e := MigrationPreview(home)
		if e != nil {
			return nil, e
		}
		if truth(preview["requiresMigration"]) {
			data, e := os.ReadFile(legacy)
			if e != nil {
				return nil, e
			}
			v, e = decodeObject(data)
			if e != nil {
				return nil, e
			}
			sourceHash := hash(string(data))
			backup := filepath.Join(home, "backups", "config", strings.ReplaceAll(now(), ":", "-")+"-"+token(4), "catalog-v3.json")
			if e = atomicBytes(backup, data, 0600); e != nil {
				return nil, e
			}
			if hashFile(backup) != sourceHash {
				return nil, fail("BACKUP_VERIFY", "No coincide el backup; migración detenida.", 500)
			}
			v["version"] = SchemaVersion
			defaults(v)
			v["migration"] = J{"fromSchema": 3, "at": now(), "backup": backup, "sourceHash": sourceHash}
			// Existing trust values are retained, but v7 fingerprints trigger one re-review.
			if e = validateState(v); e != nil {
				return nil, e
			}
			target, _ := json.MarshalIndent(v, "", "  ")
			target = append(target, '\n')
			j := J{"phase": "prepared", "source": legacy, "sourceHash": sourceHash, "target": file, "targetHash": hash(string(target)), "backup": backup, "version": Version}
			if e = writeJSON(journal, j); e != nil {
				return nil, e
			}
			if hashFile(legacy) != sourceHash {
				return nil, fail("MIGRATION_CONFLICT", "El catálogo original cambió durante el backup.", 409)
			}
			if e = atomicBytes(file, target, 0600); e != nil {
				return nil, e
			}
			if e = writeLegacyMarker(legacy, file, backup); e != nil {
				return nil, e
			}
			j["phase"] = "committed"
			if e = writeJSON(journal, j); e != nil {
				return nil, e
			}
		} else {
			v = initialState()
			if e = writeJSON(file, v); e != nil {
				return nil, e
			}
			if e = writeLegacyMarker(legacy, file, ""); e != nil {
				return nil, e
			}
		}
	}
	defaults(v)
	s.value = v
	if e = s.Update(func(d J) error {
		for _, raw := range arr(d["operations"]) {
			op := obj(raw)
			if contains([]string{"running", "queued"}, str(op["state"])) {
				op["state"] = "interrupted"
				op["error"] = J{"code": "AGENT_RESTARTED", "message": "El agente se reinició. Docker puede seguir trabajando; revisa el estado real."}
				op["endedAt"] = now()
			}
		}
		return nil
	}); e != nil {
		return nil, e
	}
	return s, nil
}
func writeLegacyMarker(legacy, canonical, backup string) error {
	return writeJSON(legacy, J{"version": -1, "migratedTo": canonical, "backup": backup, "message": "NearProd 0.7: catálogo canónico en config/catalog.json. No iniciar 0.6 con este HOME ni borrar este marcador."})
}
func (s *Store) Get() J { s.mu.RLock(); defer s.mu.RUnlock(); return copyJ(s.value) }
func (s *Store) Update(fn func(J) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := copyJ(s.value)
	if e := fn(next); e != nil {
		return e
	}
	if e := validateState(next); e != nil {
		return e
	}
	if e := writeJSON(s.File, next); e != nil {
		return e
	}
	s.value = copyJ(next)
	return nil
}
func (s *Store) Stack(id string) (J, error) {
	for _, raw := range arr(s.Get()["stacks"]) {
		if str(obj(raw)["id"]) == id {
			return obj(raw), nil
		}
	}
	return nil, fail("STACK_NOT_FOUND", "Aplicación desconocida: "+id, 404)
}
func (s *Store) StackUID(uid string) (J, error) {
	for _, raw := range arr(s.Get()["stacks"]) {
		if str(obj(raw)["uid"]) == uid {
			return obj(raw), nil
		}
	}
	return nil, fail("STACK_NOT_FOUND", "Aplicación ya no registrada.", 404)
}
func editStack(d J, id string, fn func(J) error) error {
	for _, raw := range arr(d["stacks"]) {
		if str(obj(raw)["id"]) == id {
			return fn(obj(raw))
		}
	}
	return fail("STACK_NOT_FOUND", "Aplicación no encontrada.", 404)
}
func configPaths(home string) J {
	return J{"home": home, "catalog": filepath.Join(home, "config", "catalog.json"), "defaultDatabaseRoot": filepath.Join(home, "databases"), "configurationBackups": filepath.Join(home, "backups", "config"), "release": Version, "schemaVersion": SchemaVersion, "note": "Actualizar el binario no borra ni reubica configuración o datos. Volúmenes Docker y carpetas ya registradas conservan su ubicación."}
}
func rawStateSummary(v J) string {
	return fmt.Sprintf("%d proyectos, %d instancias", len(arr(v["stacks"])), len(arr(at(v, "infra", "instances"))))
}
