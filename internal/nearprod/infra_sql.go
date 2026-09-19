package nearprod

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const pgAdminScript = `export PGPASSWORD="$(cat /run/secrets/admin)"; exec psql -X --set ON_ERROR_STOP=1 --host 127.0.0.1 --username postgres --dbname "$1" --tuples-only --no-align`
const mysqlAdminScript = `export MYSQL_PWD="$(cat /run/secrets/admin)"; exec mysql --protocol=socket --user=root --batch --skip-column-names`
const redisAdminScript = `export REDISCLI_AUTH="$(cat /run/secrets/admin)"; exec redis-cli --user npadmin "$@"`

func (i *Infrastructure) redactor(r J) *Redactor {
	rd := NewRedactor()
	if vault, e := i.Secrets(r); e == nil {
		rd.Add(str(vault["admin"]))
		for _, v := range obj(vault["databases"]) {
			rd.Add(str(obj(v)["password"]))
		}
	}
	return rd
}
func (i *Infrastructure) Admin(ctx context.Context, r J, input, database string) (Result, error) {
	c, e := i.Container(ctx, r)
	if e != nil {
		return Result{}, e
	}
	script := pgAdminScript
	if str(r["engine"]) == "mysql" {
		script = mysqlAdminScript
	}
	return i.Docker.Call(ctx, []string{"exec", "-i", str(c["id"]), "sh", "-ec", script, "--", database}, RunOptions{Input: strings.NewReader(input), Redact: i.redactor(r), Exact: true, Limit: 1 << 20, Timeout: time.Minute})
}
func (i *Infrastructure) RedisAdmin(ctx context.Context, r J, args []string) (Result, error) {
	c, e := i.Container(ctx, r)
	if e != nil {
		return Result{}, e
	}
	return i.Docker.Call(ctx, append([]string{"exec", str(c["id"]), "sh", "-ec", redisAdminScript, "--"}, args...), RunOptions{Redact: i.redactor(r), Exact: true, Limit: 1 << 20})
}
func (i *Infrastructure) CreateDatabase(ctx context.Context, req J, line func(string, string)) (J, error) {
	if !truth(req["confirm"]) {
		return nil, fail("CONFIRM_REQUIRED", "Confirma crear la base y su usuario.", 409)
	}
	r, e := i.Instance(str(req["instance"]))
	if e != nil {
		return nil, e
	}
	name := str(req["name"])
	if !validDB(name) {
		return nil, fail("DATABASE_NAME", "Usa minúsculas, números y underscore, empezando con letra; hasta 40 caracteres.", 400)
	}
	var existing J
	for _, v := range arr(i.State()["databases"]) {
		d := obj(v)
		if str(d["instanceUid"]) == str(r["uid"]) && str(d["name"]) == name {
			existing = d
		}
	}
	if existing != nil && str(existing["state"]) == "ready" {
		if u := str(req["username"]); u != "" && u != str(existing["username"]) {
			return nil, fail("DATABASE_ACCOUNT", "La cuenta existente se conserva.", 409)
		}
		return J{"database": existing, "noOp": true}, nil
	}
	if str(r["engine"]) == "redis" {
		for _, v := range arr(i.State()["databases"]) {
			d := obj(v)
			if str(d["instanceUid"]) == str(r["uid"]) && (existing == nil || str(d["id"]) != str(existing["id"])) {
				return nil, fail("REDIS_DEDICATED", "Redis está dedicado a otra aplicación; crea otra instancia.", 409)
			}
		}
	}
	if _, e = i.Start(ctx, str(r["id"]), line); e != nil {
		return nil, e
	}
	r, _ = i.Instance(str(r["id"]))
	engine := str(r["engine"])
	id := engine + "-" + hash(str(r["uid"]) + ":" + name)[:16]
	if existing != nil {
		id = str(existing["id"])
	}
	username := text(req["username"], "np_"+hash(id)[:16])
	if existing != nil {
		if req["username"] != nil && username != str(existing["username"]) {
			return nil, fail("DATABASE_ACCOUNT", "No se cambia el usuario al reintentar.", 409)
		}
		username = str(existing["username"])
	}
	if !validUser(username) || engine == "mysql" && len(username) > 32 {
		return nil, fail("USER_NAME", "Usuario limitado no válido.", 400)
	}
	if existing == nil {
		for _, v := range arr(i.State()["databases"]) {
			d := obj(v)
			if str(d["instanceUid"]) == str(r["uid"]) && str(d["username"]) == username {
				return nil, fail("USER_DUPLICATE", "La cuenta ya está asignada a otra base.", 409)
			}
		}
		if engine != "redis" {
			qdb, quser := "SELECT datname FROM pg_database WHERE datname="+sqlString(name)+";", "SELECT rolname FROM pg_roles WHERE rolname="+sqlString(username)+";"
			if engine == "mysql" {
				qdb = "SELECT SCHEMA_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME=" + sqlString(name) + ";"
				quser = "SELECT User FROM mysql.user WHERE User=" + sqlString(username) + ";"
			}
			for _, q := range []string{qdb, quser} {
				res, e := i.Admin(ctx, r, q, "postgres")
				if e != nil {
					return nil, e
				}
				if strings.TrimSpace(res.Stdout) != "" {
					return nil, fail("DATABASE_EXISTS", "Base/cuenta externa ya existe: no se adoptará ni se cambiará su contraseña.", 409)
				}
			}
		}
		vault, e := i.Secrets(r)
		if e != nil {
			return nil, e
		}
		obj(vault["databases"])[id] = J{"password": secretHex()}
		if e = writeJSON(i.SecretFile(r), vault); e != nil {
			return nil, e
		}
		if e = i.Store.Update(func(d J) error {
			infra := obj(d["infra"])
			infra["databases"] = append(arr(infra["databases"]), J{"id": id, "instanceUid": r["uid"], "name": name, "username": username, "state": "pending", "createdAt": now()})
			return nil
		}); e != nil {
			return nil, e
		}
	}
	vault, e := i.Secrets(r)
	if e != nil {
		return nil, e
	}
	password := str(at(vault, "databases", id, "password"))
	if password == "" {
		return nil, fail("SECRETS_MISSING", "La credencial registrada no existe; no se regenerará.", 409)
	}
	provision := func() error {
		switch engine {
		case "postgres":
			res, e := i.Admin(ctx, r, "SELECT rolname FROM pg_roles WHERE rolname="+sqlString(username)+";", "postgres")
			if e != nil {
				return e
			}
			verb := "CREATE"
			if strings.TrimSpace(res.Stdout) != "" {
				verb = "ALTER"
			}
			_, e = i.Admin(ctx, r, verb+" ROLE "+pgID(username)+" LOGIN PASSWORD "+sqlString(password)+" NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;", "postgres")
			if e != nil {
				return e
			}
			res, e = i.Admin(ctx, r, "SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname="+sqlString(name)+";", "postgres")
			if e != nil {
				return e
			}
			owner := strings.TrimSpace(res.Stdout)
			if owner != "" && owner != username {
				return fail("DATABASE_OWNER", "La base existente tiene otro propietario.", 409)
			}
			if owner == "" {
				if _, e = i.Admin(ctx, r, "CREATE DATABASE "+pgID(name)+" OWNER "+pgID(username)+";", "postgres"); e != nil {
					return e
				}
			}
			if _, e = i.Admin(ctx, r, "REVOKE ALL ON DATABASE "+pgID(name)+" FROM PUBLIC; GRANT CONNECT, TEMPORARY ON DATABASE "+pgID(name)+" TO "+pgID(username)+";", "postgres"); e != nil {
				return e
			}
			_, e = i.Admin(ctx, r, "REVOKE CREATE ON SCHEMA public FROM PUBLIC; GRANT USAGE, CREATE ON SCHEMA public TO "+pgID(username)+";", name)
			return e
		case "mysql":
			q := "CREATE DATABASE IF NOT EXISTS " + myID(name) + " CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;\nCREATE USER IF NOT EXISTS " + sqlString(username) + "@'%' IDENTIFIED BY " + sqlString(password) + ";\nALTER USER " + sqlString(username) + "@'%' IDENTIFIED BY " + sqlString(password) + ";\nGRANT ALL PRIVILEGES ON " + myID(name) + ".* TO " + sqlString(username) + "@'%';"
			_, e := i.Admin(ctx, r, q, "")
			return e
		case "redis":
			if _, e := i.Render(r); e != nil {
				return e
			}
			res, e := i.RedisAdmin(ctx, r, []string{"ACL", "LOAD"})
			if e != nil {
				return e
			}
			if strings.TrimSpace(res.Stdout) != "OK" {
				return fail("REDIS_ACL", "No se cargó la cuenta Redis.", 422)
			}
		}
		return nil
	}
	e = provision()
	var probe J
	if e == nil {
		probe, e = i.Probe(ctx, id)
	}
	state := "ready"
	if e != nil {
		state = "failed"
	}
	storeErr := i.Store.Update(func(d J) error {
		for _, v := range arr(at(d, "infra", "databases")) {
			db := obj(v)
			if str(db["id"]) == id {
				db["state"] = state
				if e != nil {
					db["error"] = publicError(e)
				} else {
					delete(db, "error")
				}
			}
		}
		return nil
	})
	if e != nil {
		return nil, e
	}
	if storeErr != nil {
		return nil, storeErr
	}
	db, _ := i.Database(id)
	return J{"database": db, "probe": probe, "note": "Cuenta limitada creada; no se modifican .env ni migraciones del proyecto."}, nil
}
func (i *Infrastructure) Connection(id string, reveal bool) (J, error) {
	db, e := i.Database(id)
	if e != nil {
		return nil, e
	}
	r, e := i.Instance(str(db["instanceUid"]))
	if e != nil {
		return nil, e
	}
	vault, e := i.Secrets(r)
	if e != nil {
		return nil, e
	}
	password := str(at(vault, "databases", id, "password"))
	if password == "" {
		return nil, fail("SECRETS_MISSING", "Credencial ausente.", 409)
	}
	pw, uriPW := "••••••••", "<contraseña>"
	if reveal {
		pw, uriPW = password, password
	}
	engine := str(r["engine"])
	port := integer(at(engineOptions, engine, "port"))
	scheme := engine
	if scheme == "postgres" {
		scheme = "postgresql"
	}
	var database any = db["name"]
	if engine == "redis" {
		database = 0
	}
	uri := func(host string, p int) string {
		return fmt.Sprintf("%s://%s:%s@%s:%d/%s", scheme, str(db["username"]), uriPW, host, p, str(database))
	}
	internal := J{"host": r["hostname"], "port": port, "url": uri(str(r["hostname"]), port)}
	var local any
	if hp := integer(r["hostPort"]); hp > 0 {
		local = J{"host": "127.0.0.1", "port": hp, "url": uri("127.0.0.1", hp)}
	}
	return J{"databaseId": id, "engine": engine, "database": database, "user": db["username"], "password": pw, "internal": internal, "local": local, "revealed": reveal, "note": "Credenciales privadas no cifradas. Acceso interno para consumidores conectados; localhost dentro de un contenedor no es esta instancia."}, nil
}
func (i *Infrastructure) Client(ctx context.Context, db, r J, query string, redisArgs []string) (Result, error) {
	vault, e := i.Secrets(r)
	if e != nil {
		return Result{}, e
	}
	password := str(at(vault, "databases", str(db["id"]), "password"))
	if password == "" {
		return Result{}, fail("SECRETS_MISSING", "Falta la credencial.", 409)
	}
	engine := str(r["engine"])
	cmd := []string{}
	env := map[string]string{}
	var input io.Reader
	if engine == "postgres" {
		cmd = []string{"psql", "-X", "-h", str(r["hostname"]), "-U", str(db["username"]), "-d", str(db["name"]), "-v", "ON_ERROR_STOP=1", "-tA"}
		env["PGPASSWORD"] = password
		input = strings.NewReader(query)
	} else if engine == "mysql" {
		cmd = []string{"mysql", "--protocol=TCP", "--local-infile=0", "--host", str(r["hostname"]), "--user", str(db["username"]), "--database", str(db["name"]), "--batch", "--skip-column-names"}
		env["MYSQL_PWD"] = password
		input = strings.NewReader(query)
	} else {
		cmd = append([]string{"redis-cli", "-h", str(r["hostname"]), "--user", str(db["username"])}, redisArgs...)
		env["REDISCLI_AUTH"] = password
	}
	return i.Helper(ctx, r, cmd, HelperOptions{RunOptions: RunOptions{Input: input, Env: env}})
}
func (i *Infrastructure) Probe(ctx context.Context, id string) (J, error) {
	db, e := i.Database(id)
	if e != nil {
		return nil, e
	}
	r, e := i.Instance(str(db["instanceUid"]))
	if e != nil {
		return nil, e
	}
	if _, e = i.Container(ctx, r); e != nil {
		return nil, e
	}
	var res Result
	switch str(r["engine"]) {
	case "postgres":
		res, e = i.Client(ctx, db, r, "BEGIN; CREATE TEMP TABLE nearprod_connection_test(value int); INSERT INTO nearprod_connection_test VALUES (1); SELECT value FROM nearprod_connection_test; ROLLBACK;", nil)
	case "mysql":
		res, e = i.Client(ctx, db, r, "CREATE TEMPORARY TABLE nearprod_connection_test(value int); INSERT INTO nearprod_connection_test VALUES (1); SELECT value FROM nearprod_connection_test; DROP TEMPORARY TABLE nearprod_connection_test;", nil)
	case "redis":
		key, value := "np-test:"+token(8), token(8)
		res, e = i.Client(ctx, db, r, "", []string{"SET", key, value, "EX", "30"})
		if e == nil && strings.TrimSpace(res.Stdout) != "OK" {
			e = fail("CONNECTION_WRITE", "Redis no confirmó escritura.", 422)
		}
		if e == nil {
			res, e = i.Client(ctx, db, r, "", []string{"GET", key})
			if e == nil && strings.TrimSpace(res.Stdout) != value {
				e = fail("CONNECTION_READ", "Redis no devolvió el valor escrito.", 422)
			}
		}
		if e == nil {
			_, e = i.Client(ctx, db, r, "", []string{"DEL", key})
		}
		res.Stdout = "1"
	}
	if e != nil {
		return nil, e
	}
	found := false
	for _, v := range strings.Fields(res.Stdout) {
		found = found || v == "1"
	}
	if !found {
		return nil, fail("CONNECTION_RESULT", "No se confirmó lectura/escritura con la credencial.", 422)
	}
	return J{"database": id, "authenticated": true, "readWrite": true, "from": "temporary-container-on-data-network", "checkedAt": now(), "note": "Cliente temporal real en la red del motor. No comprueba las consultas de negocio del framework."}, nil
}

