package nearprod

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var hostPartRE = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

func routeHost(v any) (string, error) {
	h := strings.ToLower(str(v))
	if len(h) > 253 || !strings.HasSuffix(h, ".localhost") {
		return "", fail("ROUTE_HOST", "Usa proyecto.localhost sin esquema, puerto, ruta ni wildcard.", 400)
	}
	for _, p := range strings.Split(h, ".") {
		if !hostPartRE.MatchString(p) {
			return "", fail("ROUTE_HOST", "Dominio local no válido.", 400)
		}
	}
	return h, nil
}
func normalizeRoutes(raw any) (A, error) {
	if raw == nil {
		return A{}, nil
	}
	routes, ok := raw.([]any)
	if !ok || len(routes) > 8 {
		return nil, fail("ROUTES_LIMIT", "Configura hasta 8 URLs.", 400)
	}
	out := A{}
	seen := map[string]bool{}
	for _, v := range routes {
		r := obj(v)
		host, e := routeHost(r["host"])
		if e != nil {
			return nil, e
		}
		if seen[host] {
			return nil, fail("ROUTE_DUPLICATE", "Dominio duplicado.", 409)
		}
		seen[host] = true
		svc := str(r["service"])
		p, e := intRange(r["port"], 1, 65535, "Puerto HTTP interno")
		if e != nil {
			return nil, e
		}
		if !svcRE.MatchString(svc) {
			return nil, fail("ROUTE_SERVICE", "Servicio Compose no válido.", 400)
		}
		n := J{"host": host, "service": svc, "port": p}
		if r["verify"] != nil {
			vt := obj(r["verify"])
			vp, e := intRange(vt["port"], 1, 65535, "Puerto verify")
			if e != nil {
				return nil, e
			}
			if !svcRE.MatchString(str(vt["service"])) {
				return nil, fail("ROUTE_SERVICE", "Servicio verify no válido.", 400)
			}
			n["verify"] = J{"service": vt["service"], "port": vp}
		}
		out = append(out, n)
	}
	return out, nil
}
func proxyNames(owner string) J {
	prefix := "nearprod-" + hash(owner)[:12]
	return J{"project": prefix + "-gateway", "network": prefix + "-edge", "networkKey": "np_edge_" + hash(owner)[:12], "uid": prefix + "-gateway"}
}
func routeKey(stack J, host string) string        { return "np-" + hash(A{stack["uid"], host})[:24] }
func serviceAlias(stack J, service string) string { return "np-" + hash(A{stack["uid"], service})[:24] }
func routeURL(host string, port int) string {
	if port == 80 {
		return "http://" + host
	}
	return fmt.Sprintf("http://%s:%d", host, port)
}
func networksOf(service J) J {
	if service["networks"] == nil {
		return J{"default": nil}
	}
	if v := obj(service["networks"]); len(v) > 0 {
		return copyJ(v)
	}
	out := J{}
	for _, s := range ss(service["networks"]) {
		out[s] = nil
	}
	if len(out) == 0 {
		out["default"] = nil
	}
	return out
}
func routingPlan(stack J, mode string, model J, owner string, active []string) (J, error) {
	routes, e := normalizeRoutes(stack["routes"])
	if e != nil {
		return nil, e
	}
	names := proxyNames(owner)
	key := str(names["networkKey"])
	blockers, warnings, risks, targets := A{}, A{}, A{}, A{}
	services, networks := J{}, J{}
	if len(routes) > 0 && obj(model["networks"])[key] != nil {
		blockers = append(blockers, "El Compose usa una clave de red reservada por NearProd.")
	}
	for _, v := range routes {
		route := obj(v)
		t := route
		if mode == "verify" && route["verify"] != nil {
			t = obj(route["verify"])
		}
		svcname := str(t["service"])
		svc := obj(at(model, "services", svcname))
		host := str(route["host"])
		if len(svc) == 0 {
			blockers = append(blockers, host+": servicio no encontrado en "+mode)
			continue
		}
		if !contains(active, svcname) {
			blockers = append(blockers, host+": activa el perfil del servicio.")
			continue
		}
		if svc["network_mode"] != nil {
			blockers = append(blockers, host+": network_mode no compatible con redes gestionadas.")
			continue
		}
		nets := networksOf(svc)
		for n := range nets {
			if truth(at(model, "networks", n, "internal")) {
				risks = append(risks, host+": se amplía la conectividad SOLO de este servicio HTTP; conserva sus redes internas.")
			}
		}
		if len(arr(svc["ports"])) > 0 {
			warnings = append(warnings, svcname+": se conservan puertos publicados; pueden colisionar con otros proyectos.")
		}
		if integer(t["port"]) == 9000 && strings.Contains(strings.ToLower(str(svc["image"])+svcname), "fpm") {
			blockers = append(blockers, host+": PHP-FPM no es HTTP; selecciona Caddy/Nginx/Apache.")
		}
		alias := serviceAlias(stack, svcname)
		targets = append(targets, J{"host": host, "service": svcname, "port": t["port"], "alias": alias, "key": routeKey(stack, host)})
		nets[key] = J{"aliases": A{alias}}
		services[svcname] = J{"networks": nets}
	}
	if len(routes) > 0 {
		networks[key] = J{"name": names["network"], "external": true}
	}
	return J{"targets": targets, "blockers": blockers, "warnings": warnings, "risks": risks, "services": services, "networks": networks, "network": names["network"]}, nil
}
func dynamicConfig(stacks A) J {
	routers, services, middlewares := J{}, J{}, J{}
	for _, raw := range stacks {
		stack := obj(raw)
		for _, r := range arr(at(stack, "proxyApplied", "routes")) {
			route := obj(r)
			key := routeKey(stack, str(route["host"]))
			mark := key + "-stamp"
			routers[key] = J{"rule": "Host(`" + str(route["host"]) + "`)", "entryPoints": A{"web"}, "service": key, "middlewares": A{mark}}
			services[key] = J{"loadBalancer": J{"passHostHeader": true, "servers": A{J{"url": fmt.Sprintf("http://%s:%d", serviceAlias(stack, str(route["service"])), integer(route["port"]))}}}}
			middlewares[mark] = J{"headers": J{"customResponseHeaders": J{"X-Nearprod-Route": key}}}
		}
	}
	return J{"http": J{"routers": routers, "services": services, "middlewares": middlewares}}
}

