package nearprod

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Infrastructure struct {
	Store     *Store
	Docker    *Docker
	Compose   *Compose
	PortCheck PortCheck
}

func (i *Infrastructure) State() J { return obj(i.Store.Get()["infra"]) }
func (i *Infrastructure) Instance(id string) (J, error) {
	for _, v := range arr(i.State()["instances"]) {
		r := obj(v)
		if str(r["id"]) == id || str(r["uid"]) == id {
			return r, nil
		}
	}
	return nil, fail("INSTANCE_NOT_FOUND", "Instancia no encontrada.", 404)
}
func (i *Infrastructure) Database(id string) (J, error) {
	for _, v := range arr(i.State()["databases"]) {
		if str(obj(v)["id"]) == id {
			return obj(v), nil
		}
	}
	return nil, fail("DATABASE_NOT_FOUND", "Base/credencial no encontrada.", 404)
}
func (i *Infrastructure) ArchivedInstance(id string) (J, error) {
	for _, raw := range arr(i.State()["archivedInstances"]) {
		archived := obj(raw)
		instance := obj(archived["instance"])
		if str(instance["id"]) == id || str(instance["uid"]) == id {
			return archived, nil
		}
	}
	return nil, fail("ARCHIVED_INSTANCE_NOT_FOUND", "Instancia archivada no encontrada.", 404)
}
func instanceDatabases(infra J, uid string) A {
	out := A{}
	for _, raw := range arr(infra["databases"]) {
		if str(obj(raw)["instanceUid"]) == uid {
			out = append(out, copyJ(obj(raw)))
		}
	}
	return out
}
func (i *Infrastructure) Dir(r J) string {
	if d := str(r["managedDir"]); d != "" {
		return filepath.Join(i.Store.Home, d)
	}
	return filepath.Join(i.Store.Home, "infra", str(r["uid"]))
}
func (i *Infrastructure) SecretFile(r J) string { return filepath.Join(i.Dir(r), "vault.json") }
func (i *Infrastructure) Secrets(r J) (J, error) {
	v, e := readJSON(i.SecretFile(r), 1<<20)
	if e != nil {
		return nil, fail("SECRETS_MISSING", "Faltan credenciales privadas. No se regenerarán sobre datos existentes; restaura el vault.", 409)
	}
	if str(v["owner"]) != str(i.Store.Get()["owner"]) || str(v["uid"]) != str(r["uid"]) || !regexp.MustCompile(`^[a-f0-9]{48}$`).MatchString(str(v["admin"])) {
		return nil, fail("SECRETS_OWNER", "Vault no válido o de otra instancia.", 409)
	}
	for _, d := range obj(v["databases"]) {
		if !regexp.MustCompile(`^[a-f0-9]{48}$`).MatchString(str(obj(d)["password"])) {
			return nil, fail("SECRETS_FORMAT", "Credencial privada inválida.", 409)
		}
	}
	return v, nil
}
func (i *Infrastructure) Consumers(r J) A {
	state := i.Store.Get()
	dbids := []string{}
	for _, v := range arr(at(state, "infra", "databases")) {
		d := obj(v)
		if str(d["instanceUid"]) == str(r["uid"]) {
			dbids = append(dbids, str(d["id"]))
		}
	}
	out := A{}
	for _, v := range arr(at(state, "infra", "bindings")) {
		b := obj(v)
		if contains(dbids, str(b["databaseId"])) {
			b = copyJ(b)
			b["stack"] = nil
			for _, s := range arr(state["stacks"]) {
				if str(obj(s)["uid"]) == str(b["stackUid"]) {
					b["stack"] = obj(s)["id"]
				}
			}
			out = append(out, b)
		}
	}
	return out
}
func instanceLocation(r J) string {
	switch str(at(r, "persistence", "kind")) {
	case "volume":
		return "Docker volume: " + str(r["volume"])
	case "folder":
		return filepath.Join(str(at(r, "persistence", "path")), "data")
	default:
		return "Sin persistencia"
	}
}
func (i *Infrastructure) List(obs J) J {
	infra := i.State()
	out := A{}
	for _, v := range arr(infra["instances"]) {
		r := obj(v)
		cs := A{}
		dbs := A{}
		for _, raw := range arr(obs["containers"]) {
			c := obj(raw)
			if str(c["project"]) == str(r["projectName"]) {
				cs = append(cs, c)
			}
		}
		for _, raw := range arr(infra["databases"]) {
			d := obj(raw)
			if str(d["instanceUid"]) == str(r["uid"]) {
				dbs = append(dbs, d)
			}
		}
		out = append(out, merge(merge(r, stackState(cs, []string{"database"}, truth(obs["connected"]))), J{"containers": cs, "consumers": i.Consumers(r), "databases": dbs, "location": instanceLocation(r), "internalHost": r["hostname"], "internalPort": at(engineOptions, str(r["engine"]), "port")}))
	}
	archived := A{}
	for _, raw := range arr(infra["archivedInstances"]) {
		snapshot := obj(raw)
		r := obj(snapshot["instance"])
		archived = append(archived, J{"id": r["id"], "uid": r["uid"], "name": r["name"], "engine": r["engine"], "requestedImage": r["requestedImage"], "persistence": copyJ(obj(r["persistence"])), "location": instanceLocation(r), "hostPort": r["hostPort"], "memoryMiB": r["memoryMiB"], "initialized": r["initialized"], "databaseCount": len(arr(snapshot["databases"])), "archivedAt": snapshot["archivedAt"], "lifecycle": "archived"})
	}
	archivedDatabases := A{}
	for _, raw := range arr(infra["archivedDatabases"]) {
		snapshot := obj(raw)
		database := obj(snapshot["database"])
		archivedDatabases = append(archivedDatabases, merge(databaseSummary(database), J{"archivedAt": snapshot["archivedAt"], "lifecycle": "archived"}))
	}
	return J{"defaultDataRoot": filepath.Join(i.Store.Home, "databases"), "engines": engineOptions, "folderUnsupportedImages": folderUnsupportedImages(i.Store.Get()), "folderRestrictions": folderRestrictions(i.Store.Get()), "instances": out, "archivedInstances": archived, "archivedDatabases": archivedDatabases, "bindings": list(infra["bindings"]), "connected": truth(obs["connected"]), "checkedAt": obs["checkedAt"], "note": "Datos persistentes por instancia. Actualizar NearProd no los mueve. Traefik pertenece a Accesos locales."}
}
func (i *Infrastructure) Ports(ctx context.Context, req J) (J, error) {
	start := 15432
	if req["start"] != nil {
		start = integer(req["start"])
	}
	end := start + 9
	if req["end"] != nil {
		end = integer(req["end"])
	}
	if _, e := intRange(start, 1024, 65535, "Inicio"); e != nil {
		return nil, e
	}
	if _, e := intRange(end, start, min(65535, start+127), "Fin (hasta 128 puertos)"); e != nil {
		return nil, e
	}
	exclude := ""
	if s := str(req["instance"]); s != "" {
		r, e := i.Instance(s)
		if e != nil {
			return nil, e
		}
		exclude = str(r["uid"])
	}
	reservations := map[int]string{}
	for _, v := range allInfraInstances(i.State()) {
		r := obj(v)
		if str(r["uid"]) != exclude && integer(r["hostPort"]) > 0 {
			reservations[integer(r["hostPort"])] = str(r["name"])
		}
	}
	proxy := obj(i.Store.Get()["proxy"])
	if truth(proxy["enabled"]) {
		reservations[integer(proxy["port"])] = "Traefik"
	}
	cs := A{}
	dockerChecked := false
	if _, e := i.Docker.Info(ctx); e == nil {
		if values, e := i.Docker.Containers(ctx, true); e == nil {
			cs = values
			dockerChecked = true
		}
	}
	out := A{}
	for port := start; port <= end; port++ {
		reason := ""
		if r := reservations[port]; r != "" {
			reason = "Reservado por " + r
		}
		for _, v := range cs {
			c := obj(v)
			if str(at(c, "labels", LResource)) == exclude && exclude != "" {
				continue
			}
			if !truth(c["running"]) {
				continue
			}
			for _, raw := range arr(c["ports"]) {
				p := obj(raw)
				if integer(p["port"]) == port && strings.HasSuffix(str(p["container"]), "/tcp") {
					reason = "Publicado por " + str(c["name"])
				}
			}
		}
		free := false
		if reason == "" {
			free = truth(i.PortCheck(port)["available"])
			if !free {
				reason = "Ocupado o no disponible en loopback"
			}
		}
		out = append(out, J{"port": port, "available": free, "reason": reason})
	}
	return J{"start": start, "end": end, "ports": out, "dockerChecked": dockerChecked, "checkedAt": now(), "note": "Disponibilidad momentánea, no reserva; se revisa al iniciar."}, nil
}
func (i *Infrastructure) Preview(ctx context.Context, input J) (J, error) {
	def, e := normalizeInstance(input, i.Store.Home)
	if e != nil {
		return nil, e
	}
	if e = validateFolderPlatform(i.Store.Get(), def); e != nil {
		return nil, e
	}
	for _, v := range allInfraInstances(i.State()) {
		r := obj(v)
		if str(r["id"]) == str(def["id"]) || str(r["uid"]) == str(def["id"]) || str(r["id"]) == str(def["uid"]) || str(r["uid"]) == str(def["uid"]) {
			return nil, fail("INSTANCE_RESERVED", "El ID pertenece a una instancia activa o archivada.", 409)
		}
	}
	if str(at(def, "persistence", "kind")) == "folder" {
		p, e := dataDirectory(str(at(def, "persistence", "path")), i.Store.Home, true)
		if e != nil {
			return nil, e
		}
		obj(def["persistence"])["path"] = p
		for _, v := range allInfraInstances(i.State()) {
			r := obj(v)
			if str(at(r, "persistence", "kind")) == "folder" {
				old := str(at(r, "persistence", "path"))
				if within(old, p) || within(p, old) {
					return nil, fail("DATA_OVERLAP", "La carpeta se solapa con otra instancia.", 409)
				}
			}
		}
	}
	info, e := i.Docker.Info(ctx)
	if e != nil {
		return nil, e
	}
	if !i.Compose.Supports(ctx, "up", "--wait-timeout") {
		return nil, fail("COMPOSE_CAPABILITY", "La infraestructura requiere Compose con --wait-timeout.", 409)
	}
	if p := integer(def["hostPort"]); p > 0 {
		r, e := i.Ports(ctx, J{"start": p, "end": p})
		if e != nil {
			return nil, e
		}
		if !truth(r["dockerChecked"]) || !truth(obj(arr(r["ports"])[0])["available"]) {
			return nil, fail("PORT_OCCUPIED", "Puerto ocupado/reservado; selecciona otro.", 409)
		}
	}
	warnings := A{"Se descarga y fija la imagen oficial. No se cambian repositorios, .env o bases de datos de tus aplicaciones.", "Un límite de RAM no es una reserva. Las instancias comparten los recursos de la VM.", "La persistencia es por instancia, no por base lógica; no equivale a un backup."}
	if str(def["engine"]) == "redis" {
		warnings = append(warnings, "Redis dedicado a una aplicación; no aislamiento multiapp mediante números de DB.")
	}
	review := J{"definition": def, "engine": J{"endpoint": info["endpoint"], "id": info["id"], "architecture": info["architecture"]}, "warnings": warnings}
	review["fingerprint"] = hash(review)
	review["note"] = "Confirmar crea e inicia la instancia. Después crea la base/credencial del proyecto."
	return review, nil
}
func (i *Infrastructure) Create(ctx context.Context, req J, line func(string, string)) (J, error) {
	if !truth(req["confirm"]) {
		return nil, fail("CONFIRM_REQUIRED", "Confirma crear la instancia.", 409)
	}
	pre, e := i.Preview(ctx, req)
	if e != nil {
		return nil, e
	}
	if str(pre["fingerprint"]) != str(req["fingerprint"]) {
		return nil, fail("PREVIEW_CHANGED", "Cambió la revisión; vuelve a comprobarla.", 409)
	}
	info := obj(pre["engine"])
	owner := str(i.Store.Get()["owner"])
	uid := hash(token(24))[:16]
	short := hash(owner)[:8]
	r := merge(obj(pre["definition"]), J{"uid": uid, "projectName": "np-infra-" + short + "-" + uid, "network": "np-data-" + short + "-" + uid, "volume": "np-data-" + short + "-" + uid, "hostname": "np-" + str(at(pre, "definition", "engine")) + "-" + uid, "createdAt": now(), "binding": J{"endpoint": info["endpoint"], "engineId": info["id"]}, "resolvedImage": nil, "initialized": false, "managedDir": filepath.Join("config", "resources", uid)})
	if e = writeJSON(i.SecretFile(r), J{"owner": owner, "uid": uid, "admin": secretHex(), "databases": J{}}); e != nil {
		return nil, e
	}
	if str(at(r, "persistence", "kind")) == "folder" {
		p, e := dataDirectory(str(at(r, "persistence", "path")), i.Store.Home, true)
		if e != nil {
			return nil, e
		}
		if e = os.MkdirAll(p, 0700); e != nil {
			return nil, e
		}
		marker := filepath.Join(p, ".nearprod-resource.json")
		f, e := os.OpenFile(marker, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return nil, e
		}
		e = json.NewEncoder(f).Encode(J{"owner": owner, "uid": uid, "engine": r["engine"], "image": r["requestedImage"]})
		_ = f.Sync()
		_ = f.Close()
		if e != nil {
			return nil, e
		}
		if e = os.Mkdir(filepath.Join(p, "data"), 0700); e != nil {
			return nil, e
		}
	}
	if e = i.Store.Update(func(d J) error {
		infra := obj(d["infra"])
		infra["instances"] = append(arr(infra["instances"]), r)
		return nil
	}); e != nil {
		return nil, e
	}
	result, e := i.Start(ctx, str(r["id"]), line)
	if e != nil {
		return nil, detailed("INSTANCE_START_FAILED", "La instancia quedó registrada, pero falló su inicio. Los datos se conservan; revisa el error y reintenta.", 422, J{"instance": r["id"], "cause": publicError(e)})
	}
	return result, nil
}
func (i *Infrastructure) Verify(ctx context.Context, r J) (J, error) {
	info, e := i.Docker.Info(ctx)
	if e != nil {
		return nil, e
	}
	if e = verifyBinding(obj(r["binding"]), info); e != nil {
		return nil, e
	}
	return info, nil
}
func (i *Infrastructure) Containers(ctx context.Context, r J) (A, error) {
	if _, e := i.Verify(ctx, r); e != nil {
		return nil, e
	}
	cs, e := i.Docker.Containers(ctx, false)
	if e != nil {
		return nil, e
	}
	out := A{}
	owner := str(i.Store.Get()["owner"])
	for _, v := range cs {
		c := obj(v)
		if str(c["project"]) == str(r["projectName"]) {
			if str(at(c, "labels", LOwner)) != owner || str(at(c, "labels", LResource)) != str(r["uid"]) {
				return nil, fail("INFRA_OWNERSHIP", "Contenedor ajeno con la identidad de esta instancia.", 409)
			}
			out = append(out, c)
		}
	}
	return out, nil
}
func (i *Infrastructure) Container(ctx context.Context, r J) (J, error) {
	cs, e := i.Containers(ctx, r)
	if e != nil {
		return nil, e
	}
	for _, v := range cs {
		c := obj(v)
		if truth(c["running"]) {
			return c, nil
		}
	}
	return nil, fail("INSTANCE_OFFLINE", "La instancia no está ejecutándose; inicia el motor.", 409)
}
func (i *Infrastructure) EnsureStorage(ctx context.Context, r J, line func(string, string)) error {
	pairs := [][2]string{{"network", str(r["network"])}}
	if str(at(r, "persistence", "kind")) == "volume" {
		pairs = append(pairs, [2]string{"volume", str(r["volume"])})
	}
	for _, pair := range pairs {
		v, e := i.Docker.InspectNamed(ctx, pair[0], pair[1])
		if e != nil {
			return e
		}
		if v != nil {
			if str(at(v, "Labels", LOwner)) != str(i.Store.Get()["owner"]) || str(at(v, "Labels", LResource)) != str(r["uid"]) {
				return fail("INFRA_OWNERSHIP", "Red/volumen existente no pertenece a la instancia.", 409)
			}
		} else {
			if pair[0] == "volume" && truth(r["initialized"]) {
				return fail("DATA_VOLUME_MISSING", "Desapareció el volumen de una instancia inicializada. No se creará un motor vacío: restaura el volumen o un backup.", 409)
			}
			_, e = i.Docker.Call(ctx, []string{pair[0], "create", "--label", LOwner + "=" + str(i.Store.Get()["owner"]), "--label", LResource + "=" + str(r["uid"]), pair[1]}, RunOptions{Line: line})
			if e != nil {
				return e
			}
		}
	}
	return nil
}
func (i *Infrastructure) PinImage(ctx context.Context, r J, line func(string, string)) (J, error) {
	if str(r["resolvedImage"]) != "" {
		return r, nil
	}
	if _, e := i.Docker.Call(ctx, []string{"pull", str(r["requestedImage"])}, RunOptions{Timeout: 15 * time.Minute, Line: line}); e != nil {
		return nil, e
	}
	img, e := i.Docker.Image(ctx, str(r["requestedImage"]))
	if e != nil {
		return nil, e
	}
	info, e := i.Verify(ctx, r)
	if e != nil {
		return nil, e
	}
	normal := func(s string) string {
		switch s {
		case "x86_64", "x64":
			return "amd64"
		case "aarch64":
			return "arm64"
		}
		return s
	}
	if !strings.HasPrefix(str(img["platform"]), "linux/") || normal(str(img["architecture"])) != normal(str(info["architecture"])) {
		return nil, fail("IMAGE_PLATFORM", "Imagen no nativa para el Engine; no se habilita emulación silenciosamente.", 409)
	}
	resolved := str(img["id"])
	for _, digest := range ss(img["digests"]) {
		if strings.HasPrefix(digest, str(r["engine"])+"@") || strings.Contains(digest, "/"+str(r["engine"])+"@") {
			resolved = digest
			break
		}
	}
	if !strings.Contains(resolved, "sha256:") {
		return nil, fail("IMAGE_ID", "La imagen no tiene identidad comprobable.", 409)
	}
	e = i.Store.Update(func(d J) error {
		return editInstance(d, str(r["uid"]), func(v J) error {
			v["resolvedImage"] = resolved
			v["imageId"] = img["id"]
			v["platform"] = img["platform"]
			return nil
		})
	})
	if e != nil {
		return nil, e
	}
	return i.Instance(str(r["uid"]))
}
func editInstance(d J, uid string, fn func(J) error) error {
	for _, raw := range arr(at(d, "infra", "instances")) {
		r := obj(raw)
		if str(r["uid"]) == uid {
			return fn(r)
		}
	}
	return fail("INSTANCE_NOT_FOUND", "Instancia eliminada.", 409)
}
func (i *Infrastructure) CheckFolder(ctx context.Context, r J, line func(string, string)) error {
	if str(at(r, "persistence", "kind")) != "folder" {
		return nil
	}
	p, e := dataDirectory(str(at(r, "persistence", "path")), i.Store.Home, false)
	if e != nil {
		return e
	}
	marker, e := readJSON(filepath.Join(p, ".nearprod-resource.json"), 1<<20)
	if e != nil || str(marker["owner"]) != str(i.Store.Get()["owner"]) || str(marker["uid"]) != str(r["uid"]) || str(marker["image"]) != str(r["requestedImage"]) {
		return fail("DATA_MARKER", "Falta un marcador válido. No se inicializará una carpeta equivocada.", 409)
	}
	data := filepath.Join(p, "data")
	st, e := os.Lstat(data)
	if e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return fail("DATA_MISSING", "Directorio data ausente/no válido. No se recreará automáticamente.", 409)
	}
	probe := ".nearprod-probe-" + token(8)
	nonce := token(16)
	file := filepath.Join(p, probe)
	f, e := os.OpenFile(file, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if e != nil {
		return e
	}
	_, e = f.WriteString(nonce)
	_ = f.Close()
	if e != nil {
		return e
	}
	defer os.Remove(file)
	script := `test "$(cat "/nproot/$1")" = "$2"; touch "/npdata/$1"; chown "$(id -u "$3")":"$(id -g "$3")" "/npdata/$1"; chmod 600 "/npdata/$1"; rm "/npdata/$1"`
	_, e = i.Helper(ctx, r, []string{"sh", "-ec", script, "--", probe, nonce, str(at(engineOptions, str(r["engine"]), "user"))}, HelperOptions{RunOptions: RunOptions{Line: line}, Network: "none", Extra: []string{"--user", "0:0", "--mount", "type=bind,source=" + p + ",target=/nproot,readonly", "--mount", "type=bind,source=" + data + ",target=/npdata"}})
	if e != nil {
		return detailed("DATA_MOUNT_FAILED", "La carpeta no es visible/escribible desde Docker. Revisa montajes de Colima; no se cambiaron datos ni se aplicó chmod 777.", 422, J{"cause": publicError(e)})
	}
	return nil
}

