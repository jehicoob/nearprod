package nearprod

import (
	"context"
	"path/filepath"
	"strings"
)

func archivedStackPayload(v J) J {
	return J{"lifecycle": v["lifecycle"], "archivedAt": v["archivedAt"], "stack": copyJ(obj(v["stack"])), "bindings": list(v["bindings"])}
}

func archivedStackDigest(v J) string { return hash(archivedStackPayload(v)) }

func archivedStackUID(state J, uid string) bool {
	for _, raw := range arr(state["archivedStacks"]) {
		if str(at(raw, "stack", "uid")) == uid {
			return true
		}
	}
	return false
}

func allStacks(state J) A {
	out := append(A{}, arr(state["stacks"])...)
	for _, raw := range arr(state["archivedStacks"]) {
		if stack := obj(at(raw, "stack")); len(stack) > 0 {
			out = append(out, stack)
		}
	}
	return out
}

func (s *Service) archivedStack(id string) (J, error) {
	for _, raw := range arr(s.Store.Get()["archivedStacks"]) {
		snapshot := obj(raw)
		stack := obj(snapshot["stack"])
		if str(stack["id"]) == id || str(stack["uid"]) == id {
			return snapshot, nil
		}
	}
	return nil, fail("ARCHIVED_STACK_NOT_FOUND", "Aplicación archivada no encontrada.", 404)
}

func stackSummary(stack J) J {
	return J{"id": stack["id"], "uid": stack["uid"], "product": stack["product"], "slug": stack["slug"], "name": stack["name"], "path": stack["path"], "projectName": stack["projectName"], "routes": list(stack["routes"])}
}

func (s *Service) ArchiveStackPreview(id string) (J, error) {
	stack, e := s.Store.Stack(id)
	if e != nil {
		return nil, e
	}
	observed := s.Observed()
	if !truth(observed["connected"]) {
		return nil, fail("DOCKER_UNAVAILABLE", "Conecta el Engine para confirmar que la aplicación está detenida.", 503)
	}
	containers := A{}
	for _, raw := range arr(observed["containers"]) {
		container := obj(raw)
		if str(container["project"]) != str(stack["projectName"]) {
			continue
		}
		if !ownsContainer(stack, container, str(s.Store.Get()["owner"])) {
			return nil, fail("STACK_OWNERSHIP", "Hay contenedores ajenos con la identidad Compose de la aplicación.", 409)
		}
		if truth(container["running"]) || contains([]string{"restarting", "paused", "starting"}, str(container["state"])) {
			return nil, fail("STACK_RUNNING", "Detén la aplicación antes de archivarla para no dejar runtime fuera del catálogo.", 409)
		}
		containers = append(containers, J{"id": container["id"], "name": container["name"], "state": container["state"], "running": container["running"]})
	}
	bindings := A{}
	for _, raw := range arr(at(s.Store.Get(), "infra", "bindings")) {
		binding := obj(raw)
		if str(binding["stackUid"]) == str(stack["uid"]) {
			bindings = append(bindings, copyJ(binding))
		}
	}
	recordDigest := hash(J{"stack": stack, "bindings": bindings})
	preview := J{"lifecycleAction": "archive-stack", "stack": stackSummary(stack), "containers": containers, "bindingCount": len(bindings), "recordDigest": recordDigest, "preserved": A{"checkout y archivos Compose", "contenedores, redes y volúmenes", "rutas, confianza y configuración", "vinculaciones con bases"}, "note": "Solo se archivará metadata de NearProd. No se ejecutará Docker ni se borrarán datos."}
	preview["fingerprint"] = hash(preview)
	return preview, nil
}

