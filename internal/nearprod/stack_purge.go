package nearprod

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Purges keep their reviewed identities until every physical deletion is verified.
// The checkout is never part of the deletion plan. No global Docker prune is used.
func purgeStack(state J, target string) (J, error) {
	for _, raw := range allStacks(state) {
		stack := obj(raw)
		if str(stack["id"]) == target {
			return stack, nil
		}
	}
	return nil, fail("STACK_NOT_FOUND", "Aplicación no encontrada.", 404)
}

func stackPurgePending(state J, uid string) bool { return at(state, "stackPurges", uid) != nil }

func stackPurgeBindings(state J, uid string) A {
	out := A{}
	for _, raw := range arr(at(state, "infra", "bindings")) {
		if str(at(raw, "stackUid")) == uid {
			out = append(out, raw)
		}
	}
	for _, raw := range arr(state["archivedStacks"]) {
		if str(at(raw, "stack", "uid")) == uid {
			out = append(out, arr(at(raw, "bindings"))...)
		}
	}
	return out
}

func detachPurgeBindings(state J, uid string) {
	remaining := A{}
	for _, raw := range arr(at(state, "infra", "bindings")) {
		if str(at(raw, "stackUid")) != uid {
			remaining = append(remaining, raw)
		}
	}
	obj(state["infra"])["bindings"] = remaining
	for _, raw := range arr(state["archivedStacks"]) {
		snapshot := obj(raw)
		if str(at(snapshot, "stack", "uid")) == uid {
			snapshot["bindings"] = A{}
			snapshot["snapshotDigest"] = archivedStackDigest(snapshot)
		}
	}
}

func purgeResource(kind, id, name string, identity any) J {
	return J{"kind": kind, "id": id, "name": name, "digest": hash(identity)}
}

func purgeDockerIdentity(kind string, value J) J {
	if kind == "container" {
		// Docker does not guarantee mount ordering between consecutive inspections.
		// Hash every mount property, but normalize this unordered collection first.
		mounts := value["Mounts"]
		if len(arr(mounts)) > 1 {
			ordered := list(mounts)
			sort.Slice(ordered, func(a, b int) bool { return hash(ordered[a]) < hash(ordered[b]) })
			mounts = ordered
		}
		return J{"id": value["Id"], "created": value["Created"], "labels": at(value, "Config", "Labels"), "image": value["Image"], "mounts": mounts}
	}
	labels := value["Labels"]
	if kind == "image" {
		labels = at(value, "Config", "Labels")
	}
	tags := value["RepoTags"]
	if len(ss(tags)) > 1 {
		tags = sortedStrings(ss(tags))
	}
	return J{"id": value["Id"], "name": value["Name"], "created": value["Created"], "createdAt": value["CreatedAt"], "labels": labels, "driver": value["Driver"], "options": value["Options"], "tags": tags}
}