type PortCheck func(int) J

func availablePort(port int) J {
	return checkPortWith(port, net.Listen, func(addr string) (net.Conn, error) { return net.DialTimeout("tcp", addr, 750*time.Millisecond) })
}
func checkPortWith(port int, listen func(string, string) (net.Listener, error), dial func(string) (net.Conn, error)) J {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, e := listen("tcp4", addr)
	if e == nil {
		_ = ln.Close()
		return J{"available": true}
	}
	if errors.Is(e, syscall.EADDRINUSE) {
		return J{"available": false, "error": "IN_USE"}
	}
	if errors.Is(e, syscall.EACCES) || errors.Is(e, syscall.EPERM) {
		conn, err := dial(addr)
		if err == nil {
			_ = conn.Close()
			return J{"available": false, "error": "IN_USE"}
		}
		if errors.Is(err, syscall.ECONNREFUSED) {
			return J{"available": true, "warning": "HOST_BIND_PERMISSION"}
		}
		return J{"available": false, "error": "PERMISSION"}
	}
	return J{"available": false, "error": "PORT_CHECK"}
}

type Proxy struct {
	Store     *Store
	Docker    *Docker
	Compose   *Compose
	PortCheck PortCheck
	mu        sync.Mutex
}

func (p *Proxy) Settings() J {
	return merge(J{"enabled": false, "port": 80, "image": TraefikImage}, obj(p.Store.Get()["proxy"]))
}