func (s *Service) ArchiveStack(ctx context.Context, req J) (J, error) {
	if !truth(req["confirm"]) {
		return nil, fail("CONFIRM_REQUIRED", "Confirma archivar la aplicación sin borrar recursos.", 409)
	}
	if ctx.Err() != nil || s.ctx.Err() != nil {
		return nil, fail("REQUEST_CANCELLED", "La solicitud terminó antes de comprobar Docker; no se archivó la aplicación.", 408)
	}
	s.mutation.Lock()
	defer s.mutation.Unlock()
	if ctx.Err() != nil || s.ctx.Err() != nil {
		return nil, fail("REQUEST_CANCELLED", "La solicitud terminó antes de comprobar Docker; no se archivó la aplicación.", 408)
	}
	if e := s.Idle(str(req["target"])); e != nil {
		return nil, e
	}
	// Docker is outside the catalog lock, so observe it as late as possible before commit.
	s.Refresh(ctx)
	if ctx.Err() != nil || s.ctx.Err() != nil {
		return nil, fail("REQUEST_CANCELLED", "No se pudo completar una observación fresca de Docker; no se archivó la aplicación.", 408)
	}
	preview, e := s.ArchiveStackPreview(str(req["target"]))
	if e != nil {
		return nil, e
	}
	if str(preview["fingerprint"]) != str(req["fingerprint"]) {
		return nil, fail("PREVIEW_CHANGED", "La aplicación o su runtime cambió desde la revisión.", 409)
	}
	uid := str(at(preview, "stack", "uid"))
	if e = s.Idle(str(at(preview, "stack", "id"))); e != nil {
		return nil, e
	}
	s.StopWatch(str(at(preview, "stack", "id")))
	e = s.Store.Update(func(state J) error {
		var current J
		stacks := A{}
		for _, raw := range arr(state["stacks"]) {
			stack := obj(raw)
			if str(stack["uid"]) == uid {
				current = copyJ(stack)
			} else {
				stacks = append(stacks, stack)
			}
		}
		if current == nil {
			return fail("STACK_NOT_FOUND", "La aplicación ya no está activa.", 409)
		}
		bindings, remaining := A{}, A{}
		for _, raw := range arr(at(state, "infra", "bindings")) {
			binding := obj(raw)
			if str(binding["stackUid"]) == uid {
				bindings = append(bindings, copyJ(binding))
			} else {
				remaining = append(remaining, binding)
			}
		}
		if hash(J{"stack": current, "bindings": bindings}) != str(preview["recordDigest"]) {
			return fail("PREVIEW_CHANGED", "La metadata cambió desde la revisión.", 409)
		}
		snapshot := J{"lifecycle": "archived", "archivedAt": now(), "stack": current, "bindings": bindings}
		snapshot["snapshotDigest"] = archivedStackDigest(snapshot)
		state["stacks"] = stacks
		obj(state["infra"])["bindings"] = remaining
		state["archivedStacks"] = append(arr(state["archivedStacks"]), snapshot)
		return nil
	})
	if e != nil {
		return nil, e
	}
	s.emitCatalog()
	return J{"stack": at(preview, "stack", "id"), "archived": true, "runtimeChanged": false, "dataPreserved": true}, nil
}

func (s *Service) RestoreStackPreview(id string) (J, error) {
	snapshot, e := s.archivedStack(id)
	if e != nil {
		return nil, e
	}
	if archivedStackDigest(snapshot) != str(snapshot["snapshotDigest"]) {
		return nil, fail("STACK_ARCHIVE_DIGEST", "El snapshot de aplicación no coincide con su digest.", 409)
	}
	stack := obj(snapshot["stack"])
	candidate := s.Store.Get()
	remaining := A{}
	for _, raw := range arr(candidate["archivedStacks"]) {
		if str(at(raw, "stack", "uid")) != str(stack["uid"]) {
			remaining = append(remaining, raw)
		}
	}
	candidate["archivedStacks"] = remaining
	candidate["stacks"] = append(arr(candidate["stacks"]), copyJ(stack))
	obj(candidate["infra"])["bindings"] = append(arr(at(candidate, "infra", "bindings")), list(snapshot["bindings"])...)
	if e = validateState(candidate); e != nil {
		return nil, e
	}
	preview := J{"lifecycleAction": "restore-stack", "stack": stackSummary(stack), "bindingCount": len(arr(snapshot["bindings"])), "recordDigest": snapshot["snapshotDigest"], "note": "La aplicación y sus vinculaciones volverán al catálogo. No se iniciará ni detendrá runtime."}
	preview["fingerprint"] = hash(preview)
	return preview, nil
}