type HelperOptions struct {
	RunOptions
	Network string
	Extra   []string
}

func (i *Infrastructure) Helper(ctx context.Context, r J, cmd []string, o HelperOptions) (Result, error) {
	if len(cmd) == 0 {
		return Result{}, fail("HELPER_COMMAND", "Cliente vacío.", 400)
	}
	name := "np-client-" + hash(token(24))[:16]
	owner := str(i.Store.Get()["owner"])
	network := o.Network
	if network == "" {
		network = str(r["network"])
	}
	args := []string{"run", "--rm", "--name", name, "--label", LOwner + "=" + owner, "--label", LHelper + "=" + str(r["uid"]), "--network", network, "--memory", "192m", "--pids-limit", "64", "--cpus", "1", "--security-opt", "no-new-privileges:true"}
	if o.Input != nil {
		args = append(args, "-i")
	}
	for _, k := range keys(JFromEnv(o.Env)) {
		args = append(args, "--env", k)
	}
	args = append(args, o.Extra...)
	args = append(args, "--entrypoint", cmd[0], str(r["resolvedImage"]))
	args = append(args, cmd[1:]...)
	redact := NewRedactor()
	for _, v := range o.Env {
		redact.Add(v)
	}
	o.Redact = redact
	o.Exact = o.Output == nil
	o.Limit = 1 << 20
	if o.Timeout == 0 {
		o.Timeout = time.Minute
	}
	result, e := i.Docker.Call(ctx, args, o.RunOptions)
	// Cleanup never uses the cancelled context; ownership and endpoint are checked.
	cleanCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := i.Verify(cleanCtx, r); err == nil {
		res, err := i.Docker.Runner.Run(cleanCtx, "docker", i.Docker.Args("inspect", name), RunOptions{Exact: true, Limit: 2 << 20})
		if err == nil && res.Code == 0 {
			var vs []J
			if json.Unmarshal([]byte(res.Stdout), &vs) == nil && len(vs) == 1 && str(at(vs[0], "Config", "Labels", LOwner)) == owner && str(at(vs[0], "Config", "Labels", LHelper)) == str(r["uid"]) {
				if _, err = i.Docker.Call(cleanCtx, []string{"rm", "-f", str(vs[0]["Id"])}, RunOptions{}); err != nil && e == nil {
					e = fail("HELPER_CLEANUP", "No se pudo limpiar el cliente temporal propio; revisa np-client.", 422)
				}
			}
		}
	}
	return result, e
}
func JFromEnv(env map[string]string) J {
	j := J{}
	for k, v := range env {
		j[k] = v
	}
	return j
}