// Generated overlays can contain credentials. Only their digest is exposed.
func purgeFileIdentity(file string) (J, error) {
	info, e := os.Lstat(file)
	if os.IsNotExist(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	if !info.Mode().IsRegular() {
		return nil, fail("PURGE_FILE", "El archivo generado no es un archivo regular: "+file, 409)
	}
	parent, e := filepath.EvalSymlinks(filepath.Dir(file))
	if e != nil {
		return nil, e
	}
	if parent != filepath.Clean(filepath.Dir(file)) {
		return nil, fail("PURGE_FILE", "La carpeta generada contiene enlaces simbólicos.", 409)
	}
	data, e := readLimited(file, 4<<20)
	if e != nil {
		return nil, e
	}
	return J{"content": hash(string(data))}, nil
}

func (s *Service) purgeModelResources(ctx context.Context, stack J) (J, error) {
	// Resolution validates paths and Compose syntax but does not run lifecycle hooks.
	compose := *s.Compose
	compose.Infra = nil
	external := J{"network": J{}, "volume": J{}}
	for _, mode := range keys(obj(stack["modes"])) {
		r, e := compose.Resolve(ctx, stack, mode)
		if e != nil {
			return nil, e
		}
		for _, kind := range []string{"network", "volume"} {
			for key, raw := range obj(r.Model[kind+"s"]) {
				value := obj(raw)
				if truth(value["external"]) {
					obj(external[kind])[text(value["name"], key)] = true
				}
			}
		}
	}
	return external, nil
}

func (s *Service) stackPurgeInventory(ctx context.Context, stack J, external J) (A, A, A, error) {
	resources, preserved, blockers := A{}, A{}, A{}
	state := s.Store.Get()
	owner, uid, project := str(state["owner"]), str(stack["uid"]), str(stack["projectName"])
	containers, e := s.Docker.Containers(ctx, true)
	if e != nil {
		return nil, nil, nil, e
	}
	candidates := map[string]map[string]bool{"network": {}, "volume": {}, "image": {}}
	anonymous := map[string]bool{}
	for _, raw := range containers {
		c := obj(raw)
		if str(c["project"]) != project && str(at(c, "labels", LStack)) != uid {
			continue
		}
		if !ownsContainer(stack, c, owner) {
			blockers = append(blockers, "Contenedor ajeno con la identidad de esta aplicación: "+str(c["name"]))
			continue
		}
		v, err := s.Docker.InspectNamed(ctx, "container", str(c["id"]))
		if err != nil {
			return nil, nil, nil, err
		}
		if v == nil {
			return nil, nil, nil, fail("PREVIEW_CHANGED", "Un contenedor cambió durante la revisión.", 409)
		}
		resources = append(resources, purgeResource("container", str(c["id"]), str(c["name"]), purgeDockerIdentity("container", v)))
		for name := range obj(c["networks"]) {
			candidates["network"][name] = true
		}
		candidates["image"][str(c["imageId"])] = true
		for _, mount := range arr(v["Mounts"]) {
			m := obj(mount)
			switch str(m["Type"]) {
			case "volume":
				name := str(m["Name"])
				candidates["volume"][name] = true
				// Docker-generated anonymous names are recovered before removing containers.
				if cidRE.MatchString(name) && len(name) == 64 {
					anonymous[name] = true
				}
			case "bind":
				source := str(m["Source"])
				if !truth(m["RW"]) || source == str(stack["path"]) || strings.HasPrefix(source, str(stack["path"])+string(os.PathSeparator)) {
					preserved = append(preserved, J{"kind": "bind", "name": source, "reason": "Archivo o carpeta del checkout/origen de solo lectura; se conserva el código."})
				} else {
					blockers = append(blockers, "Carpeta de datos externa sin propiedad verificable: "+source+". Migra o retira sus datos antes de eliminar la aplicación.")
				}
			}
		}
	}
	for _, kind := range []string{"network", "volume", "image"} {
		for _, label := range []string{LProject + "=" + project, LStack + "=" + uid} {
			args := []string{kind, "ls", "--quiet", "--filter", "label=" + label}
			if kind == "image" {
				args = append(args, "--all", "--no-trunc")
			}
			res, err := s.Docker.Call(ctx, args, RunOptions{Exact: true, Limit: 2 << 20})
			if err != nil {
				return nil, nil, nil, err
			}
			for _, id := range strings.Fields(res.Stdout) {
				candidates[kind][id] = true
			}
		}
		seen := map[string]bool{}
		for _, id := range sortedPurgeKeys(candidates[kind]) {
			if id == "" {
				continue
			}
			v, err := s.Docker.InspectNamed(ctx, kind, id)
			if err != nil {
				return nil, nil, nil, err
			}
			if v == nil {
				continue
			}
			name := text(v["Name"], id)
			identity := text(v["Id"], name)
			if kind == "volume" {
				identity = name
			}
			if seen[identity] {
				continue
			}
			seen[identity] = true
			labels := obj(v["Labels"])
			if kind == "image" {
				labels = obj(at(v, "Config", "Labels"))
			}
			owned := str(labels[LOwner]) == owner && str(labels[LStack]) == uid
			legacy := str(labels[LProject]) == project && str(labels[LOwner]) == "" && str(labels[LStack]) == ""
			if truth(at(external, kind, name)) {
				preserved = append(preserved, J{"kind": kind, "name": name, "reason": "Recurso declarado externo; no fue creado para esta aplicación."})
				continue
			}
			anonymousLabels := len(labels) == 0 || len(labels) == 1 && labels["com.docker.volume.anonymous"] != nil
			if !owned && !legacy && !(kind == "volume" && anonymous[name] && anonymousLabels) {
				if kind == "volume" {
					blockers = append(blockers, "No se puede atribuir el volumen a la aplicación: "+name)
				} else {
					preserved = append(preserved, J{"kind": kind, "name": name, "reason": "Infraestructura o imagen externa; sin propiedad de la aplicación."})
				}
				continue
			}
			for _, other := range containers {
				c := obj(other)
				if ownsContainer(stack, c, owner) {
					continue
				}
				shared := kind == "network" && at(c, "networks", name) != nil || kind == "image" && str(c["imageId"]) == identity
				if kind == "volume" {
					for _, mount := range arr(c["mounts"]) {
						shared = shared || str(at(mount, "name")) == name
					}
				}
				if shared {
					blockers = append(blockers, kind+" "+name+" también lo usa "+str(c["name"])+"; separa ese recurso antes de eliminarlo.")
				}
			}
			// Driver-backed volumes can point at host/NFS data that volume rm does not erase.
			if kind == "volume" && (str(v["Driver"]) != "local" || len(obj(v["Options"])) > 0) {
				blockers = append(blockers, "El volumen "+name+" usa almacenamiento externo; no se puede verificar la eliminación de sus datos.")
			}
			resources = append(resources, purgeResource(kind, identity, name, purgeDockerIdentity(kind, v)))
		}
	}
	sortPurgeResources(resources)
	return resources, preserved, blockers, nil
}

func sortedPurgeKeys(v map[string]bool) []string {
	out := []string{}
	for k := range v {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
func sortPurgeResources(v A) {
	sort.Slice(v, func(i, j int) bool {
		return str(at(v[i], "kind"))+str(at(v[i], "id")) < str(at(v[j], "kind"))+str(at(v[j], "id"))
	})
}

func (s *Service) PurgeStackPreview(ctx context.Context, target string) (J, error) {
	state := s.Store.Get()
	stack, e := purgeStack(state, target)
	if e != nil {
		return nil, e
	}
	uid := str(stack["uid"])
	info, e := s.Docker.Info(ctx)
	if e != nil {
		return nil, e
	}
	if len(obj(stack["binding"])) > 0 {
		if e = verifyBinding(obj(stack["binding"]), info); e != nil {
			return nil, e
		}
	}
	journal := obj(at(state, "stackPurges", uid))
	external := obj(journal["external"])
	if len(journal) == 0 {
		external, e = s.purgeModelResources(ctx, stack)
		if e != nil {
			return nil, e
		}
	} else if e = verifyBinding(obj(journal["engine"]), info); e != nil {
		return nil, e
	}
	resources, preserved, blockers, e := s.stackPurgeInventory(ctx, stack, external)
	if e != nil {
		return nil, e
	}
	if len(resources) > 0 && len(obj(stack["binding"])) == 0 {
		blockers = append(blockers, "La aplicación no tiene vinculación verificada con este Engine. Vincula sus recursos antes de eliminarlos.")
	}
	bindings := stackPurgeBindings(state, uid)
	databaseIDs := map[string]bool{}
	for _, raw := range bindings {
		databaseIDs[str(at(raw, "databaseId"))] = true
	}
	if len(journal) > 0 {
		for _, raw := range arr(journal["resources"]) {
			if str(at(raw, "kind")) == "database" {
				databaseIDs[str(at(raw, "id"))] = true
			}
		}
	}
	// Only the target's bindings are removed from a private, read-only preview copy.
	previewState := copyJ(state)
	detachPurgeBindings(previewState, uid)
	infra := *s.Infra
	infra.Store = &Store{Home: s.Store.Home, value: previewState}
	for _, id := range sortedPurgeKeys(databaseIDs) {
		database, err := infra.Database(id)
		if err != nil {
			if len(journal) > 0 {
				continue
			}
			return nil, err
		}
		p, err := infra.PurgeDatabasePreview(ctx, J{"database": id, "mode": "purge"})
		if err != nil {
			blockers = append(blockers, id+": "+str(publicError(err)["message"]))
			continue
		}
		r := purgeResource("database", id, str(database["name"]), purgeDatabaseRecordDigest(database))
		r["database"] = copyJ(database)
		r["instance"] = database["instanceUid"]
		resources = append(resources, r)
		preserved = append(preserved, J{"kind": "instance", "name": at(p, "instance", "name"), "reason": "Instancia de infraestructura independiente; se eliminan únicamente la base, cuenta y credencial de esta aplicación."})
	}
	for _, directory := range []string{"config/generated", "generated"} {
		file := filepath.Join(s.Store.Home, directory, uid+".json")
		identity, err := purgeFileIdentity(file)
		if err != nil {
			return nil, err
		}
		if identity != nil {
			resources = append(resources, purgeResource("file", file, file, identity))
		}
	}
	if len(journal) > 0 {
		known := map[string]J{}
		for _, raw := range arr(journal["resources"]) {
			r := obj(raw)
			known[str(r["kind"])+":"+str(r["id"])] = r
		}
		for _, raw := range resources {
			r := obj(raw)
			previous := known[str(r["kind"])+":"+str(r["id"])]
			if previous == nil || str(previous["digest"]) != str(r["digest"]) {
				blockers = append(blockers, "Un recurso nuevo o reemplazado impide continuar: "+str(r["name"]))
			}
		}
		// Retain anonymous volumes and other identities even after containers disappear.
		resources = list(journal["resources"])
	}
	sortPurgeResources(resources)
	// Inventory presentation is a set, independent of Docker inspection ordering.
	sort.Slice(preserved, func(a, b int) bool { return hash(preserved[a]) < hash(preserved[b]) })
	sort.Slice(blockers, func(a, b int) bool { return str(blockers[a]) < str(blockers[b]) })
	preview := J{"lifecycleAction": "purge-stack", "stack": stackSummary(stack), "resources": resources, "preserved": preserved, "blockers": blockers, "external": external, "engine": J{"engineId": info["id"], "endpoint": info["endpoint"]}, "recordDigest": hash(J{"stack": stack, "bindings": bindings}), "retry": len(journal) > 0, "irreversible": true, "note": "Se detendrán los contenedores y se borrarán los recursos y datos listados, las vinculaciones, rutas, confianza y registro de la aplicación. El checkout se conserva. No se purgan cachés globales ni backups independientes."}
	preview["fingerprint"] = hash(preview)
	return preview, nil
}

func (s *Service) PurgeStack(req J) (J, error) {
	if !truth(req["confirm"]) || !truth(req["acknowledgeDataLoss"]) || str(req["typedId"]) != str(req["target"]) {
		return nil, fail("PURGE_CONFIRMATION", "Escribe el ID exacto y confirma la pérdida irreversible de datos.", 409)
	}
	req = copyJ(req)
	s.mutation.Lock()
	defer s.mutation.Unlock()
	return s.Ops.Submit("purge-stack", []string{str(req["target"])}, true, false, func(ctx context.Context, line func(string, string)) (any, error) {
		s.mutation.Lock()
		defer s.mutation.Unlock()
		defer func() { s.emitCatalog(); go s.Refresh(s.ctx) }()
		p, e := s.PurgeStackPreview(ctx, str(req["target"]))
		if e != nil {
			return nil, e
		}
		if str(p["fingerprint"]) != str(req["fingerprint"]) {
			return nil, fail("PREVIEW_CHANGED", "Cambió la aplicación o sus recursos. Vuelve a revisar la eliminación.", 409)
		}
		if len(arr(p["blockers"])) > 0 {
			return nil, detailed("PURGE_BLOCKED", "Resuelve los bloqueos antes de eliminar la aplicación.", 409, J{"blockers": p["blockers"]})
		}
		uid := str(at(p, "stack", "uid"))
		s.StopWatch(str(req["target"]))
		e = s.Store.Update(func(state J) error {
			stack, err := purgeStack(state, str(req["target"]))
			if err != nil {
				return err
			}
			if hash(J{"stack": stack, "bindings": stackPurgeBindings(state, uid)}) != str(p["recordDigest"]) {
				return fail("PREVIEW_CHANGED", "Cambió la metadata de la aplicación.", 409)
			}
			if state["stackPurges"] == nil {
				state["stackPurges"] = J{}
			}
			if !stackPurgePending(state, uid) {
				obj(state["stackPurges"])[uid] = copyJ(p)
			}
			detachPurgeBindings(state, uid)
			delete(stack, "proxyApplied")
			for _, raw := range arr(state["archivedStacks"]) {
				if str(at(raw, "stack", "uid")) == uid {
					obj(raw)["snapshotDigest"] = archivedStackDigest(obj(raw))
				}
			}
			return nil
		})
		if e != nil {
			return nil, e
		}
		if e = s.Proxy.SyncRoutes(); e != nil {
			return nil, e
		}
		// Stop/remove consumers first, then purge data and storage, then generated files.
		for _, kind := range []string{"container", "database", "volume", "network", "image", "file"} {
			for _, raw := range arr(p["resources"]) {
				r := obj(raw)
				if str(r["kind"]) != kind {
					continue
				}
				line("Eliminando "+kind+": "+str(r["name"]), "stdout")
				if e = s.deleteStackResource(ctx, p, r); e != nil {
					return nil, detailed("PURGE_PARTIAL", "Eliminación incompleta en "+kind+" "+str(r["name"])+": "+str(publicError(e)["message"])+". El plan se conserva para reintentar.", 422, J{"resource": r["name"], "cause": publicError(e)})
				}
			}
		}
		// Fresh Docker inventory must contain no newly created owned resources.
		stack, e := purgeStack(s.Store.Get(), str(req["target"]))
		if e != nil {
			return nil, e
		}
		remaining, _, blockers, e := s.stackPurgeInventory(ctx, stack, obj(p["external"]))
		if e != nil {
			return nil, e
		}
		if len(remaining) > 0 || len(blockers) > 0 {
			return nil, fail("PURGE_REMAINS", "Quedan recursos de la aplicación o cambió su propiedad. El registro se conserva.", 409)
		}
		for _, raw := range arr(p["resources"]) {
			if e = s.verifyStackResourceAbsent(ctx, obj(raw)); e != nil {
				return nil, e
			}
		}
		info, e := s.Docker.Info(ctx)
		if e != nil {
			return nil, e
		}
		if e = verifyBinding(obj(p["engine"]), info); e != nil {
			return nil, e
		}
		e = s.Store.Update(func(state J) error {
			if len(stackPurgeBindings(state, uid)) > 0 {
				return fail("PURGE_REMAINS", "Aparecieron nuevas vinculaciones.", 409)
			}
			for _, key := range []string{"stacks", "archivedStacks"} {
				remaining := A{}
				for _, raw := range arr(state[key]) {
					candidate := obj(raw)
					if key == "archivedStacks" {
						candidate = obj(candidate["stack"])
					}
					if str(candidate["uid"]) != uid {
						remaining = append(remaining, raw)
					}
				}
				state[key] = remaining
			}
			delete(obj(state["stackPurges"]), uid)
			return nil
		})
		if e != nil {
			return nil, e
		}
		line("Recursos y datos eliminados; checkout conservado.", "stdout")
		return J{"purged": true, "checkoutPreserved": true, "verified": true}, nil
	})
}

func (s *Service) deleteStackResource(ctx context.Context, plan, resource J) error {
	info, e := s.Docker.Info(ctx)
	if e != nil {
		return e
	}
	if e = verifyBinding(obj(plan["engine"]), info); e != nil {
		return e
	}
	kind, id := str(resource["kind"]), str(resource["id"])
	if kind == "database" {
		db, e := s.Infra.Database(id)
		if e != nil {
			return s.verifyStackResourceAbsent(ctx, resource)
		}
		if hash(purgeDatabaseRecordDigest(db)) != str(resource["digest"]) {
			return fail("PURGE_IDENTITY", "Cambió la identidad de la base.", 409)
		}
		p, e := s.Infra.PurgeDatabasePreview(ctx, J{"database": id, "mode": "purge"})
		if e != nil {
			return e
		}
		_, e = s.Infra.PurgeDatabase(ctx, J{"database": id, "mode": "purge", "confirm": true, "acknowledgeDataLoss": true, "typedId": id, "fingerprint": p["fingerprint"]})
		return e
	}
	if kind == "file" {
		value, e := purgeFileIdentity(id)
		if e != nil {
			return e
		}
		if value == nil {
			return nil
		}
		if hash(value) != str(resource["digest"]) {
			return fail("PURGE_IDENTITY", "Cambió el archivo generado.", 409)
		}
		return os.Remove(id)
	}
	v, e := s.Docker.InspectNamed(ctx, kind, id)
	if e != nil {
		return e
	}
	if v == nil {
		return nil
	}
	if hash(purgeDockerIdentity(kind, v)) != str(resource["digest"]) {
		return fail("PURGE_IDENTITY", "Un recurso fue reemplazado o cambió: "+str(resource["name"]), 409)
	}
	args := []string{kind, "rm", id}
	if kind == "container" {
		if truth(at(v, "State", "Paused")) {
			if _, e = s.Docker.Call(ctx, []string{"unpause", id}, RunOptions{}); e != nil {
				return e
			}
		}
		if truth(at(v, "State", "Running")) {
			if _, e = s.Docker.Call(ctx, []string{"stop", "--time", "20", id}, RunOptions{}); e != nil {
				return e
			}
		}
		args = []string{"rm", id}
	}
	if kind == "image" {
		args = []string{"image", "rm", "--no-prune", id}
	}
	_, e = s.Docker.Call(ctx, args, RunOptions{})
	if e != nil {
		return e
	}
	return s.verifyStackResourceAbsent(ctx, resource)
}

func (s *Service) verifyStackResourceAbsent(ctx context.Context, resource J) error {
	kind, id := str(resource["kind"]), str(resource["id"])
	if kind == "database" {
		if _, e := s.Infra.Database(id); e == nil {
			return fail("PURGE_REMAINS", "Queda la base registrada: "+id, 409)
		}
		instance, e := s.Infra.Instance(str(resource["instance"]))
		if e != nil {
			return e
		}
		if _, e = s.Infra.verifyPurgePhysical(ctx, instance, obj(resource["database"]), "metadata"); e != nil {
			return e
		}
		vault, e := s.Infra.Secrets(instance)
		if e != nil {
			return e
		}
		if at(vault, "databases", id) != nil {
			return fail("PURGE_REMAINS", "Queda una credencial de la aplicación.", 409)
		}
		return nil
	}
	if kind == "file" {
		if _, e := os.Lstat(id); os.IsNotExist(e) {
			return nil
		} else if e != nil {
			return e
		}
		return fail("PURGE_REMAINS", "Queda un archivo generado.", 409)
	}
	v, e := s.Docker.InspectNamed(ctx, kind, id)
	if e != nil {
		return e
	}
	if v != nil {
		return fail("PURGE_REMAINS", "Queda el recurso: "+str(resource["name"]), 409)
	}
	return nil
}