func (s *Service) RestoreStack(req J) (J, error) {
	if !truth(req["confirm"]) {
		return nil, fail("CONFIRM_REQUIRED", "Confirma restaurar la aplicación.", 409)
	}
	preview, e := s.RestoreStackPreview(str(req["target"]))
	if e != nil {
		return nil, e
	}
	if str(preview["fingerprint"]) != str(req["fingerprint"]) {
		return nil, fail("PREVIEW_CHANGED", "La aplicación archivada cambió desde la revisión.", 409)
	}
	uid := str(at(preview, "stack", "uid"))
	s.mutation.Lock()
	defer s.mutation.Unlock()
	if e = s.Idle(str(at(preview, "stack", "id"))); e != nil {
		return nil, e
	}
	e = s.Store.Update(func(state J) error {
		var snapshot J
		remaining := A{}
		for _, raw := range arr(state["archivedStacks"]) {
			value := obj(raw)
			if str(at(value, "stack", "uid")) == uid {
				snapshot = copyJ(value)
			} else {
				remaining = append(remaining, value)
			}
		}
		if snapshot == nil || archivedStackDigest(snapshot) != str(preview["recordDigest"]) {
			return fail("PREVIEW_CHANGED", "El snapshot cambió desde la revisión.", 409)
		}
		state["archivedStacks"] = remaining
		state["stacks"] = append(arr(state["stacks"]), copyJ(obj(snapshot["stack"])))
		obj(state["infra"])["bindings"] = append(arr(at(state, "infra", "bindings")), list(snapshot["bindings"])...)
		return nil
	})
	if e != nil {
		return nil, e
	}
	s.emitCatalog()
	return J{"stack": at(preview, "stack", "id"), "restored": true, "runtimeChanged": false, "dataPreserved": true}, nil
}

func (s *Service) DeleteGroupPreview(id string) (J, error) {
	var group J
	state := s.Store.Get()
	for _, raw := range arr(state["groups"]) {
		if str(obj(raw)["id"]) == id {
			group = obj(raw)
		}
	}
	if group == nil {
		return nil, fail("GROUP_NOT_FOUND", "Grupo no encontrado.", 404)
	}
	active, archived := 0, 0
	for _, raw := range arr(state["stacks"]) {
		if str(obj(raw)["product"]) == id {
			active++
		}
	}
	for _, raw := range arr(state["archivedStacks"]) {
		if str(at(raw, "stack", "product")) == id {
			archived++
		}
	}
	if active+archived > 0 {
		return nil, detailed("GROUP_NOT_EMPTY", "El grupo contiene aplicaciones activas o archivadas.", 409, J{"active": active, "archived": archived})
	}
	preview := J{"lifecycleAction": "delete-group", "group": copyJ(group), "activeApplications": active, "archivedApplications": archived, "note": "Solo se eliminará el grupo vacío del catálogo."}
	preview["fingerprint"] = hash(preview)
	return preview, nil
}

