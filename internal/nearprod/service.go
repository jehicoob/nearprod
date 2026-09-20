package nearprod

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type watchEntry struct {
	Value  J
	Cancel context.CancelFunc
	Done   chan struct{}
}
type Service struct {
	Store      *Store
	Runner     Runner
	Docker     *Docker
	Compose    *Compose
	Proxy      *Proxy
	Infra      *Infrastructure
	Runtime    *Runtime
	Tools      *ToolsManager
	Ops        *Operations
	Bus        *Bus
	ctx        context.Context
	cancel     context.CancelFunc
	mutation   sync.Mutex
	observedMu sync.RWMutex
	observed   J
	refreshMu  sync.Mutex
	watchMu    sync.Mutex
	watches    map[string]*watchEntry
	wg         sync.WaitGroup
}

func NewService(parent context.Context, store *Store, runner Runner) *Service {
	ctx, cancel := context.WithCancel(parent)
	if runner == nil {
		runner = &ExecRunner{ToolPath: func(name string) string { return platformToolPath(store.Get(), runtime.GOOS, name) }}
	}
	bus := NewBus()
	docker := &Docker{Runner: runner, Store: store}
	compose := &Compose{Runner: runner, Store: store, Docker: docker}
	infra := &Infrastructure{Store: store, Docker: docker, Compose: compose, PortCheck: availablePort}
	compose.Infra = infra
	runtime := &Runtime{Runner: runner, Store: store, Docker: docker}
	s := &Service{Store: store, Runner: runner, Docker: docker, Compose: compose, Infra: infra, Runtime: runtime, Proxy: &Proxy{Store: store, Docker: docker, Compose: compose, PortCheck: availablePort}, Tools: &ToolsManager{Runner: runner, Store: store, Runtime: runtime}, Ops: NewOperations(ctx, store, bus), Bus: bus, ctx: ctx, cancel: cancel, observed: J{"connected": false, "checkedAt": nil, "containers": A{}, "error": J{"code": "NOT_CHECKED", "message": "Engine sin comprobar."}}, watches: map[string]*watchEntry{}}
	return s
}
func (s *Service) emitCatalog() { s.Bus.Send(J{"type": "catalog"}) }
func (s *Service) Idle(targets ...string) error {
	if s.Ops.Busy(targets) {
		return fail("OPERATION_CONFLICT", "Hay una operación incompatible activa.", 409)
	}
	return nil
}
func (s *Service) Catalog() J {
	d := s.Store.Get()
	groups := list(d["groups"])
	for _, v := range arr(d["stacks"]) {
		st := obj(v)
		found := false
		for _, g := range groups {
			found = found || str(obj(g)["id"]) == str(st["product"])
		}
		if !found {
			groups = append(groups, J{"id": st["product"], "name": st["product"]})
		}
	}
	ops := A{}
	for _, v := range arr(d["operations"]) {
		op, e := s.Ops.Get(str(obj(v)["id"]))
		if e == nil {
			ops = append(ops, op)
		} else {
			ops = append(ops, v)
		}
	}
	return J{"version": Version, "runtimeLanguage": "Go", "host": s.Runtime.Capabilities(), "roots": d["roots"], "runtime": d["runtime"], "proxy": s.Proxy.Settings(), "groups": groups, "stacks": d["stacks"], "operations": ops, "infrastructure": s.Infra.List(s.Observed()), "storage": configPaths(s.Store.Home)}
}
func (s *Service) Observed() J {
	s.observedMu.RLock()
	defer s.observedMu.RUnlock()
	return copyJ(s.observed)
}
func (s *Service) Status() J {
	obs := s.Observed()
	d := s.Store.Get()
	stacks := A{}
	for _, v := range arr(d["stacks"]) {
		st := obj(v)
		cs := A{}
		for _, v := range arr(obs["containers"]) {
			c := obj(v)
			if str(c["project"]) == str(st["projectName"]) {
				cs = append(cs, merge(c, J{"owned": ownsContainer(st, c, str(d["owner"]))}))
			}
		}
		state := stackState(cs, ss(st["expectedServices"]), truth(obs["connected"]))
		stacks = append(stacks, merge(state, J{"id": st["id"], "routes": s.Proxy.Routes(st, obs), "containers": cs, "watch": s.WatchStatus(str(st["id"]))}))
	}
	return merge(obs, J{"proxy": s.Proxy.Summary(obs), "infrastructure": s.Infra.List(obs), "stacks": stacks})
}
func (s *Service) Refresh(ctx context.Context) J {
	if s.ctx.Err() != nil || ctx.Err() != nil {
		return s.Status()
	}
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	if s.ctx.Err() != nil || ctx.Err() != nil {
		return s.Status()
	}
	v, e := s.Docker.Snapshot(ctx)
	if e != nil {
		v = J{"connected": false, "checkedAt": now(), "containers": A{}, "error": publicError(e)}
	}
	s.observedMu.Lock()
	s.observed = v
	s.observedMu.Unlock()
	result := s.Status()
	s.Bus.Send(J{"type": "status", "status": result})
	return result
}
func (s *Service) Monitor() {
	s.wg.Add(2)
	trigger := make(chan struct{}, 1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		s.Refresh(s.ctx)
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				s.Refresh(s.ctx)
			case <-trigger:
				timer := time.NewTimer(400 * time.Millisecond)
				select {
				case <-s.ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
				s.Refresh(s.ctx)
			}
		}
	}()
	go func() {
		defer s.wg.Done()
		for {
			s.runEvents(trigger)
			select {
			case <-s.ctx.Done():
				return
			case <-time.After(10 * time.Second):
			}
		}
	}()
}
func (s *Service) runEvents(trigger chan struct{}) {
	if _, e := s.Docker.Info(s.ctx); e != nil {
		return
	}
	_, _ = s.Docker.Call(s.ctx, []string{"events", "--filter", "type=container", "--format", "{{json .}}"}, RunOptions{Stream: true, Line: func(_ string, _ string) {
		select {
		case trigger <- struct{}{}:
		default:
		}
	}})
}
func (s *Service) AddRoot(raw string) (J, error) {
	root, e := canonical(raw, nil, "directory")
	if e != nil {
		return nil, e
	}
	if root == filepath.Dir(root) {
		return nil, fail("ROOT_TOO_BROAD", "No se admite todo el disco como raíz.", 400)
	}
	s.mutation.Lock()
	defer s.mutation.Unlock()
	if e = s.Idle(); e != nil {
		return nil, e
	}
	e = s.Store.Update(func(v J) error {
		if !contains(ss(v["roots"]), root) {
			v["roots"] = append(arr(v["roots"]), root)
		}
		return nil
	})
	s.emitCatalog()
	return J{"root": root}, e
}
func (s *Service) Discover(ctx context.Context, req J) (J, error) {
	root, e := canonical(str(req["root"]), ss(s.Store.Get()["roots"]), "directory")
	if e != nil {
		return nil, e
	}
	depth, max := 8, 20000
	if req["depth"] != nil {
		depth = integer(req["depth"])
	}
	if req["maxEntries"] != nil {
		max = integer(req["maxEntries"])
	}
	return Discover(ctx, root, depth, max, ss(req["ignore"]))
}
func (s *Service) ProjectOptions(req J) (J, error) {
	var files []string
	if req["files"] != nil {
		files = ss(req["files"])
	}
	v, e := projectOptions(s.ctx, str(req["path"]), ss(s.Store.Get()["roots"]), files)
	if e != nil {
		return nil, e
	}
	known := []string{}
	for _, raw := range arr(s.Observed()["containers"]) {
		c := obj(raw)
		if str(at(c, "labels", LWorking)) == str(v["path"]) {
			known = append(known, str(c["project"]))
		}
	}
	known = unique(known)
	registered := []string{}
	for _, st := range arr(s.Store.Get()["stacks"]) {
		if str(obj(st)["path"]) == str(v["path"]) {
			registered = append(registered, str(obj(st)["projectName"]))
		}
	}
	v["existingProjects"] = cloneStrings(known)
	if len(known) == 1 {
		v["projectName"] = known[0]
		v["projectNameSource"] = "existing"
	} else if len(registered) == 1 {
		v["projectName"] = registered[0]
		v["projectNameSource"] = "existing"
	}
	return v, nil
}
func (s *Service) Group(req J, rename bool) (J, error) {
	name, e := requireText(req["name"], "Nombre del grupo", 120)
	if e != nil {
		return nil, e
	}
	id := text(req["id"], slug(name))
	if !validID(id) {
		return nil, fail("GROUP_ID", "Identificador de grupo inválido.", 400)
	}
	s.mutation.Lock()
	defer s.mutation.Unlock()
	if e = s.Idle(); e != nil {
		return nil, e
	}
	g := J{"id": id, "name": name}
	e = s.Store.Update(func(d J) error {
		for _, raw := range arr(d["groups"]) {
			v := obj(raw)
			if str(v["id"]) == id {
				if !rename {
					return fail("GROUP_DUPLICATE", "El grupo ya existe; selecciónalo.", 409)
				}
				v["name"] = name
				return nil
			}
		}
		if rename {
			return fail("GROUP_NOT_FOUND", "Grupo no encontrado.", 404)
		}
		d["groups"] = append(arr(d["groups"]), g)
		return nil
	})
	s.emitCatalog()
	return g, e
}
func localLink(raw string) (string, error) {
	u, e := url.Parse(raw)
	if e != nil || u.User != nil || u.Fragment != "" || u.Scheme != "http" && u.Scheme != "https" {
		return "", fail("LINK_INVALID", "Solo enlaces HTTP/HTTPS locales sin credenciales.", 400)
	}
	host := u.Hostname()
	if !contains([]string{"localhost", "127.0.0.1", "::1"}, host) && !strings.HasSuffix(host, ".localhost") {
		return "", fail("LINK_LOCAL", "Usa un enlace local.", 400)
	}
	return u.String(), nil
}
func (s *Service) Normalize(input J) (J, error) {
	roots := ss(s.Store.Get()["roots"])
	path, e := canonical(str(input["path"]), roots, "directory")
	if e != nil {
		return nil, e
	}
	product, slugID, project := str(input["product"]), str(input["slug"]), str(input["projectName"])
	if !validID(product) || !validID(slugID) || !validID(project) {
		return nil, fail("STACK_IDENTIFIER", "Grupo, identificador y proyecto Compose deben usar minúsculas, guiones, underscore y números.", 400)
	}
	name, e := requireText(text(input["name"], slugID), "Nombre", 120)
	if e != nil {
		return nil, e
	}
	if project == str(proxyNames(str(s.Store.Get()["owner"]))["project"]) {
		return nil, fail("PROXY_PROJECT_RESERVED", "Nombre de proyecto reservado al proxy.", 409)
	}
	modes := J{}
	values := obj(input["modes"])
	if values["dev"] == nil {
		return nil, fail("MODE_REQUIRED", "Selecciona la configuración de desarrollo.", 400)
	}
	for mode, raw := range values {
		if mode != "dev" && mode != "verify" {
			return nil, fail("MODE_INVALID", "Modos permitidos: dev y verify.", 400)
		}
		v := obj(raw)
		files, envs, profiles := ss(v["files"]), ss(v["envFiles"]), ss(v["profiles"])
		if len(files) < 1 || len(files) > 16 || len(unique(files)) != len(files) || len(envs) > 16 || len(profiles) > 32 {
			return nil, fail("FILES_INVALID", "Usa 1–16 Compose distintos, hasta 16 env y 32 perfiles.", 400)
		}
		for _, p := range profiles {
			if p != "*" && !svcRE.MatchString(p) {
				return nil, fail("PROFILE_INVALID", "Perfil Compose inválido.", 400)
			}
		}
		resolved, environment := A{}, A{}
		for _, f := range files {
			c, e := canonical(resolvePath(path, f), roots, "file")
			if e != nil {
				return nil, e
			}
			resolved = append(resolved, c)
		}
		for _, f := range envs {
			c, e := canonical(resolvePath(path, f), roots, "file")
			if e != nil {
				return nil, e
			}
			environment = append(environment, c)
		}
		modes[mode] = J{"files": resolved, "envFiles": environment, "profiles": cloneStrings(profiles)}
	}
	links := A{}
	if len(arr(input["links"])) > 8 {
		return nil, fail("LINKS_LIMIT", "Hasta 8 enlaces.", 400)
	}
	for _, raw := range arr(input["links"]) {
		v := obj(raw)
		label, e := requireText(text(v["label"], "Abrir aplicación"), "Etiqueta", 80)
		if e != nil {
			return nil, e
		}
		u, e := localLink(str(v["url"]))
		if e != nil {
			return nil, e
		}
		links = append(links, J{"label": label, "url": u})
	}
	routes, e := normalizeRoutes(input["routes"])
	if e != nil {
		return nil, e
	}
	return J{"id": product + "/" + slugID, "product": product, "slug": slugID, "name": name, "uid": token(16), "path": path, "projectName": project, "modes": modes, "routes": routes, "links": links, "trust": J{}, "activeMode": nil, "expectedServices": A{}, "createdAt": now()}, nil
}
func (s *Service) Register(req J, batch bool) (J, error) {
	defs := A{req}
	if batch {
		defs = arr(req["definitions"])
	}
	if len(defs) < 1 || len(defs) > 32 {
		return nil, fail("BATCH_INVALID", "Selecciona entre 1 y 32 aplicaciones.", 400)
	}
	normalized := A{}
	ids := []string{}
	for _, v := range defs {
		n, e := s.Normalize(obj(v))
		if e != nil {
			return nil, e
		}
		normalized = append(normalized, n)
		ids = append(ids, str(n["id"]))
	}
	groupName := str(req["groupName"])
	if groupName != "" {
		if _, e := requireText(groupName, "Nombre del grupo", 120); e != nil {
			return nil, e
		}
	}
	s.mutation.Lock()
	defer s.mutation.Unlock()
	if e := s.Idle(ids...); e != nil {
		return nil, e
	}
	e := s.Store.Update(func(v J) error {
		if e := s.Idle(ids...); e != nil {
			return e
		}
		for _, raw := range normalized {
			n := obj(raw)
			for _, st := range arr(v["stacks"]) {
				x := obj(st)
				if str(x["id"]) == str(n["id"]) || str(x["projectName"]) == str(n["projectName"]) {
					return fail("STACK_DUPLICATE", "Ya existe la identidad/proyecto Compose. No se guardó parcialmente la selección.", 409)
				}
			}
			for _, ir := range arr(at(v, "infra", "instances")) {
				if str(obj(ir)["projectName"]) == str(n["projectName"]) {
					return fail("PROJECT_RESERVED", "Nombre reservado a infraestructura.", 409)
				}
			}
			v["stacks"] = append(arr(v["stacks"]), n)
			found := false
			for _, g := range arr(v["groups"]) {
				found = found || str(obj(g)["id"]) == str(n["product"])
			}
			if !found {
				v["groups"] = append(arr(v["groups"]), J{"id": n["product"], "name": text(groupName, str(n["product"]))})
			}
		}
		return nil
	})
	if e != nil {
		return nil, e
	}
	s.emitCatalog()
	if !batch {
		return obj(normalized[0]), nil
	}
	return J{"stacks": normalized, "note": "Guardado sin ejecutar contenedores ni modificar repositorios."}, nil
}
func (s *Service) Edit(id string, input J) (J, error) {
	old, e := s.Store.Stack(id)
	if e != nil {
		return nil, e
	}
	replacement, e := s.Normalize(merge(old, input))
	if e != nil {
		return nil, e
	}
	for _, k := range []string{"slug", "projectName", "path"} {
		if str(old[k]) != str(replacement[k]) {
			return nil, fail("IDENTITY_IMMUTABLE", "Se conserva carpeta, identificador y proyecto Compose. Puedes cambiar grupo/nombre/configuración.", 409)
		}
	}
	s.mutation.Lock()
	defer s.mutation.Unlock()
	if e = s.Idle(id, str(replacement["id"])); e != nil {
		return nil, e
	}
	s.StopWatch(id)
	e = s.Store.Update(func(v J) error {
		if e := s.Idle(id, str(replacement["id"])); e != nil {
			return e
		}
		currentChanged := false
		for _, st := range arr(v["stacks"]) {
			x := obj(st)
			if str(x["id"]) != id && str(x["id"]) == str(replacement["id"]) {
				return fail("STACK_DUPLICATE", "Identificador ya utilizado en ese grupo.", 409)
			}
			if str(x["id"]) == id {
				currentChanged = hash(x) != hash(old)
			}
		}
		if currentChanged {
			return fail("CATALOG_CHANGED", "Otra edición cambió la aplicación; recarga antes de guardar.", 409)
		}
		e := editStack(v, id, func(st J) error {
			changed := hash(A{st["routes"], st["modes"]}) != hash(A{replacement["routes"], replacement["modes"]})
			for _, k := range []string{"id", "product", "name", "links", "routes", "modes"} {
				st[k] = replacement[k]
			}
			if changed {
				delete(st, "proxyApplied")
				st["trust"] = J{}
			}
			return nil
		})
		if e != nil {
			return e
		}
		found := false
		for _, g := range arr(v["groups"]) {
			found = found || str(obj(g)["id"]) == str(replacement["product"])
		}
		if !found {
			v["groups"] = append(arr(v["groups"]), J{"id": replacement["product"], "name": text(input["groupName"], str(replacement["product"]))})
		}
		return nil
	})
	if e != nil {
		return nil, e
	}
	if e = s.Proxy.SyncRoutes(); e != nil {
		return nil, e
	}
	s.emitCatalog()
	return s.Store.Stack(str(replacement["id"]))
}
func (s *Service) Remove(id string, confirm bool) (J, error) {
	if !confirm {
		return nil, fail("CONFIRM_REQUIRED", "Confirma quitar del catálogo; no borra recursos.", 409)
	}
	s.mutation.Lock()
	defer s.mutation.Unlock()
	if e := s.Idle(id); e != nil {
		return nil, e
	}
	old, e := s.Store.Stack(id)
	if e != nil {
		return nil, e
	}
	s.StopWatch(id)
	e = s.Store.Update(func(v J) error {
		if e := s.Idle(id); e != nil {
			return e
		}
		stacks, bindings := A{}, A{}
		for _, x := range arr(v["stacks"]) {
			if str(obj(x)["id"]) != id {
				stacks = append(stacks, x)
			}
		}
		for _, x := range arr(at(v, "infra", "bindings")) {
			if str(obj(x)["stackUid"]) != str(old["uid"]) {
				bindings = append(bindings, x)
			}
		}
		v["stacks"] = stacks
		obj(v["infra"])["bindings"] = bindings
		return nil
	})
	if e != nil {
		return nil, e
	}
	if e = s.Proxy.SyncRoutes(); e != nil {
		return nil, e
	}
	s.emitCatalog()
	return J{"removed": id, "note": "No se borraron contenedores, imágenes, redes, volúmenes ni carpetas."}, nil
}
func (s *Service) Preview(ctx context.Context, id, mode string) (J, error) {
	st, e := s.Store.Stack(id)
	if e != nil {
		return nil, e
	}
	r, e := s.Compose.Resolve(ctx, st, text(mode, "dev"))
	if e != nil {
		return nil, e
	}
	return r.Preview, nil
}
func (s *Service) Trust(ctx context.Context, id string, req J) (J, error) {
	mode := text(req["mode"], "dev")
	p, e := s.Preview(ctx, id, mode)
	if e != nil {
		return nil, e
	}
	if str(p["fingerprint"]) != str(req["fingerprint"]) {
		return nil, fail("PREVIEW_CHANGED", "El Compose cambió desde la revisión.", 409)
	}
	if len(arr(p["blockers"])) > 0 {
		return nil, detailed("VERIFY_BLOCKED", "Configuración bloqueada; revisa los motivos.", 422, J{"blockers": p["blockers"]})
	}
	if len(arr(p["risks"])) > 0 && !truth(req["allowUnsafe"]) {
		return nil, detailed("RISK_APPROVAL_REQUIRED", "Confirma las capacidades sensibles.", 409, J{"risks": p["risks"]})
	}
	s.mutation.Lock()
	defer s.mutation.Unlock()
	if e = s.Idle(id); e != nil {
		return nil, e
	}
	e = s.Store.Update(func(v J) error {
		if e := s.Idle(id); e != nil {
			return e
		}
		return editStack(v, id, func(st J) error {
			obj(st["trust"])[mode] = J{"fingerprint": p["fingerprint"], "allowUnsafe": truth(req["allowUnsafe"]), "reviewedAt": now()}
			return nil
		})
	})
	s.emitCatalog()
	return J{"approved": true, "id": id, "mode": mode}, e
}
func (s *Service) Adoption(ctx context.Context, id string) (J, error) {
	st, e := s.Store.Stack(id)
	if e != nil {
		return nil, e
	}
	info, e := s.Docker.Info(ctx)
	if e != nil {
		return nil, e
	}
	all, e := s.Docker.Containers(ctx, false)
	if e != nil {
		return nil, e
	}
	cs, ids := A{}, []string{}
	allowed := true
	for _, v := range all {
		c := obj(v)
		if str(c["project"]) == str(st["projectName"]) {
			cs = append(cs, c)
			ids = append(ids, str(c["id"]))
			owner := str(at(c, "labels", LOwner))
			allowed = allowed && str(at(c, "labels", LWorking)) == str(st["path"]) && (owner == "" || owner == str(s.Store.Get()["owner"]))
		}
	}
	return J{"id": id, "info": info, "containers": cs, "allowed": allowed, "fingerprint": hash(J{"endpoint": info["endpoint"], "engine": info["id"], "ids": sortedStrings(ids), "path": st["path"], "project": st["projectName"]}), "warning": "Adopta solo recursos que reconoces. Se conservan nombres y volúmenes; no basta la coincidencia de nombre."}, nil
}
func (s *Service) Adopt(ctx context.Context, id string, req J) (J, error) {
	if !truth(req["confirm"]) {
		return nil, fail("CONFIRM_REQUIRED", "Confirma adoptar los contenedores.", 409)
	}
	p, e := s.Adoption(ctx, id)
	if e != nil {
		return nil, e
	}
	if !truth(p["allowed"]) {
		return nil, fail("ADOPTION_MISMATCH", "La carpeta o propietario no coincide.", 409)
	}
	if str(p["fingerprint"]) != str(req["fingerprint"]) {
		return nil, fail("PREVIEW_CHANGED", "Los contenedores cambiaron.", 409)
	}
	ids := A{}
	for _, v := range arr(p["containers"]) {
		ids = append(ids, obj(v)["id"])
	}
	s.mutation.Lock()
	defer s.mutation.Unlock()
	if e = s.Idle(id); e != nil {
		return nil, e
	}
	e = s.Store.Update(func(v J) error {
		if e := s.Idle(id); e != nil {
			return e
		}
		return editStack(v, id, func(st J) error {
			st["binding"] = J{"endpoint": at(p, "info", "endpoint"), "engineId": at(p, "info", "id"), "adoptedIds": ids}
			return nil
		})
	})
	s.emitCatalog()
	return J{"id": id, "adopted": ids}, e
}
func (s *Service) Targets(target string) (A, error) {
	if target == "" {
		return nil, fail("TARGET_REQUIRED", "Selecciona una aplicación o grupo; no se actuará sobre todo implícitamente.", 400)
	}
	out := A{}
	for _, v := range arr(s.Store.Get()["stacks"]) {
		st := obj(v)
		if str(st["id"]) == target || str(st["product"]) == target {
			out = append(out, st)
		}
	}
	if len(out) == 0 {
		return nil, fail("TARGET_NOT_FOUND", "Aplicación/grupo no encontrado: "+target, 404)
	}
	return out, nil
}
func (s *Service) Action(target, action string, request J) (J, error) {
	if !contains([]string{"up", "stop", "restart", "rebuild"}, action) {
		return nil, fail("ACTION_INVALID", "Acción no válida.", 400)
	}
	selected, e := s.Targets(target)
	if e != nil {
		return nil, e
	}
	req := copyJ(request)
	svc := str(req["service"])
	if svc != "" && (!svcRE.MatchString(svc) || len(selected) != 1) {
		return nil, fail("SERVICE_TARGET", "Un servicio requiere una aplicación única y un nombre válido.", 400)
	}
	if m := str(req["mode"]); m != "" && m != "dev" && m != "verify" {
		return nil, fail("MODE_INVALID", "Modo no válido.", 400)
	}
	ids := []string{}
	for _, st := range selected {
		ids = append(ids, str(obj(st)["id"]))
	}
	s.mutation.Lock()
	defer s.mutation.Unlock()
	return s.Ops.Submit(action, ids, truth(req["startRuntime"]), action == "up" || action == "rebuild", func(ctx context.Context, line func(string, string)) (any, error) {
		defer func() { s.emitCatalog(); go s.Refresh(s.ctx) }()
		if truth(req["startRuntime"]) {
			if _, e := s.Docker.Info(ctx); e != nil {
				if _, e = s.Runtime.Start(ctx, line); e != nil {
					return nil, e
				}
			}
		}
		results := A{}
		targets := append(A{}, selected...)
		if action == "stop" {
			for a, b := 0, len(targets)-1; a < b; a, b = a+1, b-1 {
				targets[a], targets[b] = targets[b], targets[a]
			}
		}
		for _, v := range targets {
			id := str(obj(v)["id"])
			if ctx.Err() != nil {
				return results, fail("CANCELLED", "Se canceló el cliente; revisa el estado de Docker.", 409)
			}
			line(action+": "+id, "stdout")
			result, e := s.actOne(ctx, id, action, req, line)
			if e != nil {
				results = append(results, J{"id": id, "state": "failed", "error": publicError(e)})
			} else {
				results = append(results, J{"id": id, "state": "succeeded", "result": result})
			}
		}
		for _, v := range results {
			if str(obj(v)["state"]) == "failed" {
				return nil, detailed("ACTION_PARTIAL", "Uno o más objetivos fallaron. Los éxitos anteriores no se revierten.", 422, J{"results": results})
			}
		}
		return results, nil
	})
}
func (s *Service) actOne(ctx context.Context, id, action string, req J, line func(string, string)) (J, error) {
	st, e := s.Store.Stack(id)
	if e != nil {
		return nil, e
	}
	s.StopWatch(id)
	svc := str(req["service"])
	if action == "stop" || action == "restart" {
		return s.Docker.Existing(ctx, st, action, svc, RunOptions{Line: line})
	}
	mode := text(req["mode"], text(st["activeMode"], "dev"))
	if active := str(st["activeMode"]); active != "" && active != mode {
		if !truth(req["confirmMode"]) {
			return nil, fail("MODE_CONFIRM_REQUIRED", "Cambiar de modo recrea servicios y comparte datos. Confirma explícitamente.", 409)
		}
		if svc != "" {
			return nil, fail("PARTIAL_MODE_SWITCH", "Cambia el modo del stack completo.", 409)
		}
	}
	res, e := s.Compose.Approved(ctx, st, mode)
	if e != nil {
		return nil, e
	}
	info, e := s.Docker.Info(ctx)
	if e != nil {
		return nil, e
	}
	if len(obj(st["binding"])) > 0 {
		if e = verifyBinding(obj(st["binding"]), info); e != nil {
			return nil, e
		}
	}
	all, e := s.Docker.Containers(ctx, false)
	if e != nil {
		return nil, e
	}
	expected, e := activeServices(res.Model, ss(at(st, "modes", mode, "profiles")), svc)
	if e != nil {
		return nil, e
	}
	for _, v := range all {
		c := obj(v)
		if str(c["project"]) != str(st["projectName"]) {
			continue
		}
		if at(res.Model, "services", str(c["service"])) == nil {
			return nil, fail("ORPHANED_SERVICES", "El Compose omite servicios ya existentes. Resuélvelos explícitamente; no se eliminan.", 409)
		}
		if !ownsContainer(st, c, str(s.Store.Get()["owner"])) {
			return nil, fail("ADOPTION_REQUIRED", "Vincula los contenedores existentes antes de modificarlos.", 409)
		}
		if svc == "" && str(st["activeMode"]) != "" && str(st["activeMode"]) != mode && truth(c["running"]) && !contains(expected, str(c["service"])) {
			return nil, fail("INACTIVE_MODE_SERVICES", "El nuevo modo deja fuera servicios activos. Deténlos explícitamente.", 409)
		}
	}
	if len(arr(res.Routing["targets"])) > 0 {
		if e = s.Proxy.Ensure(ctx, line); e != nil {
			return nil, e
		}
	}
	if e = s.Infra.EnsureBindings(ctx, st, mode, line); e != nil {
		return nil, e
	}
	if st["proxyApplied"] != nil && str(at(st, "proxyApplied", "mode")) != mode {
		if e = s.Store.Update(func(v J) error { return editStack(v, id, func(st J) error { delete(st, "proxyApplied"); return nil }) }); e != nil {
			return nil, e
		}
		if e = s.Proxy.SyncRoutes(); e != nil {
			return nil, e
		}
	}
	if e = s.Store.Update(func(v J) error {
		return editStack(v, id, func(st J) error {
			if st["binding"] == nil {
				st["binding"] = J{"endpoint": info["endpoint"], "engineId": info["id"], "adoptedIds": A{}}
			}
			st["activeMode"] = mode
			if svc != "" {
				st["expectedServices"] = unique(append(ss(st["expectedServices"]), expected...))
			} else {
				st["expectedServices"] = expected
			}
			return nil
		})
	}); e != nil {
		return nil, e
	}
	st, _ = s.Store.Stack(id)
	result, runErr := s.Compose.Execute(ctx, st, mode, action, svc, truth(req["wait"]), line)
	applied := A{}
	if svc != "" {
		for _, r := range arr(at(st, "proxyApplied", "routes")) {
			if !contains(expected, str(obj(r)["service"])) {
				applied = append(applied, r)
			}
		}
	}
	if runErr != nil {
		observed, oe := s.Docker.Owned(ctx, st)
		if oe == nil {
			for _, raw := range arr(res.Routing["targets"]) {
				r := obj(raw)
				if !contains(expected, str(r["service"])) {
					continue
				}
				for _, cv := range observed {
					c := obj(cv)
					if truth(c["running"]) && str(c["service"]) == str(r["service"]) && str(at(c, "labels", LOwner)) == str(s.Store.Get()["owner"]) && contains(ss(at(c, "networks", str(res.Routing["network"]), "aliases")), str(r["alias"])) {
						applied = append(applied, r)
						break
					}
				}
			}
			if len(applied) > 0 {
				_ = s.Proxy.Apply(st, mode, applied)
			}
		}
		return nil, runErr
	}
	for _, raw := range arr(res.Routing["targets"]) {
		if svc == "" || contains(expected, str(obj(raw)["service"])) {
			applied = append(applied, raw)
		}
	}
	if e = s.Proxy.Apply(st, mode, applied); e != nil {
		return nil, e
	}
	return result, nil
}
func (s *Service) InfraAction(req J) (J, error) {
	action := str(req["action"])
	if !contains([]string{"create", "start", "stop", "database", "check", "bind", "unbind", "check-binding", "backup", "restore", "logs"}, action) {
		return nil, fail("INFRA_ACTION", "Acción de infraestructura desconocida.", 400)
	}
	req = copyJ(req)
	s.mutation.Lock()
	defer s.mutation.Unlock()
	return s.Ops.Submit("infra-"+action, []string{"infra:" + text(req["instance"], text(req["database"], text(req["binding"], action)))}, true, false, func(ctx context.Context, line func(string, string)) (any, error) {
		defer func() { s.emitCatalog(); go s.Refresh(s.ctx) }()
		switch action {
		case "create":
			return s.Infra.Create(ctx, req, line)
		case "start":
			return s.Infra.Start(ctx, str(req["instance"]), line)
		case "stop":
			return s.Infra.Stop(ctx, req, line)
		case "database":
			return s.Infra.CreateDatabase(ctx, req, line)
		case "check":
			return s.Infra.Probe(ctx, str(req["database"]))
		case "bind":
			return s.Infra.Bind(ctx, req)
		case "unbind":
			return s.Infra.Unbind(req)
		case "check-binding":
			return s.Infra.BindingCheck(ctx, text(req["binding"], str(req["id"])))
		case "backup":
			return s.Infra.Backup(ctx, req)
		case "restore":
			return s.Infra.Restore(ctx, req)
		case "logs":
			return J{"note": "Últimas 200 líneas."}, s.InfraLogs(ctx, str(req["instance"]), J{"tail": 200}, func(v J) { line(str(v["text"]), str(v["stream"])) })
		}
		return nil, fail("INFRA_ACTION", "Acción no válida.", 400)
	})
}
func (s *Service) ProxyAction(req J) (J, error) {
	action := str(req["action"])
	if !contains([]string{"start", "stop"}, action) {
		return nil, fail("PROXY_ACTION", "Usa start o stop.", 400)
	}
	req = copyJ(req)
	s.mutation.Lock()
	defer s.mutation.Unlock()
	return s.Ops.Submit("proxy-"+action, []string{}, true, false, func(ctx context.Context, line func(string, string)) (any, error) {
		defer func() { s.emitCatalog(); go s.Refresh(s.ctx) }()
		if action == "stop" {
			return s.Proxy.Stop(ctx, req, line)
		}
		return s.Proxy.Start(ctx, req, line)
	})
}
func (s *Service) RuntimeAction(req J) (J, error) {
	action := str(req["action"])
	if !contains([]string{"start", "configure"}, action) {
		return nil, fail("RUNTIME_ACTION", "Usa start o configure.", 400)
	}
	if !truth(req["confirm"]) {
		return nil, fail("CONFIRM_REQUIRED", "Confirma la operación de runtime.", 409)
	}
	req = copyJ(req)
	s.mutation.Lock()
	defer s.mutation.Unlock()
	return s.Ops.Submit("runtime-"+action, []string{}, true, false, func(ctx context.Context, line func(string, string)) (any, error) {
		defer func() { go s.Refresh(s.ctx) }()
		if action == "start" {
			return s.Runtime.Start(ctx, line)
		}
		s.StopWatches()
		return s.Runtime.Configure(ctx, req, line)
	})
}
func (s *Service) ToolsAction(req J) (J, error) {
	if s.HasWatch() {
		return nil, fail("WATCH_ACTIVE", "Detén Watch antes de mantener herramientas.", 409)
	}
	req = copyJ(req)
	s.mutation.Lock()
	defer s.mutation.Unlock()
	return s.Ops.Submit("tool-"+str(req["action"]), []string{}, true, false, func(ctx context.Context, line func(string, string)) (any, error) {
		return s.Tools.Apply(ctx, req, line)
	})
}
func (s *Service) SetRuntime(req J) (J, error) {
	rt := J{"kind": req["kind"], "context": req["context"], "profile": text(req["profile"], "default")}
	if !contains([]string{"colima", "native"}, str(rt["kind"])) || !validID(str(rt["profile"])) || !validID(str(rt["context"])) {
		return nil, fail("RUNTIME_INVALID", "Contexto/perfil no válidos.", 400)
	}
	if !contains(ss(at(s.Runtime.Capabilities(), "runtime", "supportedKinds")), str(rt["kind"])) {
		return nil, fail("RUNTIME_PLATFORM", "Ese tipo de runtime no está soportado en esta plataforma.", 400)
	}
	s.mutation.Lock()
	defer s.mutation.Unlock()
	if e := s.Idle(); e != nil {
		return nil, e
	}
	d := s.Store.Get()
	bound := at(d, "proxy", "binding") != nil || len(arr(at(d, "infra", "instances"))) > 0
	for _, st := range arr(d["stacks"]) {
		bound = bound || obj(st)["binding"] != nil
	}
	if bound && hash(rt) != hash(d["runtime"]) {
		return nil, fail("BOUND_RUNTIME", "Hay recursos vinculados a este Engine. Usa otro NEARPROD_HOME para otro runtime.", 409)
	}
	if s.HasWatch() {
		return nil, fail("WATCH_ACTIVE", "Detén Watch antes de cambiar contexto.", 409)
	}
	e := s.Store.Update(func(v J) error { v["runtime"] = rt; return nil })
	s.emitCatalog()
	go s.Refresh(s.ctx)
	return rt, e
}
func (s *Service) Close() {
	s.cancel()
	s.Ops.Stop()
	s.StopWatches()
	s.wg.Wait()
	s.refreshMu.Lock()
	s.refreshMu.Unlock()
}

var _ = fmt.Sprintf
var _ = errors.Is
var _ = os.ErrNotExist