func (i *Infrastructure) BindingPreview(ctx context.Context, req J) (J, error) {
	stack, e := i.Store.Stack(str(req["target"]))
	if e != nil {
		return nil, e
	}
	db, e := i.Database(str(req["database"]))
	if e != nil {
		return nil, e
	}
	r, e := i.Instance(str(db["instanceUid"]))
	if e != nil {
		return nil, e
	}
	if str(db["state"]) != "ready" {
		return nil, fail("DATABASE_PENDING", "Termina de crear/comprobar la base.", 409)
	}
	mode := text(req["mode"], "dev")
	if at(stack, "modes", mode) == nil {
		return nil, fail("MODE_MISSING", "Modo no disponible.", 400)
	}
	services := ss(req["services"])
	if len(services) == 0 || len(services) > 16 || len(unique(services)) != len(services) {
		return nil, fail("BINDING_SERVICES", "Selecciona servicios consumidores distintos.", 400)
	}
	mapping := obj(req["mapping"])
	if len(mapping) == 0 {
		variable := "DATABASE_URL"
		if str(r["engine"]) == "redis" {
			variable = "REDIS_URL"
		}
		mapping = J{"url": variable}
	}
	mapping, e = normalizedMapping(mapping)
	if e != nil {
		return nil, e
	}
	resolved, e := i.Compose.Resolve(ctx, stack, mode)
	if e != nil {
		return nil, e
	}
	warnings := A{}
	for _, svc := range services {
		model := obj(at(resolved.Model, "services", svc))
		if len(model) == 0 || !svcRE.MatchString(svc) {
			return nil, fail("SERVICE_NOT_FOUND", "Consumidor no encontrado: "+svc, 404)
		}
		if model["network_mode"] != nil {
			return nil, fail("BINDING_NETWORK_MODE", "Consumidor usa network_mode; se necesitan redes Compose.", 409)
		}
		for _, v := range mapping {
			if obj(model["environment"])[str(v)] != nil {
				warnings = append(warnings, svc+": reemplazará la variable "+str(v))
			}
		}
	}
	if str(r["engine"]) == "redis" {
		for _, v := range arr(i.State()["bindings"]) {
			b := obj(v)
			if str(b["databaseId"]) == str(db["id"]) && str(b["stackUid"]) != str(stack["uid"]) {
				return nil, fail("REDIS_DEDICATED", "La credencial Redis pertenece a otra aplicación.", 409)
			}
		}
	}
	pre := J{"database": db["id"], "instance": r["id"], "target": stack["id"], "stackUid": stack["uid"], "mode": mode, "services": cloneStrings(services), "mapping": mapping, "network": r["network"], "host": r["hostname"], "warnings": warnings, "composeFingerprint": resolved.Preview["fingerprint"]}
	pre["fingerprint"] = hash(pre)
	pre["note"] = "Se conectarán SOLO estos servicios. Conserva tu DB propia; no migra datos ni cambia .env. Revisa/aprueba y usa Iniciar para aplicar."
	return pre, nil
}
func (i *Infrastructure) Bind(ctx context.Context, req J) (J, error) {
	if !truth(req["confirm"]) {
		return nil, fail("CONFIRM_REQUIRED", "Confirma vincular la base.", 409)
	}
	pre, e := i.BindingPreview(ctx, req)
	if e != nil {
		return nil, e
	}
	if str(pre["fingerprint"]) != str(req["fingerprint"]) {
		return nil, fail("PREVIEW_CHANGED", "Cambió la vinculación; revisa de nuevo.", 409)
	}
	id := "binding-" + hash(str(pre["stackUid"]) + ":" + str(pre["mode"]) + ":" + str(pre["database"]))[:16]
	// Preserve legacy binding IDs when the user edits the same relationship.
	for _, raw := range arr(i.State()["bindings"]) {
		b := obj(raw)
		if str(b["stackUid"]) == str(pre["stackUid"]) && str(b["mode"]) == str(pre["mode"]) && str(b["databaseId"]) == str(pre["database"]) {
			id = str(b["id"])
			break
		}
	}

	record := J{"id": id, "databaseId": pre["database"], "stackUid": pre["stackUid"], "mode": pre["mode"], "services": pre["services"], "mapping": pre["mapping"], "createdAt": now()}
	e = i.Store.Update(func(d J) error {
		infra := obj(d["infra"])
		bindings := A{}
		for _, v := range arr(infra["bindings"]) {
			if str(obj(v)["id"]) != id {
				bindings = append(bindings, v)
			}
		}
		infra["bindings"] = append(bindings, record)
		return editStack(d, str(pre["target"]), func(s J) error { s["trust"] = J{}; return nil })
	})
	return J{"binding": record, "note": "Guardado. Revisa/aprueba e inicia para aplicar; no basta Reiniciar."}, e
}
func (i *Infrastructure) Unbind(req J) (J, error) {
	if !truth(req["confirm"]) {
		return nil, fail("CONFIRM_REQUIRED", "Confirma desvincular sin borrar la base.", 409)
	}
	id := text(req["binding"], str(req["id"]))
	found := false
	e := i.Store.Update(func(d J) error {
		infra := obj(d["infra"])
		out := A{}
		for _, v := range arr(infra["bindings"]) {
			b := obj(v)
			if str(b["id"]) == id {
				found = true
				for _, s := range arr(d["stacks"]) {
					if str(obj(s)["uid"]) == str(b["stackUid"]) {
						obj(s)["trust"] = J{}
					}
				}
			} else {
				out = append(out, b)
			}
		}
		if !found {
			return fail("BINDING_NOT_FOUND", "Vinculación no encontrada.", 404)
		}
		infra["bindings"] = out
		return nil
	})
	return J{"removed": id, "note": "Base y datos conservados. Revisa/inicia el consumidor para retirar variables y redes generadas."}, e
}
func (i *Infrastructure) Plan(ctx context.Context, stack J, mode string, model J, redact *Redactor) (J, error) {
	services, networks := J{}, J{}
	summary, risks, blockers, bindings := A{}, A{}, A{}, A{}
	for _, raw := range arr(i.State()["bindings"]) {
		b := obj(raw)
		if str(b["stackUid"]) != str(stack["uid"]) || str(b["mode"]) != mode {
			continue
		}
		bindings = append(bindings, b)
		db, e := i.Database(str(b["databaseId"]))
		if e != nil {
			return nil, e
		}
		r, e := i.Instance(str(db["instanceUid"]))
		if e != nil {
			return nil, e
		}
		connection, e := i.Connection(str(db["id"]), true)
		if e != nil {
			return nil, e
		}
		if redact != nil {
			redact.Add(str(connection["password"]))
			redact.Add(str(at(connection, "internal", "url")))
		}
		values := J{"url": at(connection, "internal", "url"), "host": at(connection, "internal", "host"), "port": str(at(connection, "internal", "port")), "database": str(connection["database"]), "user": connection["user"], "password": connection["password"]}
		key := "np_data_" + str(r["uid"])
		if obj(model["networks"])[key] != nil {
			blockers = append(blockers, "El Compose usa una clave de red reservada de datos.")
		}
		networks[key] = J{"name": r["network"], "external": true}
		for _, name := range ss(b["services"]) {
			svc := obj(at(model, "services", name))
			if len(svc) == 0 {
				blockers = append(blockers, name+": consumidor ya no existe.")
				continue
			}
			if svc["network_mode"] != nil {
				blockers = append(blockers, name+": network_mode incompatible con la red de datos.")
				continue
			}
			current := obj(services[name])
			nets := networksOf(svc)
			for k, v := range obj(current["networks"]) {
				nets[k] = v
			}
			nets[key] = nil
			env := obj(current["environment"])
			for field, k := range obj(b["mapping"]) {
				varName := str(k)
				if old, exists := env[varName]; exists && old != values[field] {
					blockers = append(blockers, name+": variable "+varName+" colisiona entre vínculos.")
				}
				env[varName] = values[field]
			}
			services[name] = J{"networks": nets, "environment": env}
			risks = append(risks, name+": conexión a datos compartidos; red accesible por sus consumidores. No modifica ni apaga la DB propia del proyecto.")
		}
		if str(db["state"]) != "ready" {
			blockers = append(blockers, str(db["name"])+": base pendiente/fallida.")
		}
		summary = append(summary, J{"databaseId": db["id"], "name": db["name"], "instance": r["id"], "engine": r["engine"], "mode": mode, "services": b["services"], "network": r["network"], "host": r["hostname"], "port": at(connection, "internal", "port"), "variables": valuesOf(obj(b["mapping"])), "state": db["state"]})
	}
	return J{"services": services, "networks": networks, "summary": summary, "risks": risks, "blockers": blockers, "fingerprint": hash(J{"bindings": bindings, "summary": summary, "credentialsHash": hash(services)})}, nil
}
func valuesOf(v J) A {
	out := A{}
	for _, k := range keys(v) {
		out = append(out, v[k])
	}
	return out
}
func (i *Infrastructure) EnsureBindings(ctx context.Context, stack J, mode string, line func(string, string)) error {
	seen := map[string]bool{}
	for _, raw := range arr(i.State()["bindings"]) {
		b := obj(raw)
		if str(b["stackUid"]) != str(stack["uid"]) || str(b["mode"]) != mode {
			continue
		}
		db, e := i.Database(str(b["databaseId"]))
		if e != nil {
			return e
		}
		if str(db["state"]) != "ready" {
			return fail("DATABASE_PENDING", "La base vinculada no está lista.", 409)
		}
		uid := str(db["instanceUid"])
		if !seen[uid] {
			if _, e = i.Start(ctx, uid, line); e != nil {
				return e
			}
			seen[uid] = true
		}
	}
	return nil
}
func (i *Infrastructure) BindingCheck(ctx context.Context, id string) (J, error) {
	var b J
	for _, v := range arr(i.State()["bindings"]) {
		if str(obj(v)["id"]) == id {
			b = obj(v)
		}
	}
	if b == nil {
		return nil, fail("BINDING_NOT_FOUND", "Vinculación no encontrada.", 404)
	}
	stack, e := i.Store.StackUID(str(b["stackUid"]))
	if e != nil {
		return nil, e
	}
	db, e := i.Database(str(b["databaseId"]))
	if e != nil {
		return nil, e
	}
	r, e := i.Instance(str(db["instanceUid"]))
	if e != nil {
		return nil, e
	}
	cs, e := i.Docker.Owned(ctx, stack)
	if e != nil {
		return nil, e
	}
	targets, names := []string{}, []string{}
	for _, name := range ss(b["services"]) {
		found := false
		for _, v := range cs {
			c := obj(v)
			if str(c["service"]) == name && truth(c["running"]) {
				found = true
				if obj(c["networks"])[str(r["network"])] == nil {
					return nil, fail("CONSUMER_NETWORK", "Falta la red aplicada: revisa y usa Iniciar.", 409)
				}
				targets = append(targets, str(c["id"]))
				names = append(names, str(c["name"]))
			}
		}
		if !found {
			return nil, fail("CONSUMER_OFFLINE", "Inicia todos los consumidores seleccionados.", 409)
		}
	}
	conn, e := i.Connection(str(db["id"]), true)
	if e != nil {
		return nil, e
	}
	values := J{"url": at(conn, "internal", "url"), "host": at(conn, "internal", "host"), "port": str(at(conn, "internal", "port")), "database": str(conn["database"]), "user": conn["user"], "password": conn["password"]}
	response, e := i.Docker.Call(ctx, append([]string{"inspect"}, targets...), RunOptions{Exact: true, Limit: 8 << 20, Redact: i.redactor(r)})
	if e != nil {
		return nil, e
	}
	var raw []J
	if json.Unmarshal([]byte(response.Stdout), &raw) != nil {
		return nil, fail("DOCKER_RESPONSE", "Inspect no válido.", 503)
	}
	for _, v := range raw {
		env := J{}
		for _, s := range ss(at(v, "Config", "Env")) {
			k, val, ok := strings.Cut(s, "=")
			if ok {
				env[k] = val
			}
		}
		for field, k := range obj(b["mapping"]) {
			if str(env[str(k)]) != str(values[field]) {
				return nil, fail("CONSUMER_ENV", "Variables efectivas diferentes: revisa y usa Iniciar, no Reiniciar.", 409)
			}
		}
	}
	probe, e := i.Probe(ctx, str(db["id"]))
	if e != nil {
		return nil, e
	}
	return merge(probe, J{"binding": id, "consumers": names, "networkApplied": true, "environmentApplied": true}), nil
}