func (s *Service) DeleteGroup(req J) (J, error) {
	if !truth(req["confirm"]) {
		return nil, fail("CONFIRM_REQUIRED", "Confirma eliminar el grupo vacío.", 409)
	}
	preview, e := s.DeleteGroupPreview(str(req["id"]))
	if e != nil {
		return nil, e
	}
	if str(preview["fingerprint"]) != str(req["fingerprint"]) {
		return nil, fail("PREVIEW_CHANGED", "El grupo cambió desde la revisión.", 409)
	}
	s.mutation.Lock()
	defer s.mutation.Unlock()
	if e = s.Idle(); e != nil {
		return nil, e
	}
	e = s.Store.Update(func(state J) error {
		for _, raw := range allStacks(state) {
			if str(obj(raw)["product"]) == str(req["id"]) {
				return fail("GROUP_NOT_EMPTY", "El grupo recibió una aplicación; no se eliminó.", 409)
			}
		}
		groups := A{}
		for _, raw := range arr(state["groups"]) {
			if str(obj(raw)["id"]) != str(req["id"]) {
				groups = append(groups, raw)
			}
		}
		state["groups"] = groups
		return nil
	})
	if e != nil {
		return nil, e
	}
	s.emitCatalog()
	return J{"group": req["id"], "deleted": true}, nil
}

func rootSelector(state J, raw string) (string, error) {
	candidate, e := filepath.Abs(expandHome(raw))
	if e != nil || strings.ContainsAny(candidate, "\x00\r\n") {
		return "", fail("ROOT_INVALID", "Raíz no válida.", 400)
	}
	candidate = filepath.Clean(candidate)
	registered := ss(state["roots"])
	if contains(registered, candidate) {
		return candidate, nil
	}
	if resolved, resolveErr := filepath.EvalSymlinks(candidate); resolveErr == nil {
		candidate = filepath.Clean(resolved)
	}
	for _, root := range registered {
		if root == candidate {
			return root, nil
		}
	}
	return "", fail("ROOT_NOT_FOUND", "Raíz no registrada.", 404)
}

func (s *Service) RemoveRootPreview(raw string) (J, error) {
	state := s.Store.Get()
	root, e := rootSelector(state, raw)
	if e != nil {
		return nil, e
	}
	active, archived := 0, 0
	for _, stackRaw := range arr(state["stacks"]) {
		if within(root, str(obj(stackRaw)["path"])) {
			active++
		}
	}
	for _, stackRaw := range arr(state["archivedStacks"]) {
		if within(root, str(at(stackRaw, "stack", "path"))) {
			archived++
		}
	}
	if active+archived > 0 {
		return nil, detailed("ROOT_IN_USE", "La raíz contiene aplicaciones activas o archivadas.", 409, J{"active": active, "archived": archived})
	}
	preview := J{"lifecycleAction": "remove-root", "root": root, "activeApplications": active, "archivedApplications": archived, "note": "Solo se retirará la referencia del catálogo; no se borrará ninguna carpeta."}
	preview["fingerprint"] = hash(preview)
	return preview, nil
}

func (s *Service) RemoveRoot(req J) (J, error) {
	if !truth(req["confirm"]) {
		return nil, fail("CONFIRM_REQUIRED", "Confirma retirar la raíz sin borrar carpetas.", 409)
	}
	preview, e := s.RemoveRootPreview(str(req["root"]))
	if e != nil {
		return nil, e
	}
	if str(preview["fingerprint"]) != str(req["fingerprint"]) {
		return nil, fail("PREVIEW_CHANGED", "La raíz cambió desde la revisión.", 409)
	}
	s.mutation.Lock()
	defer s.mutation.Unlock()
	if e = s.Idle(); e != nil {
		return nil, e
	}
	e = s.Store.Update(func(state J) error {
		root, e := rootSelector(state, str(req["root"]))
		if e != nil {
			return e
		}
		for _, raw := range allStacks(state) {
			if within(root, str(obj(raw)["path"])) {
				return fail("ROOT_IN_USE", "La raíz recibió una aplicación; no se retiró.", 409)
			}
		}
		roots := A{}
		for _, value := range ss(state["roots"]) {
			if value != root {
				roots = append(roots, value)
			}
		}
		state["roots"] = roots
		return nil
	})
	if e != nil {
		return nil, e
	}
	s.emitCatalog()
	return J{"root": preview["root"], "removed": true, "filesystemChanged": false}, nil
}