func (i *Infrastructure) Render(r J) (string, error) {
	vault, e := i.Secrets(r)
	if e != nil {
		return "", e
	}
	dir := i.Dir(r)
	admin := filepath.Join(dir, "admin.secret")
	if e = atomicBytes(admin, []byte(str(vault["admin"])+"\n"), 0444); e != nil {
		return "", e
	}
	engine := str(r["engine"])
	major := imageMajor(str(r["requestedImage"]))
	target := "/data"
	if engine == "mysql" {
		target = "/var/lib/mysql"
	}
	if engine == "postgres" {
		target = "/var/lib/postgresql/data"
		if major >= 18 {
			target = "/var/lib/postgresql"
		}
	}
	volumes := A{}
	kind := str(at(r, "persistence", "kind"))
	if kind == "volume" {
		volumes = append(volumes, J{"type": "volume", "source": "data", "target": target})
	}
	if kind == "folder" {
		volumes = append(volumes, J{"type": "bind", "source": filepath.Join(str(at(r, "persistence", "path")), "data"), "target": target, "bind": J{"create_host_path": false}})
	}
	env := J{}
	command := A{}
	health := J{"interval": "3s", "timeout": "5s", "retries": 40, "start_period": "15s"}
	switch engine {
	case "postgres":
		env["POSTGRES_PASSWORD_FILE"] = "/run/secrets/admin"
		env["POSTGRES_INITDB_ARGS"] = "--auth-host=scram-sha-256"
		if major >= 18 {
			env["PGDATA"] = fmt.Sprintf("/var/lib/postgresql/%d/docker", major)
		}
		command = A{"postgres", "-c", "shared_buffers=64MB", "-c", "work_mem=4MB", "-c", "maintenance_work_mem=32MB", "-c", fmt.Sprintf("max_connections=%d", integer(r["maxConnections"]))}
		health["test"] = A{"CMD-SHELL", `PGPASSWORD=$(cat /run/secrets/admin) psql -X -h 127.0.0.1 -U postgres -d postgres -tAc "SELECT 1" | grep -qx 1`}
	case "mysql":
		env["MYSQL_ROOT_PASSWORD_FILE"] = "/run/secrets/admin"
		command = A{"mysqld", "--innodb-buffer-pool-size=134217728", "--performance-schema=OFF", "--partial-revokes=ON", fmt.Sprintf("--max-connections=%d", integer(r["maxConnections"]))}
		health = J{"test": A{"CMD-SHELL", `MYSQL_PWD=$(cat /run/secrets/admin) mysql --protocol=socket -uroot -Nse "SELECT 1" | grep -qx 1`}, "interval": "4s", "timeout": "8s", "retries": 45, "start_period": "30s"}
	case "redis":
		acl := []string{"user default off", "user npadmin on #" + hash(str(vault["admin"])) + " ~* &* +@all"}
		for _, raw := range arr(i.State()["databases"]) {
			d := obj(raw)
			if str(d["instanceUid"]) == str(r["uid"]) {
				secret := str(at(vault, "databases", str(d["id"]), "password"))
				if secret != "" {
					acl = append(acl, "user "+str(d["username"])+" on #"+hash(secret)+" ~* &* +@read +@write +@transaction +@scripting +@connection +@pubsub -@admin -@dangerous")
				}
			}
		}
		configDir := filepath.Join(dir, "config")
		if e = os.MkdirAll(configDir, 0755); e != nil {
			return "", e
		}
		if e = os.Chmod(configDir, 0755); e != nil {
			return "", e
		}
		if e = atomicBytes(filepath.Join(configDir, "users.acl"), []byte(strings.Join(acl, "\n")+"\n"), 0644); e != nil {
			return "", e
		}
		aof := "yes"
		if kind == "none" {
			aof = "no"
		}
		config := fmt.Sprintf("bind 0.0.0.0\nprotected-mode yes\ndatabases 1\nport 6379\naclfile /etc/nearprod/users.acl\nmaxmemory %dmb\nmaxmemory-policy noeviction\nappendonly %s\nappendfsync everysec\nsave \"\"\ndir /data\n", max(32, integer(r["memoryMiB"])/3), aof)
		if e = atomicBytes(filepath.Join(configDir, "redis.conf"), []byte(config), 0644); e != nil {
			return "", e
		}
		volumes = append(volumes, J{"type": "bind", "source": configDir, "target": "/etc/nearprod", "read_only": true, "bind": J{"create_host_path": false}})
		command = A{"redis-server", "/etc/nearprod/redis.conf"}
		health = J{"test": A{"CMD-SHELL", `REDISCLI_AUTH=$(cat /run/secrets/admin) redis-cli --user npadmin ping | grep -qx PONG`}, "interval": "3s", "timeout": "5s", "retries": 30, "start_period": "5s"}
	}
	svc := J{"image": r["resolvedImage"], "labels": J{LOwner: i.Store.Get()["owner"], LResource: r["uid"]}, "environment": env, "command": command, "volumes": volumes, "secrets": A{J{"source": "admin", "target": "admin"}}, "networks": J{"data": J{"aliases": A{r["hostname"]}}}, "restart": "no", "mem_limit": fmt.Sprintf("%dm", integer(r["memoryMiB"])), "pids_limit": 256, "healthcheck": health, "logging": J{"driver": "json-file", "options": J{"max-size": "5m", "max-file": "2"}}}
	if engine == "postgres" {
		svc["shm_size"] = "128m"
	}
	if kind == "none" {
		svc["tmpfs"] = A{"/data"}
	}
	if hp := integer(r["hostPort"]); hp > 0 {
		svc["ports"] = A{J{"target": at(engineOptions, engine, "port"), "published": fmt.Sprint(hp), "host_ip": "127.0.0.1", "protocol": "tcp"}}
	}
	model := J{"services": J{"database": svc}, "networks": J{"data": J{"external": true, "name": r["network"]}}, "secrets": J{"admin": J{"file": admin}}}
	if kind == "volume" {
		model["volumes"] = J{"data": J{"external": true, "name": r["volume"]}}
	}
	file := filepath.Join(dir, "compose.json")
	return file, writeCompose(file, model)
}
func (i *Infrastructure) Start(ctx context.Context, id string, line func(string, string)) (J, error) {
	r, e := i.Instance(id)
	if e != nil {
		return nil, e
	}
	if e = validateFolderPlatform(i.Store.Get(), r); e != nil {
		return nil, e
	}
	if _, e = i.Verify(ctx, r); e != nil {
		return nil, e
	}
	if _, e = i.Secrets(r); e != nil {
		return nil, e
	}
	if e = i.CheckStorageIdentity(ctx, r); e != nil {
		return nil, e
	}
	cs, e := i.Containers(ctx, r)
	if e != nil {
		return nil, e
	}
	for _, v := range cs {
		c := obj(v)
		if truth(r["initialized"]) && truth(c["running"]) && str(c["health"]) == "healthy" {
			return J{"instance": r["id"], "noOp": true, "healthy": true}, nil
		}
	}
	bound := false
	for _, v := range cs {
		c := obj(v)
		for _, raw := range arr(c["ports"]) {
			p := obj(raw)
			bound = bound || truth(c["running"]) && integer(p["port"]) == integer(r["hostPort"])
		}
	}
	if hp := integer(r["hostPort"]); hp > 0 && !bound {
		ports, e := i.Ports(ctx, J{"start": hp, "end": hp, "instance": id})
		if e != nil {
			return nil, e
		}
		if !truth(ports["dockerChecked"]) || !truth(obj(arr(ports["ports"])[0])["available"]) {
			return nil, fail("PORT_OCCUPIED", "Puerto de la instancia ocupado.", 409)
		}
	}
	r, e = i.PinImage(ctx, r, line)
	if e != nil {
		return nil, e
	}
	if e = i.CheckFolder(ctx, r, line); e != nil {
		return nil, e
	}
	if e = i.EnsureStorage(ctx, r, line); e != nil {
		return nil, e
	}
	file, e := i.Render(r)
	if e != nil {
		return nil, e
	}
	_, e = i.Docker.Call(ctx, []string{"compose", "--project-directory", i.Dir(r), "--project-name", str(r["projectName"]), "-f", file, "up", "--detach", "--no-build", "--pull", "never", "--wait", "--wait-timeout", "240"}, RunOptions{Timeout: 6 * time.Minute, Line: line})
	if e != nil {
		return nil, e
	}
	c, e := i.Container(ctx, r)
	if e != nil {
		return nil, e
	}
	if str(c["health"]) != "healthy" {
		return nil, fail("INSTANCE_NOT_READY", "El motor no está saludable; consulta los logs.", 422)
	}
	if str(r["engine"]) == "postgres" {
		if _, e = i.Admin(ctx, r, "REVOKE CONNECT ON DATABASE postgres FROM PUBLIC; REVOKE CONNECT ON DATABASE template1 FROM PUBLIC;", "postgres"); e != nil {
			return nil, e
		}
	}
	e = i.Store.Update(func(d J) error {
		return editInstance(d, str(r["uid"]), func(v J) error { v["initialized"] = true; v["startedAt"] = now(); return nil })
	})
	return J{"instance": r["id"], "healthy": true, "container": c["id"], "internalHost": r["hostname"], "internalPort": at(engineOptions, str(r["engine"]), "port"), "hostPort": r["hostPort"]}, e
}
func databaseSummaries(databases A) A {
	out := A{}
	for _, raw := range databases {
		db := obj(raw)
		out = append(out, J{"id": db["id"], "name": db["name"], "username": db["username"], "state": db["state"]})
	}
	return out
}
func (i *Infrastructure) inspectInstanceLifecycle(ctx context.Context, r J, databases A, requireStopped bool) (J, error) {
	if _, e := i.Verify(ctx, r); e != nil {
		return nil, e
	}
	if _, e := i.Secrets(r); e != nil {
		return nil, e
	}
	containers, e := i.Containers(ctx, r)
	if e != nil {
		return nil, e
	}
	containerSummary := A{}
	for _, raw := range containers {
		c := obj(raw)
		if requireStopped && (truth(c["running"]) || contains([]string{"restarting", "paused"}, str(c["state"]))) {
			return nil, fail("INSTANCE_RUNNING", "Detén la instancia y confirma su estado antes de archivarla.", 409)
		}
		containerSummary = append(containerSummary, J{"id": c["id"], "name": c["name"], "state": c["state"], "running": c["running"], "health": c["health"]})
	}
	owner, uid := str(i.Store.Get()["owner"]), str(r["uid"])
	resource := func(kind, name string) (J, error) {
		value, e := i.Docker.InspectNamed(ctx, kind, name)
		if e != nil {
			return nil, e
		}
		if value != nil && (str(at(value, "Labels", LOwner)) != owner || str(at(value, "Labels", LResource)) != uid) {
			return nil, fail("INFRA_OWNERSHIP", "Un recurso existente no pertenece a esta instancia.", 409)
		}
		return J{"name": name, "exists": value != nil}, nil
	}
	network, e := resource("network", str(r["network"]))
	if e != nil {
		return nil, e
	}
	resources := J{"network": network}
	switch str(at(r, "persistence", "kind")) {
	case "volume":
		volume, e := resource("volume", str(r["volume"]))
		if e != nil {
			return nil, e
		}
		if truth(r["initialized"]) && !truth(volume["exists"]) {
			return nil, fail("DATA_VOLUME_MISSING", "Desapareció el volumen inicializado. Recupera el almacenamiento antes de cambiar su ciclo de vida.", 409)
		}
		resources["volume"] = volume
	case "folder":
		path, e := dataDirectory(str(at(r, "persistence", "path")), i.Store.Home, false)
		if e != nil {
			return nil, e
		}
		marker, e := readJSON(filepath.Join(path, ".nearprod-resource.json"), 1<<20)
		if e != nil || str(marker["owner"]) != owner || str(marker["uid"]) != uid || str(marker["image"]) != str(r["requestedImage"]) {
			return nil, fail("DATA_MARKER", "Falta un marcador válido de la instancia.", 409)
		}
		st, e := os.Lstat(filepath.Join(path, "data"))
		if e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return nil, fail("DATA_MISSING", "El directorio de datos no existe o no es válido.", 409)
		}
		resources["folder"] = J{"path": path, "exists": true}
	}
	return J{"containers": containerSummary, "resources": resources, "databases": databaseSummaries(databases)}, nil
}
func (i *Infrastructure) ArchiveInstancePreview(ctx context.Context, id string) (J, error) {
	r, e := i.Instance(id)
	if e != nil {
		return nil, e
	}
	consumers := i.Consumers(r)
	if len(consumers) > 0 {
		return nil, detailed("INSTANCE_BOUND", "Desvincula todos los consumidores antes de archivar la instancia.", 409, J{"consumers": consumers})
	}
	for _, databaseRaw := range instanceDatabases(i.State(), str(r["uid"])) {
		if bindings := databaseLifecycleBindings(i.Store.Get(), str(obj(databaseRaw)["id"])); len(bindings) > 0 {
			return nil, detailed("INSTANCE_BOUND", "Restaura la aplicación archivada y desvincula su base antes de archivar la instancia.", 409, J{"bindings": bindings})
		}
	}
	for _, raw := range arr(i.State()["archivedDatabases"]) {
		if str(at(raw, "database", "instanceUid")) == str(r["uid"]) {
			return nil, fail("INSTANCE_ARCHIVED_DATABASES", "Restaura o purga las bases archivadas antes de archivar la instancia completa.", 409)
		}
	}
	databases := instanceDatabases(i.State(), str(r["uid"]))
	scope, e := i.inspectInstanceLifecycle(ctx, r, databases, true)
	if e != nil {
		return nil, e
	}
	recordDigest := hash(J{"instance": r, "databases": databases})
	preview := J{"lifecycleAction": "archive-instance", "instance": J{"id": r["id"], "uid": r["uid"], "name": r["name"], "engine": r["engine"], "requestedImage": r["requestedImage"], "persistence": copyJ(obj(r["persistence"])), "location": instanceLocation(r), "hostPort": r["hostPort"], "initialized": r["initialized"]}, "databases": scope["databases"], "containers": scope["containers"], "resources": scope["resources"], "recordDigest": recordDigest, "preserved": A{"contenedores y redes", "volúmenes o carpetas", "credenciales y datos", "imágenes Docker"}, "note": "Solo se archivará metadata de NearProd. No se ejecutarán comandos Docker ni se borrarán datos."}
	preview["fingerprint"] = hash(preview)
	return preview, nil
}
func (i *Infrastructure) ArchiveInstance(ctx context.Context, req J, line func(string, string)) (J, error) {
	if !truth(req["confirm"]) {
		return nil, fail("CONFIRM_REQUIRED", "Confirma archivar la instancia sin borrar recursos ni datos.", 409)
	}
	preview, e := i.ArchiveInstancePreview(ctx, str(req["instance"]))
	if e != nil {
		return nil, e
	}
	if str(preview["fingerprint"]) != str(req["fingerprint"]) {
		return nil, fail("PREVIEW_CHANGED", "La instancia cambió desde la revisión; vuelve a comprobarla.", 409)
	}
	uid := str(at(preview, "instance", "uid"))
	e = i.Store.Update(func(d J) error {
		infra := obj(d["infra"])
		var current J
		instances := A{}
		for _, raw := range arr(infra["instances"]) {
			r := obj(raw)
			if str(r["uid"]) == uid {
				current = copyJ(r)
			} else {
				instances = append(instances, r)
			}
		}
		if current == nil {
			return fail("INSTANCE_NOT_FOUND", "La instancia ya no está activa.", 409)
		}
		for _, raw := range arr(infra["bindings"]) {
			databaseID := str(obj(raw)["databaseId"])
			for _, dbRaw := range arr(infra["databases"]) {
				db := obj(dbRaw)
				if str(db["id"]) == databaseID && str(db["instanceUid"]) == uid {
					return fail("INSTANCE_BOUND", "La instancia recibió una vinculación nueva; vuelve a revisar.", 409)
				}
			}
		}
		databases, remaining := A{}, A{}
		for _, raw := range arr(infra["databases"]) {
			db := obj(raw)
			if str(db["instanceUid"]) == uid {
				databases = append(databases, copyJ(db))
			} else {
				remaining = append(remaining, db)
			}
		}
		if hash(J{"instance": current, "databases": databases}) != str(preview["recordDigest"]) {
			return fail("PREVIEW_CHANGED", "La metadata cambió desde la revisión; vuelve a comprobarla.", 409)
		}
		archived := J{"lifecycle": "archived", "archivedAt": now(), "instance": current, "databases": databases}
		archived["snapshotDigest"] = archivedSnapshotDigest(archived)
		infra["instances"] = instances
		infra["databases"] = remaining
		infra["archivedInstances"] = append(arr(infra["archivedInstances"]), archived)
		return nil
	})
	if e != nil {
		return nil, e
	}
	if line != nil {
		line("Instancia archivada del catálogo; runtime y datos preservados.", "stdout")
	}
	return J{"instance": at(preview, "instance", "id"), "archived": true, "dataPreserved": true, "runtimeChanged": false}, nil
}
func (i *Infrastructure) RestoreInstancePreview(ctx context.Context, id string) (J, error) {
	archived, e := i.ArchivedInstance(id)
	if e != nil {
		return nil, e
	}
	if archivedSnapshotDigest(archived) != str(archived["snapshotDigest"]) {
		return nil, fail("INFRA_ARCHIVE_DIGEST", "El snapshot archivado no coincide con su digest.", 409)
	}
	r := obj(archived["instance"])
	databases := list(archived["databases"])
	scope, e := i.inspectInstanceLifecycle(ctx, r, databases, false)
	if e != nil {
		return nil, e
	}
	candidate := i.Store.Get()
	infra := obj(candidate["infra"])
	remaining := A{}
	for _, raw := range arr(infra["archivedInstances"]) {
		if str(at(raw, "instance", "uid")) != str(r["uid"]) {
			remaining = append(remaining, raw)
		}
	}
	infra["archivedInstances"] = remaining
	infra["instances"] = append(arr(infra["instances"]), copyJ(r))
	infra["databases"] = append(arr(infra["databases"]), databases...)
	if e = validateState(candidate); e != nil {
		return nil, e
	}
	preview := J{"lifecycleAction": "restore-instance", "instance": J{"id": r["id"], "uid": r["uid"], "name": r["name"], "engine": r["engine"], "requestedImage": r["requestedImage"], "persistence": copyJ(obj(r["persistence"])), "location": instanceLocation(r), "hostPort": r["hostPort"], "initialized": r["initialized"]}, "databases": scope["databases"], "containers": scope["containers"], "resources": scope["resources"], "recordDigest": archived["snapshotDigest"], "note": "La metadata volverá a la lista activa. No se iniciará ni detendrá el runtime y no se modificarán datos."}
	preview["fingerprint"] = hash(preview)
	return preview, nil
}
func (i *Infrastructure) RestoreInstance(ctx context.Context, req J, line func(string, string)) (J, error) {
	if !truth(req["confirm"]) {
		return nil, fail("CONFIRM_REQUIRED", "Confirma restaurar la instancia en el catálogo.", 409)
	}
	preview, e := i.RestoreInstancePreview(ctx, str(req["instance"]))
	if e != nil {
		return nil, e
	}
	if str(preview["fingerprint"]) != str(req["fingerprint"]) {
		return nil, fail("PREVIEW_CHANGED", "La instancia archivada cambió desde la revisión.", 409)
	}
	uid := str(at(preview, "instance", "uid"))
	e = i.Store.Update(func(d J) error {
		infra := obj(d["infra"])
		var snapshot J
		remaining := A{}
		for _, raw := range arr(infra["archivedInstances"]) {
			value := obj(raw)
			if str(at(value, "instance", "uid")) == uid {
				snapshot = copyJ(value)
			} else {
				remaining = append(remaining, value)
			}
		}
		if snapshot == nil {
			return fail("ARCHIVED_INSTANCE_NOT_FOUND", "La instancia ya no está archivada.", 409)
		}
		if archivedSnapshotDigest(snapshot) != str(snapshot["snapshotDigest"]) || str(snapshot["snapshotDigest"]) != str(preview["recordDigest"]) {
			return fail("PREVIEW_CHANGED", "El snapshot cambió desde la revisión.", 409)
		}
		infra["archivedInstances"] = remaining
		infra["instances"] = append(arr(infra["instances"]), copyJ(obj(snapshot["instance"])))
		infra["databases"] = append(arr(infra["databases"]), list(snapshot["databases"])...)
		return nil
	})
	if e != nil {
		return nil, e
	}
	if line != nil {
		line("Instancia restaurada en el catálogo; runtime y datos sin cambios.", "stdout")
	}
	return J{"instance": at(preview, "instance", "id"), "restored": true, "dataPreserved": true, "runtimeChanged": false}, nil
}
func (i *Infrastructure) StopPreview(ctx context.Context, id string) (J, error) {
	r, e := i.Instance(id)
	if e != nil {
		return nil, e
	}
	cs, e := i.Containers(ctx, r)
	if e != nil {
		return nil, e
	}
	all, e := i.Docker.Containers(ctx, false)
	if e != nil {
		return nil, e
	}
	cons := i.Consumers(r)
	for _, v := range cons {
		b := obj(v)
		active := false
		for _, raw := range all {
			c := obj(raw)
			active = active || truth(c["running"]) && str(at(c, "labels", LStack)) == str(b["stackUid"]) && contains(ss(b["services"]), str(c["service"]))
		}
		b["active"] = active
	}
	containers := A{}
	for _, v := range cs {
		c := obj(v)
		containers = append(containers, J{"id": c["id"], "name": c["name"], "running": c["running"]})
	}
	return J{"instance": r["id"], "containers": containers, "consumers": cons, "fingerprint": hash(J{"uid": r["uid"], "containers": containers, "consumers": cons}), "warning": "Detener interrumpe consumidores y clientes externos; no borra datos."}, nil
}
func (i *Infrastructure) Stop(ctx context.Context, req J, line func(string, string)) (J, error) {
	if !truth(req["confirm"]) {
		return nil, fail("CONFIRM_REQUIRED", "Confirma detener la instancia.", 409)
	}
	pre, e := i.StopPreview(ctx, str(req["instance"]))
	if e != nil {
		return nil, e
	}
	if str(pre["fingerprint"]) != str(req["fingerprint"]) {
		return nil, fail("PREVIEW_CHANGED", "Cambió el alcance; revisa de nuevo.", 409)
	}
	for _, v := range arr(pre["consumers"]) {
		if truth(obj(v)["active"]) && !truth(req["allowActive"]) {
			return nil, fail("INSTANCE_IN_USE", "Confirma explícitamente interrumpir los consumidores activos.", 409)
		}
	}
	ids := []string{}
	for _, v := range arr(pre["containers"]) {
		c := obj(v)
		if truth(c["running"]) {
			ids = append(ids, str(c["id"]))
		}
	}
	if len(ids) > 0 {
		if _, e = i.Docker.Call(ctx, append([]string{"stop", "--time", "30"}, ids...), RunOptions{Timeout: time.Minute, Line: line}); e != nil {
			return nil, e
		}
	}
	return J{"instance": req["instance"], "stopped": ids, "dataPreserved": true}, nil
}