// Keep old mount paths. Moving a bind source during an upgrade can hide data/config.
func (p *Proxy) Paths() J {
	root := filepath.Join(p.Store.Home, "proxy")
	return J{"root": root, "compose": filepath.Join(root, "compose.json"), "config": filepath.Join(root, "config"), "static": filepath.Join(root, "config", "traefik.yaml"), "dynamic": filepath.Join(root, "config", "dynamic", "routes.yaml")}
}
func (p *Proxy) IsOwn(c J) bool {
	state := p.Store.Get()
	n := proxyNames(str(state["owner"]))
	return str(c["project"]) == str(n["project"]) && str(at(c, "labels", LOwner)) == str(state["owner"]) && str(at(c, "labels", LStack)) == str(n["uid"])
}
func (p *Proxy) Summary(obs J) J {
	settings := p.Settings()
	n := proxyNames(str(p.Store.Get()["owner"]))
	state := "not-created"
	var current J
	conflict := false
	for _, v := range arr(obs["containers"]) {
		c := obj(v)
		if str(c["project"]) != str(n["project"]) {
			continue
		}
		if p.IsOwn(c) {
			current = c
		} else {
			conflict = true
		}
	}
	if len(obj(settings["binding"])) > 0 && verifyBinding(obj(settings["binding"]), obj(obs["info"])) != nil {
		conflict = true
	}
	switch {
	case !truth(obs["connected"]):
		state = "unknown"
	case conflict:
		state = "conflict"
	case current != nil && truth(current["running"]):
		state = "starting"
		if str(current["health"]) == "healthy" {
			state = "running"
		} else if str(current["health"]) == "unhealthy" {
			state = "unhealthy"
		}
	case current != nil:
		state = "stopped"
	}
	return J{"enabled": settings["enabled"], "port": settings["port"], "image": settings["image"], "project": n["project"], "network": n["network"], "state": state, "checkedAt": obs["checkedAt"], "containerId": current["id"], "health": text(current["health"], "unchecked")}
}
func (p *Proxy) Routes(stack, obs J) A {
	proxy := p.Summary(obs)
	out := A{}
	for _, raw := range arr(stack["routes"]) {
		route := obj(raw)
		var actual J
		for _, v := range arr(at(stack, "proxyApplied", "routes")) {
			if str(obj(v)["host"]) == str(route["host"]) {
				actual = obj(v)
				break
			}
		}
		state := "pending"
		if actual != nil {
			bound := verifyBinding(obj(stack["binding"]), obj(obs["info"])) == nil
			var current J
			for _, v := range arr(obs["containers"]) {
				c := obj(v)
				if truth(c["running"]) && str(c["service"]) == str(actual["service"]) && ownsContainer(stack, c, str(p.Store.Get()["owner"])) {
					current = c
					break
				}
			}
			switch {
			case !truth(obs["connected"]) || !bound:
				state = "unknown"
			case str(proxy["state"]) != "running":
				state = "proxy-unavailable"
			case current == nil:
				state = "application-stopped"
			case !contains(ss(at(current, "networks", str(proxy["network"]), "aliases")), str(actual["alias"])):
				state = "network-missing"
			default:
				state = "ready-to-check"
			}
		}
		target := route
		if actual != nil {
			target = actual
		}
		out = append(out, J{"host": route["host"], "url": routeURL(str(route["host"]), integer(proxy["port"])), "service": target["service"], "port": target["port"], "state": state, "mode": at(stack, "proxyApplied", "mode")})
	}
	return out
}
func (p *Proxy) SyncRoutes() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	paths := p.Paths()
	file := str(paths["dynamic"])
	for _, dir := range []string{str(paths["config"]), filepath.Dir(file)} {
		if e := os.MkdirAll(dir, 0755); e != nil {
			return e
		}
		st, e := os.Lstat(dir)
		if e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return fail("PROXY_PATH", "Directorio del proxy no seguro.", 409)
		}
		if e = os.Chmod(dir, 0755); e != nil {
			return e
		}
	}
	data, _ := json.MarshalIndent(dynamicConfig(arr(p.Store.Get()["stacks"])), "", "  ")
	data = append(data, '\n')
	if old, e := os.ReadFile(file); e == nil && string(old) == string(data) {
		return nil
	}
	return atomicBytes(file, data, 0644)
}
func (d *Docker) InspectNamed(ctx context.Context, kind, name string) (J, error) {
	r, e := d.Runner.Run(ctx, "docker", d.Args(kind, "inspect", name), RunOptions{Exact: true, Limit: 2 << 20})
	if e != nil {
		return nil, e
	}
	if r.Code != 0 {
		lower := strings.ToLower(r.Stderr)
		if strings.Contains(lower, "not found") || strings.Contains(lower, "no such") {
			return nil, nil
		}
		return nil, fail("RESOURCE_INSPECT", "No se pudo inspeccionar "+kind+"; no se creará un recurso a ciegas.", 503)
	}
	var values []J
	if json.Unmarshal([]byte(r.Stdout), &values) != nil || len(values) != 1 {
		return nil, fail("RESOURCE_RESPONSE", "Respuesta no válida al inspeccionar recurso.", 503)
	}
	return values[0], nil
}
func (p *Proxy) Network(ctx context.Context, create bool, line func(string, string)) (J, error) {
	s := p.Store.Get()
	n := proxyNames(str(s["owner"]))
	v, e := p.Docker.InspectNamed(ctx, "network", str(n["network"]))
	if e != nil {
		return nil, e
	}
	if v == nil {
		if !create {
			return J{"exists": false}, nil
		}
		_, e = p.Docker.Call(ctx, []string{"network", "create", "--driver", "bridge", "--label", LOwner + "=" + str(s["owner"]), "--label", LStack + "=" + str(n["uid"]), str(n["network"])}, RunOptions{Line: line})
		if e != nil {
			return nil, e
		}
		return p.Network(ctx, false, line)
	}
	if str(at(v, "Labels", LOwner)) != str(s["owner"]) || str(at(v, "Labels", LStack)) != str(n["uid"]) || str(v["Driver"]) != "bridge" {
		return nil, fail("PROXY_NETWORK_CONFLICT", "La red reservada no pertenece a este NearProd.", 409)
	}
	return J{"exists": true, "id": v["Id"]}, nil
}
func (p *Proxy) Preview(ctx context.Context, req J) (J, error) {
	settings := p.Settings()
	port := integer(req["port"])
	if req["port"] == nil {
		port = integer(settings["port"])
	}
	if _, e := intRange(port, 1, 65535, "Puerto proxy"); e != nil {
		return nil, e
	}
	if !p.Compose.Supports(ctx, "up", "--wait-timeout") {
		return nil, fail("COMPOSE_CAPABILITY", "Traefik requiere Compose con --wait/--wait-timeout.", 409)
	}
	info, e := p.Docker.Info(ctx)
	if e != nil {
		return nil, e
	}
	if len(obj(settings["binding"])) > 0 {
		if e = verifyBinding(obj(settings["binding"]), info); e != nil {
			return nil, e
		}
	}
	all, e := p.Docker.Containers(ctx, true)
	if e != nil {
		return nil, e
	}
	n := proxyNames(str(p.Store.Get()["owner"]))
	ids, occupied := []string{}, []string{}
	running, bound := false, false
	for _, v := range all {
		c := obj(v)
		own := p.IsOwn(c)
		if str(c["project"]) == str(n["project"]) {
			if !own {
				return nil, fail("PROXY_OWNERSHIP", "Hay un proxy ajeno con el nombre reservado.", 409)
			}
			ids = append(ids, str(c["id"]))
			running = running || truth(c["running"])
		}
		if truth(c["running"]) {
			for _, raw := range arr(c["ports"]) {
				b := obj(raw)
				if integer(b["port"]) == port && strings.HasSuffix(str(b["container"]), "/tcp") && contains([]string{"", "127.0.0.1", "0.0.0.0", "::"}, str(b["host"])) {
					if own {
						bound = true
					} else {
						occupied = append(occupied, str(c["name"]))
					}
				}
			}
		}
	}
	check := J{"available": true}
	if !bound {
		check = p.PortCheck(port)
	}
	blockers := A{}
	if len(occupied) > 0 {
		blockers = append(blockers, fmt.Sprintf("Puerto %d ocupado por: %s. No se detendrá ese recurso externo.", port, strings.Join(occupied, ", ")))
	} else if !truth(check["available"]) {
		blockers = append(blockers, fmt.Sprintf("Puerto %d no disponible (%s).", port, str(check["error"])))
	}
	if _, e = p.Network(ctx, false, nil); e != nil {
		return nil, e
	}
	impacted := []string{}
	for _, v := range arr(p.Store.Get()["stacks"]) {
		if len(arr(obj(v)["routes"])) > 0 {
			impacted = append(impacted, str(obj(v)["id"]))
		}
	}
	sort.Strings(ids)
	sort.Strings(impacted)
	sort.Strings(occupied)
	note := "Un Traefik compartido en loopback. No se alteran puertos de proyectos ni se usa sudo."
	if check["warning"] != nil {
		note += " El bind del agente no está autorizado, pero no hay listener TCP; Docker/Colima verificará su capacidad real al publicar."
	}
	fp := hash(J{"engine": info, "port": port, "ids": ids, "occupied": occupied, "impacted": impacted, "settings": settings})
	return J{"port": port, "image": TraefikImage, "project": n["project"], "network": n["network"], "impacted": impacted, "blockers": blockers, "running": running, "fingerprint": fp, "note": note}, nil
}
func (p *Proxy) Documents(port int) (J, J) {
	state := p.Store.Get()
	n := proxyNames(str(state["owner"]))
	paths := p.Paths()
	static := J{"global": J{"checkNewVersion": false, "sendAnonymousUsage": false}, "entryPoints": J{"web": J{"address": ":8080"}, "ping": J{"address": ":8082"}}, "ping": J{"entryPoint": "ping"}, "providers": J{"providersThrottleDuration": "200ms", "file": J{"directory": "/etc/traefik/dynamic", "watch": true}}, "log": J{"level": "WARN"}}
	svc := J{"image": TraefikImage, "command": A{"--configFile=/etc/traefik/traefik.yaml"}, "restart": "unless-stopped", "labels": J{LOwner: state["owner"], LStack: n["uid"]}, "ports": A{J{"target": 8080, "published": strconv.Itoa(port), "host_ip": "127.0.0.1", "protocol": "tcp"}}, "volumes": A{J{"type": "bind", "source": paths["config"], "target": "/etc/traefik", "read_only": true}}, "networks": A{n["networkKey"]}, "read_only": true, "cap_drop": A{"ALL"}, "security_opt": A{"no-new-privileges:true"}, "pids_limit": 128, "healthcheck": J{"test": A{"CMD", "traefik", "healthcheck", "--configFile=/etc/traefik/traefik.yaml"}, "interval": "5s", "timeout": "3s", "retries": 12, "start_period": "5s"}, "logging": J{"driver": "json-file", "options": J{"max-size": "5m", "max-file": "2"}}}
	return static, J{"services": J{"gateway": svc}, "networks": J{str(n["networkKey"]): J{"name": n["network"], "external": true}}}
}
func (p *Proxy) Start(ctx context.Context, req J, line func(string, string)) (J, error) {
	if !truth(req["confirm"]) {
		return nil, fail("CONFIRM_REQUIRED", "Confirma activar Traefik.", 409)
	}
	preview, e := p.Preview(ctx, req)
	if e != nil {
		return nil, e
	}
	if str(preview["fingerprint"]) != str(req["fingerprint"]) {
		return nil, fail("PREVIEW_CHANGED", "El entorno cambió; vuelve a revisar Traefik.", 409)
	}
	if len(arr(preview["blockers"])) > 0 {
		return nil, fail("PROXY_PORT", strings.Join(ss(preview["blockers"]), " "), 409)
	}
	info, e := p.Docker.Info(ctx)
	if e != nil {
		return nil, e
	}
	if e = p.Store.Update(func(d J) error {
		d["proxy"] = J{"enabled": true, "port": preview["port"], "image": TraefikImage, "binding": J{"endpoint": info["endpoint"], "engineId": info["id"]}}
		return nil
	}); e != nil {
		return nil, e
	}
	if _, e = p.Network(ctx, true, line); e != nil {
		return nil, e
	}
	paths := p.Paths()
	static, compose := p.Documents(integer(preview["port"]))
	if e = p.SyncRoutes(); e != nil {
		return nil, e
	}
	data, _ := json.MarshalIndent(static, "", "  ")
	if e = atomicBytes(str(paths["static"]), append(data, '\n'), 0644); e != nil {
		return nil, e
	}
	if e = writeCompose(str(paths["compose"]), compose); e != nil {
		return nil, e
	}
	_, e = p.Docker.Call(ctx, []string{"compose", "--project-directory", str(paths["root"]), "--project-name", str(preview["project"]), "-f", str(paths["compose"]), "up", "--detach", "--force-recreate", "--wait", "--wait-timeout", "90"}, RunOptions{Dir: str(paths["root"]), Timeout: 5 * time.Minute, Line: line})
	return J{"started": e == nil, "port": preview["port"], "note": "Revisa/inicia las aplicaciones para aplicar sus conexiones de red."}, e
}
func (p *Proxy) Ensure(ctx context.Context, line func(string, string)) error {
	s := p.Settings()
	if !truth(s["enabled"]) {
		return fail("PROXY_SETUP_REQUIRED", "Activa Traefik una vez desde Accesos locales.", 409)
	}
	pre, e := p.Preview(ctx, J{"port": s["port"]})
	if e != nil {
		return e
	}
	if len(arr(pre["blockers"])) > 0 {
		return fail("PROXY_PORT", strings.Join(ss(pre["blockers"]), " "), 409)
	}
	if !truth(pre["running"]) {
		_, e = p.Start(ctx, merge(pre, J{"confirm": true}), line)
		return e
	}
	all, e := p.Docker.Containers(ctx, false)
	if e != nil {
		return e
	}
	for _, raw := range all {
		c := obj(raw)
		if p.IsOwn(c) && str(c["health"]) == "unhealthy" {
			return fail("PROXY_UNHEALTHY", "Traefik no está saludable.", 422)
		}
	}
	netw, e := p.Network(ctx, false, nil)
	if e != nil {
		return e
	}
	if !truth(netw["exists"]) {
		return fail("PROXY_NETWORK", "Falta la red del proxy; activa/repara Traefik.", 409)
	}
	return p.SyncRoutes()
}
func (p *Proxy) Stop(ctx context.Context, req J, line func(string, string)) (J, error) {
	if !truth(req["confirm"]) {
		return nil, fail("CONFIRM_REQUIRED", "Confirma detener el punto de acceso de todas las aplicaciones.", 409)
	}
	info, e := p.Docker.Info(ctx)
	if e != nil {
		return nil, e
	}
	settings := p.Settings()
	if e = verifyBinding(obj(settings["binding"]), info); e != nil {
		return nil, e
	}
	cs, e := p.Docker.Containers(ctx, false)
	if e != nil {
		return nil, e
	}
	names := proxyNames(str(p.Store.Get()["owner"]))
	ids := []string{}
	for _, v := range cs {
		c := obj(v)
		if str(c["project"]) == str(names["project"]) {
			if !p.IsOwn(c) {
				return nil, fail("PROXY_OWNERSHIP", "Proxy ajeno no se detendrá.", 409)
			}
			if truth(c["running"]) {
				ids = append(ids, str(c["id"]))
			}
		}
	}
	if len(ids) > 0 {
		if _, e = p.Docker.Call(ctx, append([]string{"stop", "--time", "10"}, ids...), RunOptions{Line: line, Timeout: time.Minute}); e != nil {
			return nil, e
		}
	}
	e = p.Store.Update(func(d J) error { settings["enabled"] = false; d["proxy"] = settings; return nil })
	return J{"stopped": cloneStrings(ids), "note": "Aplicaciones, redes y datos permanecen."}, e
}
func (p *Proxy) Apply(stack J, mode string, routes A) error {
	e := p.Store.Update(func(d J) error {
		return editStack(d, str(stack["id"]), func(s J) error { s["proxyApplied"] = J{"mode": mode, "routes": routes, "appliedAt": now()}; return nil })
	})
	if e != nil {
		return e
	}
	return p.SyncRoutes()
}
func (p *Proxy) Check(ctx context.Context, stack J, host string) (J, error) {
	info, err := p.Docker.Info(ctx)
	if err != nil {
		return nil, err
	}
	if err = verifyBinding(obj(p.Settings()["binding"]), info); err != nil {
		return nil, err
	}
	if len(obj(stack["binding"])) > 0 {
		if err = verifyBinding(obj(stack["binding"]), info); err != nil {
			return nil, err
		}
	}
	valid := false
	for _, r := range arr(stack["routes"]) {
		valid = valid || str(obj(r)["host"]) == host
	}
	if !valid {
		return nil, fail("ROUTE_NOT_FOUND", "Dominio no registrado en esta aplicación.", 404)
	}
	port := integer(p.Settings()["port"])
	url := routeURL(host, port)
	timeout, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(timeout, "GET", fmt.Sprintf("http://127.0.0.1:%d/", port), nil)
	req.Host = host
	if port != 80 {
		req.Host = fmt.Sprintf("%s:%d", host, port)
	}
	client := http.Client{Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response := J{}
	res, e := client.Do(req)
	if e != nil {
		response["error"] = "HTTP_CONNECTION"
	} else {
		response["status"] = res.StatusCode
		response["marker"] = res.Header.Get("X-Nearprod-Route")
		_ = res.Body.Close()
	}
	dnsctx, cancelDNS := context.WithTimeout(ctx, 2*time.Second)
	defer cancelDNS()
	ips, de := net.DefaultResolver.LookupHost(dnsctx, host)
	dnsstate := "unresolved"
	if de == nil && len(ips) > 0 {
		dnsstate = "loopback"
		for _, ip := range ips {
			if parsed := net.ParseIP(ip); parsed == nil || !parsed.IsLoopback() {
				dnsstate = "not-loopback"
			}
		}
	}
	applied := false
	for _, r := range arr(at(stack, "proxyApplied", "routes")) {
		applied = applied || str(obj(r)["host"]) == host
	}
	state := "pending"
	if applied {
		switch {
		case e != nil:
			state = "connection-failed"
		case str(response["marker"]) != routeKey(stack, host):
			state = "route-not-loaded"
		case contains([]string{"502", "503", "504"}, str(response["status"])):
			state = "upstream-error"
		default:
			state = "http-response"
		}
	}
	return J{"host": host, "url": url, "state": state, "checkedAt": now(), "response": response, "dns": J{"state": dnsstate, "addresses": cloneStrings(ips)}, "hostsEntry": "127.0.0.1 " + host, "note": "Respuesta HTTP no certifica salud funcional; no se siguen redirecciones ni se guarda el cuerpo."}, nil
}
