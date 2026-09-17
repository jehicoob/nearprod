package nearprod

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var baseFiles = []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"}
var composeFileRE = regexp.MustCompile(`^(?:docker-)?compose(?:[._-][A-Za-z0-9_.-]+)?\.ya?ml$`)
var envFileRE = regexp.MustCompile(`^(?:\.env(?:[._-].*)?|[A-Za-z0-9_.-]+\.env(?:[._-].*)?)$`)
var ignoreDirs = map[string]bool{".git": true, "node_modules": true, ".venv": true, "venv": true, "vendor": true, "dist": true, "build": true, ".next": true, ".nuxt": true, "_build": true, "deps": true}

func Discover(ctx context.Context, root string, depth, max int, ignore []string) (J, error) {
	if depth < 0 || depth > 32 || max < 1 || max > 100000 {
		return nil, fail("SCAN_LIMIT", "Profundidad 0–32; máximo 100000 entradas.", 400)
	}
	ignored := map[string]bool{}
	for k, v := range ignoreDirs {
		ignored[k] = v
	}
	for _, s := range ignore {
		if strings.Contains(s, "/") || s == ".." {
			return nil, fail("IGNORE_INVALID", "Las exclusiones son nombres de carpeta.", 400)
		}
		ignored[s] = true
	}
	count := 0
	limited := false
	candidates := A{}
	warnings := A{}
	var walk func(string, int) error
	walk = func(dir string, level int) error {
		if e := ctx.Err(); e != nil {
			return fail("CANCELLED", "Búsqueda cancelada.", 409)
		}
		entries, e := os.ReadDir(dir)
		if e != nil {
			warnings = append(warnings, "No se pudo leer "+dir)
			return nil
		}
		files, bases := []string{}, []string{}
		for _, entry := range entries {
			count++
			if count > max {
				limited = true
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			if entry.Type().IsRegular() && composeFileRE.MatchString(entry.Name()) {
				files = append(files, entry.Name())
				if contains(baseFiles, entry.Name()) {
					bases = append(bases, entry.Name())
				}
			}
		}
		if len(files) > 0 {
			sort.Strings(files)
			sort.Strings(bases)
			rel, _ := filepath.Rel(root, dir)
			parts := strings.Split(rel, string(os.PathSeparator))
			product := slug(filepath.Base(dir))
			if rel != "." {
				product = slug(parts[0])
			}
			suggested := A{}
			if len(bases) == 1 {
				suggested = append(suggested, bases[0])
			}
			candidates = append(candidates, J{"path": dir, "relative": rel, "product": product, "slug": slug(filepath.Base(dir)), "projectName": slug(filepath.Base(dir)), "bases": cloneStrings(bases), "files": cloneStrings(files), "suggested": suggested, "ambiguous": len(bases) != 1})
		}
		if level >= depth {
			return nil
		}
		for _, entry := range entries {
			if entry.IsDir() && !ignored[entry.Name()] && entry.Type()&os.ModeSymlink == 0 {
				if e := walk(filepath.Join(dir, entry.Name()), level+1); e != nil {
					return e
				}
				if limited {
					return nil
				}
			}
		}
		return nil
	}
	if e := walk(root, 0); e != nil {
		return nil, e
	}
	return J{"root": root, "candidates": candidates, "visited": count, "truncated": limited, "warnings": warnings, "note": "Descubrimiento de archivos solamente: no se ejecutó Compose, Docker ni scripts."}, nil
}

// This is metadata extraction, NOT a Compose/YAML parser. All actual execution
// and trust checks use Docker Compose's JSON model. Ambiguous offline hints are
// labelled as such instead of making up a service graph.
func yamlHints(data []byte) J {
	if v, e := decodeObject(data); e == nil {
		return v
	}
	out := J{"services": J{}}
	services := obj(out["services"])
	section := ""
	current := ""
	serviceIndent := -1
	field := ""
	keyRE := regexp.MustCompile(`^([ \t]*)(?:["']?)([A-Za-z0-9_.-]+)["']?\s*:\s*(.*)$`)
	for _, line := range strings.Split(string(data), "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		match := keyRE.FindStringSubmatch(line)
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if len(match) > 0 {
			key, value := match[2], strings.TrimSpace(strings.SplitN(match[3], " #", 2)[0])
			if indent == 0 {
				section = key
				current = ""
				serviceIndent = -1
				if key == "name" {
					out["name"] = strings.Trim(value, "\"'")
				}
				continue
			}
			if section != "services" {
				continue
			}
			if serviceIndent < 0 {
				serviceIndent = indent
			}
			if indent == serviceIndent {
				current = key
				services[current] = J{}
				field = ""
				continue
			}
			if current == "" {
				continue
			}
			if indent > serviceIndent {
				field = key
				svc := obj(services[current])
				switch key {
				case "image", "command":
					svc[key] = strings.Trim(value, "\"'")
				case "profiles", "ports", "expose", "env_file":
					vals := A{}
					for _, p := range strings.Split(strings.Trim(value, "[]"), ",") {
						p = strings.Trim(strings.TrimSpace(p), "\"'")
						if p != "" {
							vals = append(vals, p)
						}
					}
					svc[key] = vals
				}
			}
		} else if current != "" && section == "services" && strings.HasPrefix(trim, "- ") && contains([]string{"profiles", "ports", "expose", "env_file"}, field) {
			svc := obj(services[current])
			svc[field] = append(arr(svc[field]), strings.Trim(strings.TrimSpace(strings.TrimPrefix(trim, "- ")), "\"'"))
		}
	}
	return out
}
func projectOptions(ctx context.Context, checkout string, roots []string, selected []string) (J, error) {
	p, e := canonical(checkout, roots, "directory")
	if e != nil {
		return nil, e
	}
	entries, e := os.ReadDir(p)
	if e != nil {
		return nil, e
	}
	composeFiles, envFiles := A{}, A{}
	bases := []string{}
	for _, ent := range entries {
		if !ent.Type().IsRegular() {
			continue
		}
		n := ent.Name()
		if composeFileRE.MatchString(n) {
			base := contains(baseFiles, n)
			composeFiles = append(composeFiles, J{"name": n, "path": filepath.Join(p, n), "base": base})
			if base {
				bases = append(bases, n)
			}
		}
		if envFileRE.MatchString(n) {
			template := false
			for _, suffix := range []string{".example", ".sample", ".template", ".dist"} {
				template = template || strings.HasSuffix(n, suffix)
			}
			envFiles = append(envFiles, J{"name": n, "path": filepath.Join(p, n), "template": template, "automatic": n == ".env"})
		}
	}
	suggested := []string{}
	if len(bases) == 1 {
		suggested = bases
	}
	if selected == nil {
		selected = suggested
	}
	services, defaultsNames, envs := []string{}, []string{}, []string{}
	profileMap := map[string][]string{}
	hints := A{}
	warnings := A{}
	project := slug(filepath.Base(p))
	source := "folder"
	for _, file := range selected {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		full := file
		if !filepath.IsAbs(full) {
			full = filepath.Join(p, file)
		}
		full, e = canonical(full, roots, "file")
		if e != nil {
			return nil, e
		}
		st, _ := os.Stat(full)
		if st.Size() > 2<<20 {
			return nil, fail("CONFIG_LIMIT", "Compose demasiado grande para inspección.", 413)
		}
		data, e := os.ReadFile(full)
		if e != nil {
			return nil, e
		}
		model := yamlHints(data)
		if v := str(model["name"]); validID(v) && !strings.Contains(v, "$") {
			project, source = v, "compose"
		}
		for _, name := range keys(obj(model["services"])) {
			svc := obj(at(model, "services", name))
			services = append(services, name)
			profiles := ss(svc["profiles"])
			if len(profiles) == 0 {
				defaultsNames = append(defaultsNames, name)
			}
			for _, profile := range profiles {
				profileMap[profile] = append(profileMap[profile], name)
			}
			envs = append(envs, ss(svc["env_file"])...)
			ports := []int{}
			for _, v := range append(list(svc["ports"]), arr(svc["expose"])...) {
				val := str(v)
				if j := obj(v); len(j) > 0 {
					val = str(j["target"])
				}
				parts := strings.Split(val, ":")
				portstr := strings.Split(parts[len(parts)-1], "/")[0]
				port := integer(portstr)
				if port > 0 && port <= 65535 {
					found := false
					for _, a := range ports {
						found = found || a == port
					}
					if !found {
						ports = append(ports, port)
					}
				}
			}
			var preferred any
			for _, port := range ports {
				if contains([]string{"80", "3000", "4000", "4200", "5173", "8000", "8080"}, fmtPort(port)) {
					preferred = port
					break
				}
			}
			if preferred == nil && len(ports) > 0 {
				preferred = ports[0]
			}
			if strings.Contains(name, "frontend") && preferred == nil {
				preferred = 5173
			}
			hints = append(hints, J{"service": name, "ports": ports, "suggestedPort": preferred, "note": "Sugerencia de lectura estática. Confirma el puerto interno donde escucha HTTP; Revisar usa el modelo efectivo de Compose."})
		}
	}
	profiles := A{}
	names := []string{}
	for k := range profileMap {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, p := range names {
		profiles = append(profiles, J{"name": p, "services": cloneStrings(unique(profileMap[p]))})
	}
	warnings = append(warnings, "La inspección offline es orientativa: anchors/flow/variables complejas se validan con Docker Compose en Revisar.")
	return J{"path": p, "composeFiles": composeFiles, "envFiles": envFiles, "profiles": profiles, "services": cloneStrings(unique(services)), "defaultServices": cloneStrings(unique(defaultsNames)), "serviceEnvFiles": cloneStrings(unique(envs)), "httpHints": hints, "suggestedFiles": cloneStrings(suggested), "projectName": project, "projectNameSource": source, "existingProjects": A{}, "warnings": warnings, "note": "Selecciona configuraciones de la misma aplicación, en orden. No se leen valores de .env ni se ejecuta Docker."}, nil
}
func fmtPort(i int) string { b, _ := json.Marshal(i); return string(b) }