var _ io.Reader

// Verify initialized storage even when an existing container reports healthy.
// A missing mount must never be silently replaced by an empty instance.
func (i *Infrastructure) CheckStorageIdentity(ctx context.Context, r J) error {
	if !truth(r["initialized"]) {
		return nil
	}
	switch str(at(r, "persistence", "kind")) {
	case "volume":
		v, e := i.Docker.InspectNamed(ctx, "volume", str(r["volume"]))
		if e != nil {
			return e
		}
		if v == nil {
			return fail("DATA_VOLUME_MISSING", "Desapareció el volumen inicializado. No se creará una base vacía.", 409)
		}
		if str(at(v, "Labels", LOwner)) != str(i.Store.Get()["owner"]) || str(at(v, "Labels", LResource)) != str(r["uid"]) {
			return fail("INFRA_OWNERSHIP", "El volumen no pertenece a esta instancia.", 409)
		}
	case "folder":
		p, e := dataDirectory(str(at(r, "persistence", "path")), i.Store.Home, false)
		if e != nil {
			return e
		}
		m, e := readJSON(filepath.Join(p, ".nearprod-resource.json"), 1<<20)
		if e != nil || str(m["owner"]) != str(i.Store.Get()["owner"]) || str(m["uid"]) != str(r["uid"]) || str(m["image"]) != str(r["requestedImage"]) {
			return fail("DATA_MARKER", "Falta el marcador de la instancia; no se inicializará otra carpeta.", 409)
		}
		st, e := os.Lstat(filepath.Join(p, "data"))
		if e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return fail("DATA_MISSING", "Falta el directorio de datos inicializado. Recupera el almacenamiento antes de iniciar.", 409)
		}
	}
	return nil
}