func streamHash(file string) (string, error) {
	f, e := os.Open(file)
	if e != nil {
		return "", e
	}
	defer f.Close()
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return "", e
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func (i *Infrastructure) Backup(ctx context.Context, req J) (J, error) {
	if !truth(req["confirm"]) {
		return nil, fail("CONFIRM_REQUIRED", "Confirma exportar un backup privado.", 409)
	}
	db, e := i.Database(str(req["database"]))
	if e != nil {
		return nil, e
	}
	r, e := i.Instance(str(db["instanceUid"]))
	if e != nil {
		return nil, e
	}
	engine := str(r["engine"])
	if engine == "redis" {
		return nil, fail("BACKUP_ENGINE", "El asistente lógico soporta PostgreSQL/MySQL; Redis requiere su procedimiento AOF/RDB.", 400)
	}
	dir := text(req["directory"], filepath.Join(i.Store.Home, "backups", "databases"))
	dir, e = filepath.Abs(expandHome(dir))
	if e != nil {
		return nil, e
	}
	if dir == i.Store.Home || within(filepath.Join(i.Store.Home, "databases"), dir) {
		return nil, fail("BACKUP_PATH", "Guarda el backup fuera de los datos activos del motor.", 409)
	}
	for _, raw := range arr(i.State()["instances"]) {
		r := obj(raw)
		if str(at(r, "persistence", "kind")) == "folder" && within(str(at(r, "persistence", "path")), dir) {
			return nil, fail("BACKUP_PATH", "El backup no puede escribirse dentro de datos activos.", 409)
		}
	}
	if e = safeOwnedDir(dir); e != nil {
		return nil, e
	}
	ext := "dump"
	if engine == "mysql" {
		ext = "sql"
	}
	file := filepath.Join(dir, fmt.Sprintf("%s-%d-%s.%s", str(db["name"]), time.Now().UnixMilli(), token(5), ext))
	partial := file + ".partial"
	f, e := os.OpenFile(partial, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	defer os.Remove(partial)
	c, e := i.Container(ctx, r)
	if e != nil {
		return nil, e
	}
	script := `export PGPASSWORD="$(cat /run/secrets/admin)"; exec pg_dump -h 127.0.0.1 -U postgres --format=custom --no-owner --no-privileges --dbname "$1"`
	if engine == "mysql" {
		script = `export MYSQL_PWD="$(cat /run/secrets/admin)"; exec mysqldump --protocol=socket -uroot --single-transaction --no-tablespaces --set-gtid-purged=OFF --skip-add-drop-table --skip-add-locks --skip-lock-tables "$1"`
	}
	if _, e = i.Docker.Call(ctx, []string{"exec", str(c["id"]), "sh", "-ec", script, "--", str(db["name"])}, RunOptions{Output: f, Timeout: 30 * time.Minute, Redact: i.redactor(r)}); e != nil {
		return nil, e
	}
	if e = f.Sync(); e != nil {
		return nil, e
	}
	st, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if st.Size() == 0 {
		return nil, fail("BACKUP_EMPTY", "La exportación está vacía.", 422)
	}
	if e = f.Close(); e != nil {
		return nil, e
	}
	if e = os.Rename(partial, file); e != nil {
		return nil, e
	}
	sum, e := streamHash(file)
	if e != nil {
		return nil, e
	}
	meta := J{"version": 1, "engine": engine, "image": r["requestedImage"], "database": db["name"], "createdAt": now(), "bytes": st.Size(), "sha256": sum, "format": ext}
	if e = writeJSON(file+".nearprod.json", meta); e != nil {
		return nil, detailed("BACKUP_METADATA", "El dump existe, pero no se pudo guardar el manifiesto; no se elimina el archivo.", 422, J{"file": file})
	}
	return J{"file": file, "metadata": meta, "note": "Exportación de una base. MySQL no incluye usuarios, rutinas/eventos o configuración; definers pueden requerir revisión. Ensaya la restauración en un destino nuevo."}, nil
}
func (i *Infrastructure) Restore(ctx context.Context, req J) (J, error) {
	if !truth(req["confirm"]) || !truth(req["trustedBackup"]) {
		return nil, fail("CONFIRM_REQUIRED", "Confirma backup confiable y destino vacío.", 409)
	}
	db, e := i.Database(str(req["database"]))
	if e != nil {
		return nil, e
	}
	r, e := i.Instance(str(db["instanceUid"]))
	if e != nil {
		return nil, e
	}
	engine := str(r["engine"])
	if engine == "redis" {
		return nil, fail("RESTORE_ENGINE", "Asistente de restauración SQL únicamente.", 400)
	}
	for _, v := range arr(i.State()["bindings"]) {
		if str(obj(v)["databaseId"]) == str(db["id"]) {
			return nil, fail("RESTORE_BOUND", "Usa una base nueva sin consumidores vinculados.", 409)
		}
	}
	file, e := canonical(str(req["file"]), nil, "file")
	if e != nil {
		return nil, e
	}
	meta, e := readJSON(file+".nearprod.json", 1<<20)
	if e != nil {
		return nil, e
	}
	ext := "dump"
	if engine == "mysql" {
		ext = "sql"
	}
	family := func(image string) string {
		_, tag, _ := strings.Cut(image, ":")
		parts := strings.Split(strings.Split(tag, "-")[0], ".")
		if engine == "mysql" && len(parts) >= 2 {
			return strings.Join(parts[:2], ".")
		}
		return parts[0]
	}
	if integer(meta["version"]) != 1 || str(meta["engine"]) != engine || str(meta["format"]) != ext || family(str(meta["image"])) != family(str(r["requestedImage"])) {
		return nil, fail("BACKUP_VERSION", "Requiere backup NearProd del mismo motor y familia.", 409)
	}
	// Snapshot the input before verification/use, eliminating path replacement races.
	if e = privateDir(filepath.Join(i.Store.Home, "config", "tmp")); e != nil {
		return nil, e
	}
	temp, e := os.CreateTemp(filepath.Join(i.Store.Home, "config", "tmp"), "restore-*")
	if e != nil {
		return nil, e
	}
	defer temp.Close()
	defer os.Remove(temp.Name())
	input, e := os.Open(file)
	if e != nil {
		return nil, e
	}
	h := sha256.New()
	n, e := io.Copy(io.MultiWriter(temp, h), io.LimitReader(input, 32<<30))
	_ = input.Close()
	if e != nil {
		return nil, e
	}
	if n != int64(num(meta["bytes"])) || hex.EncodeToString(h.Sum(nil)) != str(meta["sha256"]) {
		return nil, fail("BACKUP_CHECKSUM", "El backup no coincide con su manifiesto; no se importará.", 409)
	}
	if _, e = temp.Seek(0, io.SeekStart); e != nil {
		return nil, e
	}
	query := "SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname NOT LIKE 'pg_toast%' AND c.relkind IN ('r','p','v','m','S','f');"
	if engine == "mysql" {
		query = "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=" + sqlString(str(db["name"])) + ";"
	}
	res, e := i.Admin(ctx, r, query, str(db["name"]))
	if e != nil {
		return nil, e
	}
	if strings.TrimSpace(res.Stdout) != "0" {
		return nil, fail("RESTORE_NOT_EMPTY", "Destino contiene objetos: no se borrará ni sobrescribirá.", 409)
	}
	vault, e := i.Secrets(r)
	if e != nil {
		return nil, e
	}
	password := str(at(vault, "databases", str(db["id"]), "password"))
	var cmd []string
	env := map[string]string{}
	if engine == "postgres" {
		cmd = []string{"pg_restore", "--exit-on-error", "--no-owner", "--no-privileges", "--host", str(r["hostname"]), "--username", str(db["username"]), "--dbname", str(db["name"])}
		env["PGPASSWORD"] = password
	} else {
		cmd = []string{"mysql", "--protocol=TCP", "--local-infile=0", "--host", str(r["hostname"]), "--user", str(db["username"]), "--database", str(db["name"]), "--binary-mode"}
		env["MYSQL_PWD"] = password
	}
	_, e = i.Helper(ctx, r, cmd, HelperOptions{RunOptions: RunOptions{Input: temp, Env: env, Timeout: 30 * time.Minute}})
	return J{"database": db["id"], "restored": e == nil, "file": file, "note": "Restaurado como usuario limitado. Un fallo puede dejar objetos parciales; no hay rollback automático ni eliminación."}, e
}
