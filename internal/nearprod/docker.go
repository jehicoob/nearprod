package nearprod

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const LProject = "com.docker.compose.project"
const LService = "com.docker.compose.service"
const LWorking = "com.docker.compose.project.working_dir"
const LOwner = "io.nearprod.owner"
const LStack = "io.nearprod.stack"
const LResource = "io.nearprod.resource"
const LHelper = "io.nearprod.helper"

type Docker struct {
	Runner Runner
	Store  *Store
}

func (d *Docker) Args(args ...string) []string {
	return append([]string{"--context", str(at(d.Store.Get(), "runtime", "context"))}, args...)
}
func (d *Docker) Call(ctx context.Context, args []string, o RunOptions) (Result, error) {
	return checked(ctx, d.Runner, "docker", d.Args(args...), o)
}
func (d *Docker) Endpoint(ctx context.Context) (string, error) {
	rt := obj(d.Store.Get()["runtime"])
	r, e := checked(ctx, d.Runner, "docker", []string{"context", "inspect", str(rt["context"])}, RunOptions{Exact: true, Limit: 1 << 20})
	if e != nil {
		return "", e
	}
	var v []J
	if json.Unmarshal([]byte(r.Stdout), &v) != nil || len(v) != 1 {
		return "", fail("DOCKER_RESPONSE", "Contexto Docker no válido.", 503)
	}
	ep := str(at(v[0], "Endpoints", "docker", "Host"))
	if !strings.HasPrefix(ep, "unix:///") {
		return "", fail("REMOTE_CONTEXT", "Solo se permiten contextos locales por socket Unix. No se conectará al VPS.", 409)
	}
	if str(rt["kind"]) == "colima" {
		base := os.Getenv("COLIMA_HOME")
		if base == "" {
			base = filepath.Join(userHome(), ".colima")
		}
		expected := "unix://" + filepath.Join(base, str(rt["profile"]), "docker.sock")
		if ep != expected {
			return "", fail("COLIMA_CONTEXT_MISMATCH", "El contexto no corresponde al perfil Colima seleccionado.", 409)
		}
	}
	return ep, nil
}
func (d *Docker) Info(ctx context.Context) (J, error) {
	ep, e := d.Endpoint(ctx)
	if e != nil {
		return nil, e
	}
	r, e := d.Call(ctx, []string{"info", "--format", "{{json .}}"}, RunOptions{Exact: true, Limit: 2 << 20})
	if e != nil {
		return nil, e
	}
	v, e := decodeObject([]byte(r.Stdout))
	if e != nil {
		return nil, e
	}
	if str(v["ID"]) == "" || str(v["OSType"]) != "linux" {
		return nil, fail("ENGINE_UNSUPPORTED", "Se requiere un Engine Linux identificado.", 409)
	}
	return J{"endpoint": ep, "id": v["ID"], "version": v["ServerVersion"], "cpus": v["NCPU"], "memoryBytes": v["MemTotal"], "architecture": v["Architecture"], "os": "linux"}, nil
}
func publishedURL(port string, b J) any {
	p, proto, _ := strings.Cut(port, "/")
	if proto != "tcp" || !contains([]string{"", "0.0.0.0", "::", "127.0.0.1", "::1"}, str(b["HostIp"])) {
		return nil
	}
	scheme := ""
	if contains([]string{"443", "8443"}, p) {
		scheme = "https"
	} else if contains([]string{"80", "3000", "3001", "4000", "4173", "4200", "5000", "5173", "8000", "8080", "8081", "8888"}, p) {
		scheme = "http"
	}
	hp := integer(b["HostPort"])
	if scheme == "" || hp < 1 || hp > 65535 {
		return nil
	}
	host := "127.0.0.1"
	if strings.Contains(str(b["HostIp"]), ":") {
		host = "[::1]"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, host, hp)
}
func summarizeContainer(v J) J {
	labels := obj(at(v, "Config", "Labels"))
	state := obj(v["State"])
	ports := A{}
	for port, bindings := range obj(at(v, "NetworkSettings", "Ports")) {
		for _, raw := range arr(bindings) {
			b := obj(raw)
			ports = append(ports, J{"container": port, "host": b["HostIp"], "port": integer(b["HostPort"]), "url": publishedURL(port, b)})
		}
	}
	nets := J{}
	for k, v := range obj(at(v, "NetworkSettings", "Networks")) {
		nets[k] = J{"aliases": list(obj(v)["Aliases"])}
	}
	safeLabels := J{}
	for _, k := range []string{LProject, LService, LWorking, LOwner, LStack, LResource, LHelper, "com.docker.compose.project.config_files"} {
		if labels[k] != nil {
			safeLabels[k] = labels[k]
		}
	}
	return J{"id": v["Id"], "name": strings.TrimPrefix(str(v["Name"]), "/"), "project": text(labels[LProject], ""), "service": text(labels[LService], ""), "state": text(state["Status"], "unknown"), "running": truth(state["Running"]), "health": text(at(state, "Health", "Status"), "unchecked"), "exitCode": state["ExitCode"], "oom": truth(state["OOMKilled"]), "image": at(v, "Config", "Image"), "imageId": v["Image"], "platform": v["Platform"], "networks": nets, "restartCount": v["RestartCount"], "ports": ports, "labels": safeLabels}
}
func (d *Docker) Containers(ctx context.Context, all bool) (A, error) {
	args := []string{"ps", "-a", "-q"}
	if !all {
		args = append(args, "--filter", "label="+LProject)
	}
	r, e := d.Call(ctx, args, RunOptions{Exact: true, Limit: 2 << 20})
	if e != nil {
		return nil, e
	}
	ids := strings.Fields(r.Stdout)
	out := A{}
	for len(ids) > 0 {
		n := len(ids)
		if n > 80 {
			n = 80
		}
		part := ids[:n]
		ids = ids[n:]
		for _, id := range part {
			if !cidRE.MatchString(id) {
				return nil, fail("DOCKER_RESPONSE", "ID de contenedor no válido.", 503)
			}
		}
		res, e := d.Runner.Run(ctx, "docker", d.Args(append([]string{"inspect"}, part...)...), RunOptions{Exact: true, Limit: 8 << 20})
		if e != nil {
			return nil, e
		}
		var values []J
		if json.Unmarshal([]byte(res.Stdout), &values) != nil {
			return nil, fail("DOCKER_INSPECT", "No se pudieron inspeccionar los contenedores; actualiza estado.", 503)
		}
		for _, v := range values {
			out = append(out, summarizeContainer(v))
		}
	}
	return out, nil
}
func ownsContainer(stack, c J, owner string) bool {
	if str(c["project"]) != str(stack["projectName"]) {
		return false
	}
	if str(at(c, "labels", LOwner)) == owner && str(at(c, "labels", LStack)) == str(stack["uid"]) {
		return true
	}
	return contains(ss(at(stack, "binding", "adoptedIds")), str(c["id"]))
}
func verifyBinding(binding, info J) error {
	if len(binding) == 0 || str(binding["endpoint"]) != str(info["endpoint"]) || str(binding["engineId"]) != str(info["id"]) {
		return fail("ENGINE_IDENTITY", "El Engine/endpoint no coincide con la vinculación guardada. No se adoptan recursos por su nombre.", 409)
	}
	return nil
}
func (d *Docker) Owned(ctx context.Context, stack J) (A, error) {
	info, e := d.Info(ctx)
	if e != nil {
		return nil, e
	}
	if e = verifyBinding(obj(stack["binding"]), info); e != nil {
		return nil, e
	}
	all, e := d.Containers(ctx, false)
	if e != nil {
		return nil, e
	}
	out := A{}
	owner := str(d.Store.Get()["owner"])
	for _, v := range all {
		c := obj(v)
		if str(c["project"]) == str(stack["projectName"]) {
			if !ownsContainer(stack, c, owner) {
				return nil, fail("UNOWNED_CONTAINERS", "Hay contenedores sin adoptar; no se modificarán por coincidencia de nombre.", 409)
			}
			out = append(out, c)
		}
	}
	return out, nil
}
func (d *Docker) Existing(ctx context.Context, stack J, action, service string, o RunOptions) (J, error) {
	all, e := d.Owned(ctx, stack)
	if e != nil {
		return nil, e
	}
	ids := []string{}
	matched := false
	for _, v := range all {
		c := obj(v)
		if service != "" && str(c["service"]) != service {
			continue
		}
		matched = true
		if action == "stop" && !truth(c["running"]) && !contains([]string{"restarting", "paused"}, str(c["state"])) {
			continue
		}
		ids = append(ids, str(c["id"]))
	}
	if service != "" && !matched {
		return nil, fail("SERVICE_NOT_FOUND", "No hay contenedores de ese servicio.", 404)
	}
	if len(ids) > 0 {
		o.Timeout = time.Duration(60+20*len(ids)) * time.Second
		_, e = d.Call(ctx, append([]string{action, "--time", "15"}, ids...), o)
	}
	return J{"ids": cloneStrings(ids), "noOp": len(ids) == 0}, e
}
func stackState(cs A, expected []string, connected bool) J {
	r := J{"execution": "unknown", "health": "unknown"}
	if !connected {
		return r
	}
	r["health"] = "unchecked"
	if len(cs) == 0 {
		r["execution"] = "not-created"
		return r
	}
	running, healthy := 0, 0
	failed, missing, restarting, paused, starting, unhealthy := false, false, false, false, false, false
	for _, raw := range cs {
		c := obj(raw)
		if truth(c["running"]) {
			running++
			if str(c["health"]) == "healthy" {
				healthy++
			}
			starting = starting || str(c["health"]) == "starting"
			unhealthy = unhealthy || str(c["health"]) == "unhealthy"
		}
		failed = failed || truth(c["oom"]) || (!truth(c["running"]) && integer(c["exitCode"]) != 0)
		restarting = restarting || str(c["state"]) == "restarting"
		paused = paused || str(c["state"]) == "paused"
	}
	for _, name := range expected {
		found := false
		for _, v := range cs {
			found = found || str(obj(v)["service"]) == name
		}
		missing = missing || !found
	}
	switch {
	case unhealthy:
		r["health"] = "unhealthy"
	case starting:
		r["health"] = "starting"
	case running > 0 && healthy == running:
		r["health"] = "healthy"
	}
	switch {
	case failed:
		r["execution"] = "failed"
	case restarting:
		r["execution"] = "restarting"
	case paused:
		r["execution"] = "paused"
	case missing:
		r["execution"] = "partial"
	case running == len(cs):
		r["execution"] = "running"
	case running > 0:
		r["execution"] = "partial"
	default:
		r["execution"] = "stopped"
	}
	return r
}
func (d *Docker) Snapshot(ctx context.Context) (J, error) {
	info, e := d.Info(ctx)
	if e != nil {
		return nil, e
	}
	cs, e := d.Containers(ctx, false)
	if e != nil {
		return nil, e
	}
	return J{"connected": true, "checkedAt": now(), "info": info, "containers": cs}, nil
}
func (d *Docker) Image(ctx context.Context, id string) (J, error) {
	if _, e := d.Endpoint(ctx); e != nil {
		return nil, e
	}
	r, e := d.Call(ctx, []string{"image", "inspect", id}, RunOptions{Exact: true, Limit: 2 << 20})
	if e != nil {
		return nil, e
	}
	var v []J
	if json.Unmarshal([]byte(r.Stdout), &v) != nil || len(v) != 1 {
		return nil, fail("IMAGE_RESPONSE", "No se obtuvo la identidad de imagen.", 503)
	}
	x := v[0]
	platform := str(x["Os"]) + "/" + str(x["Architecture"])
	if s := str(x["Variant"]); s != "" {
		platform += "/" + s
	}
	return J{"id": x["Id"], "platform": platform, "digests": list(x["RepoDigests"]), "architecture": x["Architecture"]}, nil
}
func jsonLines(s string) A {
	var a A
	if json.Unmarshal([]byte(s), &a) == nil && a != nil {
		return a
	}
	out := A{}
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if v, e := decodeObject([]byte(line)); e == nil {
			out = append(out, v)
		}
	}
	return out
}
func (d *Docker) Stats(ctx context.Context) (A, error) {
	r, e := d.Call(ctx, []string{"stats", "--no-stream", "--format", "{{json .}}"}, RunOptions{Timeout: 20 * time.Second, Exact: true, Limit: 2 << 20})
	if e != nil {
		return nil, e
	}
	out := A{}
	for _, raw := range jsonLines(r.Stdout) {
		v := obj(raw)
		out = append(out, J{"id": text(v["ID"], str(v["Container"])), "name": v["Name"], "cpu": v["CPUPerc"], "memory": v["MemUsage"], "memoryPercent": v["MemPerc"], "pids": v["PIDs"]})
	}
	return out, nil
}
