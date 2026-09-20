package nearprod

import (
	"context"
	"strings"
)

func (i *Infrastructure) ArchivedDatabase(id string) (J, error) {
	for _, raw := range arr(i.State()["archivedDatabases"]) {
		snapshot := obj(raw)
		database := obj(snapshot["database"])
		if str(database["id"]) == id {
			return snapshot, nil
		}
	}
	return nil, fail("ARCHIVED_DATABASE_NOT_FOUND", "Base archivada no encontrada.", 404)
}

func databaseSummary(database J) J {
	return J{"id": database["id"], "instanceUid": database["instanceUid"], "name": database["name"], "username": database["username"], "state": database["state"]}
}

func purgeDatabaseRecordDigest(database J) string {
	record := copyJ(database)
	delete(record, "state")
	delete(record, "error")
	delete(record, "purge")
	return hash(record)
}

func databaseLifecycleBindings(state J, databaseID string) A {
	out := A{}
	for _, raw := range arr(at(state, "infra", "bindings")) {
		binding := obj(raw)
		if str(binding["databaseId"]) == databaseID {
			out = append(out, copyJ(binding))
		}
	}
	for _, stackRaw := range arr(state["archivedStacks"]) {
		for _, raw := range arr(at(stackRaw, "bindings")) {
			binding := obj(raw)
			if str(binding["databaseId"]) == databaseID {
				out = append(out, copyJ(binding))
			}
		}
	}
	return out
}

func (i *Infrastructure) ArchiveDatabasePreview(id string) (J, error) {
	database, e := i.Database(id)
	if e != nil {
		return nil, e
	}
	if str(database["state"]) != "ready" {
		return nil, fail("DATABASE_STATE", "La base debe estar lista antes de archivarla.", 409)
	}
	r, e := i.Instance(str(database["instanceUid"]))
	if e != nil {
		return nil, e
	}
	bindings := databaseLifecycleBindings(i.Store.Get(), id)
	if len(bindings) > 0 {
		return nil, detailed("DATABASE_BOUND", "Desvincula todos los consumidores activos o archivados antes de gestionar la base.", 409, J{"bindings": bindings})
	}
	vault, e := i.Secrets(r)
	if e != nil {
		return nil, e
	}
	if str(at(vault, "databases", id, "password")) == "" {
		return nil, fail("SECRETS_MISSING", "La credencial registrada no existe; no se archivará la base.", 409)
	}
	preview := J{"lifecycleAction": "archive-database", "database": databaseSummary(database), "instance": J{"id": r["id"], "name": r["name"], "engine": r["engine"], "location": instanceLocation(r)}, "bindingCount": 0, "recordDigest": hash(database), "note": "Se archivará solo metadata. La base física, la cuenta y su credencial se conservarán."}
	preview["fingerprint"] = hash(preview)
	return preview, nil
}

func (i *Infrastructure) ArchiveDatabase(req J) (J, error) {
	if !truth(req["confirm"]) {
		return nil, fail("CONFIRM_REQUIRED", "Confirma archivar la base sin borrar datos.", 409)
	}
	preview, e := i.ArchiveDatabasePreview(str(req["database"]))
	if e != nil {
		return nil, e
	}
	if str(preview["fingerprint"]) != str(req["fingerprint"]) {
		return nil, fail("PREVIEW_CHANGED", "La base cambió desde la revisión.", 409)
	}
	id := str(at(preview, "database", "id"))
	e = i.Store.Update(func(state J) error {
		if len(databaseLifecycleBindings(state, id)) > 0 {
			return fail("DATABASE_BOUND", "La base recibió una vinculación; vuelve a revisar.", 409)
		}
		infra := obj(state["infra"])
		var current J
		remaining := A{}
		for _, raw := range arr(infra["databases"]) {
			database := obj(raw)
			if str(database["id"]) == id {
				current = copyJ(database)
			} else {
				remaining = append(remaining, database)
			}
		}
		if current == nil || hash(current) != str(preview["recordDigest"]) {
			return fail("PREVIEW_CHANGED", "La metadata de la base cambió desde la revisión.", 409)
		}
		snapshot := J{"lifecycle": "archived", "archivedAt": now(), "database": current}
		snapshot["snapshotDigest"] = archivedDatabaseDigest(snapshot)
		infra["databases"] = remaining
		infra["archivedDatabases"] = append(arr(infra["archivedDatabases"]), snapshot)
		return nil
	})
	if e != nil {
		return nil, e
	}
	return J{"database": id, "archived": true, "dataPreserved": true, "credentialPreserved": true}, nil
}

func (i *Infrastructure) RestoreDatabasePreview(id string) (J, error) {
	snapshot, e := i.ArchivedDatabase(id)
	if e != nil {
		return nil, e
	}
	if archivedDatabaseDigest(snapshot) != str(snapshot["snapshotDigest"]) {
		return nil, fail("DATABASE_ARCHIVE_DIGEST", "El snapshot de base no coincide con su digest.", 409)
	}
	database := obj(snapshot["database"])
	r, e := i.Instance(str(database["instanceUid"]))
	if e != nil {
		return nil, e
	}
	vault, e := i.Secrets(r)
	if e != nil {
		return nil, e
	}
	if str(at(vault, "databases", id, "password")) == "" {
		return nil, fail("SECRETS_MISSING", "La credencial archivada no existe.", 409)
	}
	candidate := i.Store.Get()
	infra := obj(candidate["infra"])
	remaining := A{}
	for _, raw := range arr(infra["archivedDatabases"]) {
		if str(at(raw, "database", "id")) != id {
			remaining = append(remaining, raw)
		}
	}
	infra["archivedDatabases"] = remaining
	infra["databases"] = append(arr(infra["databases"]), copyJ(database))
	if e = validateState(candidate); e != nil {
		return nil, e
	}
	preview := J{"lifecycleAction": "restore-database", "database": databaseSummary(database), "instance": J{"id": r["id"], "name": r["name"], "engine": r["engine"], "location": instanceLocation(r)}, "recordDigest": snapshot["snapshotDigest"], "note": "La metadata volverá al catálogo activo. No se iniciará el motor ni se modificarán datos."}
	preview["fingerprint"] = hash(preview)
	return preview, nil
}

func (i *Infrastructure) RestoreDatabase(req J) (J, error) {
	if !truth(req["confirm"]) {
		return nil, fail("CONFIRM_REQUIRED", "Confirma restaurar la base en el catálogo.", 409)
	}
	preview, e := i.RestoreDatabasePreview(str(req["database"]))
	if e != nil {
		return nil, e
	}
	if str(preview["fingerprint"]) != str(req["fingerprint"]) {
		return nil, fail("PREVIEW_CHANGED", "La base archivada cambió desde la revisión.", 409)
	}
	id := str(at(preview, "database", "id"))
	e = i.Store.Update(func(state J) error {
		infra := obj(state["infra"])
		var snapshot J
		remaining := A{}
		for _, raw := range arr(infra["archivedDatabases"]) {
			value := obj(raw)
			if str(at(value, "database", "id")) == id {
				snapshot = copyJ(value)
			} else {
				remaining = append(remaining, value)
			}
		}
		if snapshot == nil || archivedDatabaseDigest(snapshot) != str(preview["recordDigest"]) {
			return fail("PREVIEW_CHANGED", "El snapshot de base cambió desde la revisión.", 409)
		}
		infra["archivedDatabases"] = remaining
		infra["databases"] = append(arr(infra["databases"]), copyJ(obj(snapshot["database"])))
		return nil
	})
	if e != nil {
		return nil, e
	}
	return J{"database": id, "restored": true, "runtimeChanged": false, "dataPreserved": true}, nil
}

func (i *Infrastructure) PurgeDatabasePreview(ctx context.Context, req J) (J, error) {
	mode := str(req["mode"])
	if !contains([]string{"backup-purge", "purge"}, mode) {
		return nil, fail("PURGE_MODE", "Elige crear backup y purgar, o purgar sin backup.", 400)
	}
	database, e := i.Database(str(req["database"]))
	if e != nil {
		return nil, e
	}
	progress := obj(database["purge"])
	if str(database["state"]) == "purging" {
		if str(progress["mode"]) != mode {
			return nil, fail("PURGE_MODE", "La purga pendiente debe reanudarse con el mismo modo.", 409)
		}
	} else if str(database["state"]) != "ready" {
		return nil, fail("DATABASE_STATE", "La base debe estar lista para iniciar una purga.", 409)
	}
	r, e := i.Instance(str(database["instanceUid"]))
	if e != nil {
		return nil, e
	}
	if str(r["engine"]) == "redis" {
		return nil, fail("PURGE_ENGINE", "Redis no aísla datos por credencial; archiva la credencial o retira la instancia completa.", 409)
	}
	bindings := databaseLifecycleBindings(i.Store.Get(), str(database["id"]))
	if len(bindings) > 0 {
		return nil, detailed("DATABASE_BOUND", "Desvincula todos los consumidores activos o archivados antes de purgar.", 409, J{"bindings": bindings})
	}
	if _, e = i.Verify(ctx, r); e != nil {
		return nil, e
	}
	if _, e = i.Container(ctx, r); e != nil {
		return nil, e
	}
	vault, e := i.Secrets(r)
	if e != nil {
		return nil, e
	}
	if str(progress["phase"]) != "metadata" && str(at(vault, "databases", str(database["id"]), "password")) == "" {
		return nil, fail("SECRETS_MISSING", "La credencial registrada no existe; no se purgará la base.", 409)
	}
	phase := "initial"
	if len(progress) > 0 {
		phase = str(progress["phase"])
	}
	physical, e := i.verifyPurgePhysical(ctx, r, database, phase)
	if e != nil {
		return nil, e
	}
	recordDigest := purgeDatabaseRecordDigest(database)
	directory := req["directory"]
	backup := any(nil)
	if len(progress) > 0 {
		recordDigest = str(progress["recordDigest"])
		directory = progress["directory"]
		backup = progress["backup"]
	}
	preview := J{"lifecycleAction": "purge-database", "mode": mode, "database": databaseSummary(database), "instance": J{"id": r["id"], "name": r["name"], "engine": r["engine"], "location": instanceLocation(r)}, "bindingCount": 0, "physical": physical, "recordDigest": recordDigest, "directory": directory, "backup": backup, "purgePhase": progress["phase"], "irreversible": true, "backupRequired": mode == "backup-purge" && backup == nil, "note": "La purga elimina físicamente la base y su cuenta limitada. No afecta bases hermanas ni la instancia."}
	preview["fingerprint"] = hash(preview)
	return preview, nil
}

func (i *Infrastructure) PurgeDatabase(ctx context.Context, req J) (J, error) {
	if !truth(req["confirm"]) {
		return nil, fail("CONFIRM_REQUIRED", "Confirma la purga física de la base.", 409)
	}
	mode := str(req["mode"])
	if mode == "purge" && (!truth(req["acknowledgeDataLoss"]) || str(req["typedId"]) != str(req["database"])) {
		return nil, fail("PURGE_CONFIRMATION", "Confirma la pérdida de datos y escribe exactamente el ID de la base.", 409)
	}
	preview, e := i.PurgeDatabasePreview(ctx, req)
	if e != nil {
		return nil, e
	}
	if str(preview["fingerprint"]) != str(req["fingerprint"]) {
		return nil, fail("PREVIEW_CHANGED", "La base cambió desde la revisión.", 409)
	}
	database, _ := i.Database(str(req["database"]))
	progress := obj(database["purge"])
	backup := obj(progress["backup"])
	if len(progress) == 0 && mode == "backup-purge" {
		backup, e = i.Backup(ctx, J{"database": req["database"], "directory": req["directory"], "confirm": true})
		if e != nil {
			return nil, detailed("PURGE_BACKUP_FAILED", "El backup falló; no se purgó ningún dato.", 422, J{"cause": publicError(e)})
		}
		current, checkErr := i.PurgeDatabasePreview(ctx, req)
		if checkErr != nil || str(current["fingerprint"]) != str(preview["fingerprint"]) {
			return nil, detailed("PREVIEW_CHANGED", "La base cambió después del backup; no se purgó.", 409, J{"backup": backup})
		}
	}
	if len(progress) == 0 {
		e = i.Store.Update(func(state J) error {
			if len(databaseLifecycleBindings(state, str(database["id"]))) > 0 {
				return fail("DATABASE_BOUND", "La base recibió una vinculación durante la purga.", 409)
			}
			for _, raw := range arr(at(state, "infra", "databases")) {
				current := obj(raw)
				if str(current["id"]) != str(database["id"]) {
					continue
				}
				if purgeDatabaseRecordDigest(current) != str(preview["recordDigest"]) || str(current["state"]) != "ready" {
					return fail("PREVIEW_CHANGED", "La metadata cambió antes de iniciar la purga.", 409)
				}
				current["state"] = "purging"
				delete(current, "error")
				current["purge"] = J{"mode": mode, "phase": "database", "startedAt": now(), "recordDigest": preview["recordDigest"], "directory": preview["directory"], "backup": backup}
				return nil
			}
			return fail("DATABASE_NOT_FOUND", "La base ya no está activa.", 409)
		})
		if e != nil {
			return nil, e
		}
		database, _ = i.Database(str(req["database"]))
		progress = obj(database["purge"])
	}
	r, _ := i.Instance(str(database["instanceUid"]))
	advance := func(from, to string) error {
		return i.Store.Update(func(state J) error {
			for _, raw := range arr(at(state, "infra", "databases")) {
				current := obj(raw)
				if str(current["id"]) == str(database["id"]) {
					purge := obj(current["purge"])
					if str(current["state"]) != "purging" || str(purge["phase"]) != from || str(purge["recordDigest"]) != str(preview["recordDigest"]) || purgeDatabaseRecordDigest(current) != str(preview["recordDigest"]) {
						return fail("PREVIEW_CHANGED", "El progreso de la purga cambió.", 409)
					}
					purge["phase"] = to
					return nil
				}
			}
			return fail("DATABASE_NOT_FOUND", "La base ya no está activa.", 409)
		})
	}
	for {
		phase := str(progress["phase"])
		switch phase {
		case "database":
			if _, e = i.verifyPurgePhysical(ctx, r, database, phase); e != nil {
				return nil, detailed("PURGE_PARTIAL", "El estado físico cambió antes de eliminar la base. El progreso se conservó para revisión.", 422, J{"backup": backup, "phase": phase, "cause": publicError(e)})
			}
			query, adminDB := "DROP DATABASE IF EXISTS "+pgID(str(database["name"]))+" WITH (FORCE);", "postgres"
			if str(r["engine"]) == "mysql" {
				query, adminDB = "DROP DATABASE IF EXISTS "+myID(str(database["name"]))+";", ""
			}
			if _, e = i.Admin(ctx, r, query, adminDB); e != nil {
				return nil, detailed("PURGE_PARTIAL", "No se pudo completar la eliminación de la base. El progreso quedó guardado para reintentar.", 422, J{"backup": backup, "phase": phase, "cause": publicError(e)})
			}
			if e = advance("database", "account"); e != nil {
				return nil, detailed("PURGE_PARTIAL", "La base se eliminó, pero no se pudo guardar el progreso. El reintento comprobará el estado físico.", 422, J{"backup": backup, "phase": phase, "cause": publicError(e)})
			}
			progress["phase"] = "account"
		case "account":
			if _, e = i.verifyPurgePhysical(ctx, r, database, phase); e != nil {
				return nil, detailed("PURGE_PARTIAL", "El estado físico cambió antes de eliminar la cuenta. El progreso se conservó para revisión.", 422, J{"backup": backup, "phase": phase, "cause": publicError(e)})
			}
			query, adminDB := "DROP ROLE IF EXISTS "+pgID(str(database["username"]))+";", "postgres"
			if str(r["engine"]) == "mysql" {
				query, adminDB = "DROP USER IF EXISTS "+sqlString(str(database["username"]))+"@'%';", ""
			}
			if _, e = i.Admin(ctx, r, query, adminDB); e != nil {
				return nil, detailed("PURGE_PARTIAL", "La base se eliminó, pero la cuenta sigue pendiente. Corrige sus dependencias y reintenta.", 422, J{"backup": backup, "phase": phase, "cause": publicError(e)})
			}
			if e = advance("account", "metadata"); e != nil {
				return nil, detailed("PURGE_PARTIAL", "Los objetos físicos se eliminaron, pero no se pudo guardar el progreso. El reintento continuará con metadata.", 422, J{"backup": backup, "phase": phase, "cause": publicError(e)})
			}
			progress["phase"] = "metadata"
		case "metadata":
			if _, e = i.verifyPurgePhysical(ctx, r, database, phase); e != nil {
				return nil, detailed("PURGE_PARTIAL", "Los objetos físicos cambiaron antes de limpiar metadata. El progreso se conservó para revisión.", 422, J{"backup": backup, "phase": phase, "cause": publicError(e)})
			}
			goto cleanup
		default:
			return nil, fail("PURGE_STATE", "El progreso de la purga no es válido.", 409)
		}
	}

cleanup:
	vault, e := i.Secrets(r)
	if e != nil {
		return nil, detailed("PURGE_METADATA", "Los datos físicos se eliminaron, pero no se pudo leer el vault para finalizar la limpieza.", 422, J{"backup": backup})
	}
	originalVault := copyJ(vault)
	delete(obj(vault["databases"]), str(database["id"]))
	if e = writeJSON(i.SecretFile(r), vault); e != nil {
		return nil, detailed("PURGE_METADATA", "Los datos físicos se eliminaron, pero no se pudo limpiar la credencial.", 422, J{"backup": backup})
	}
	e = i.Store.Update(func(state J) error {
		if len(databaseLifecycleBindings(state, str(database["id"]))) > 0 {
			return fail("DATABASE_BOUND", "La base recibió una vinculación durante la purga.", 409)
		}
		infra := obj(state["infra"])
		databases := A{}
		found := false
		for _, raw := range arr(infra["databases"]) {
			current := obj(raw)
			if str(current["id"]) == str(database["id"]) {
				purge := obj(current["purge"])
				found = str(current["state"]) == "purging" && str(purge["phase"]) == "metadata" && str(purge["recordDigest"]) == str(preview["recordDigest"]) && purgeDatabaseRecordDigest(current) == str(preview["recordDigest"])
				continue
			}
			databases = append(databases, current)
		}
		if !found {
			return fail("PREVIEW_CHANGED", "La metadata cambió durante la purga física.", 409)
		}
		infra["databases"] = databases
		return nil
	})
	if e != nil {
		if rollbackErr := writeJSON(i.SecretFile(r), originalVault); rollbackErr != nil {
			return nil, detailed("PURGE_METADATA_ROLLBACK", "Los datos físicos se eliminaron y falló restaurar la credencial después de un error de catálogo.", 500, J{"backup": backup, "cause": publicError(e), "rollback": publicError(rollbackErr)})
		}
		restored, rollbackErr := readJSON(i.SecretFile(r), 4<<20)
		if rollbackErr != nil || hash(restored) != hash(originalVault) {
			return nil, detailed("PURGE_METADATA_ROLLBACK", "Los datos físicos se eliminaron y no se pudo verificar la restauración de la credencial.", 500, J{"backup": backup, "cause": publicError(e), "rollback": publicError(rollbackErr)})
		}
		return nil, detailed("PURGE_METADATA", "Los datos físicos se eliminaron, pero la metadata no pudo finalizarse.", 422, J{"backup": backup, "cause": publicError(e)})
	}
	return J{"database": database["id"], "purged": true, "backup": backup, "dataPreservedInBackup": mode == "backup-purge"}, nil
}

func (i *Infrastructure) verifyPurgeOwnership(ctx context.Context, instance, database J) (J, error) {
	return i.verifyPurgePhysical(ctx, instance, database, "initial")
}

func (i *Infrastructure) verifyPurgePhysical(ctx context.Context, instance, database J, phase string) (J, error) {
	name, username := str(database["name"]), str(database["username"])
	if str(instance["engine"]) == "postgres" {
		owner, e := i.Admin(ctx, instance, "SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname="+sqlString(name)+";", "postgres")
		if e != nil {
			return nil, e
		}
		databaseOwner := strings.TrimSpace(owner.Stdout)
		if databaseOwner != "" && databaseOwner != username {
			return nil, fail("DATABASE_OWNERSHIP", "La base física falta o ya no pertenece a la cuenta registrada; no se purgará.", 409)
		}
		role, e := i.Admin(ctx, instance, "SELECT rolname FROM pg_roles WHERE rolname="+sqlString(username)+";", "postgres")
		if e != nil {
			return nil, e
		}
		account := strings.TrimSpace(role.Stdout)
		if account != "" && account != username {
			return nil, fail("DATABASE_OWNERSHIP", "La cuenta física registrada no existe; no se purgará.", 409)
		}
		if phase == "initial" && (databaseOwner != username || account != username) {
			return nil, fail("DATABASE_OWNERSHIP", "La base o su cuenta física registrada no existe; no se purgará.", 409)
		}
		if contains([]string{"account", "metadata"}, phase) && databaseOwner != "" {
			return nil, fail("DATABASE_OWNERSHIP", "La base reapareció durante la purga; no se continuará automáticamente.", 409)
		}
		if phase == "metadata" && account != "" {
			return nil, fail("DATABASE_OWNERSHIP", "La cuenta reapareció durante la purga; no se limpiará metadata.", 409)
		}
		if account != "" {
			limited, e := i.Admin(ctx, instance, "SELECT count(*) FROM pg_roles WHERE rolname="+sqlString(username)+" AND rolcanlogin AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls;", "postgres")
			if e != nil {
				return nil, e
			}
			memberships, e := i.Admin(ctx, instance, "SELECT count(*) FROM pg_auth_members WHERE member=(SELECT oid FROM pg_roles WHERE rolname="+sqlString(username)+");", "postgres")
			if e != nil {
				return nil, e
			}
			otherDatabases, e := i.Admin(ctx, instance, "SELECT count(*) FROM pg_database WHERE datdba=(SELECT oid FROM pg_roles WHERE rolname="+sqlString(username)+") AND datname<>"+sqlString(name)+";", "postgres")
			if e != nil {
				return nil, e
			}
			if strings.TrimSpace(limited.Stdout) != "1" || strings.TrimSpace(memberships.Stdout) != "0" || strings.TrimSpace(otherDatabases.Stdout) != "0" {
				return nil, fail("DATABASE_OWNERSHIP", "La cuenta física adquirió privilegios, membresías o bases ajenas; no se purgará.", 409)
			}
		}
		return J{"databaseOwner": databaseOwner, "databaseExists": databaseOwner != "", "account": account, "accountExists": account != "", "limited": account != ""}, nil
	}

	schema, e := i.Admin(ctx, instance, "SELECT SCHEMA_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME="+sqlString(name)+";", "")
	if e != nil {
		return nil, e
	}
	databaseExists := strings.TrimSpace(schema.Stdout) == name
	accountName := username + "@%"
	account, e := i.Admin(ctx, instance, "SELECT CONCAT(User,'@',Host) FROM mysql.user WHERE User="+sqlString(username)+" AND Host='%';", "")
	if e != nil {
		return nil, e
	}
	accountExists := strings.TrimSpace(account.Stdout) == accountName
	if phase == "initial" && (!databaseExists || !accountExists) {
		return nil, fail("DATABASE_OWNERSHIP", "El esquema o la cuenta física registrada no existe; no se purgará.", 409)
	}
	if contains([]string{"account", "metadata"}, phase) && databaseExists {
		return nil, fail("DATABASE_OWNERSHIP", "El esquema reapareció durante la purga; no se continuará automáticamente.", 409)
	}
	if phase == "metadata" && accountExists {
		return nil, fail("DATABASE_OWNERSHIP", "La cuenta reapareció durante la purga; no se limpiará metadata.", 409)
	}
	if databaseExists && !accountExists {
		return nil, fail("DATABASE_OWNERSHIP", "El esquema existe pero su cuenta registrada falta; no se purgará.", 409)
	}
	targetGrant := false
	if accountExists {
		grants, e := i.Admin(ctx, instance, "SHOW GRANTS FOR "+sqlString(username)+"@'%';", "")
		if e != nil {
			return nil, e
		}
		targetScope := " ON " + myID(name) + ".* TO "
		for _, raw := range strings.Split(strings.TrimSpace(grants.Stdout), "\n") {
			line, upper := strings.TrimSpace(raw), strings.ToUpper(strings.TrimSpace(raw))
			if line == "" || strings.HasPrefix(upper, "GRANT USAGE ON *.* TO ") {
				continue
			}
			if strings.HasPrefix(upper, "GRANT ") && strings.Contains(line, targetScope) {
				targetGrant = true
				continue
			}
			return nil, fail("DATABASE_OWNERSHIP", "La cuenta física tiene privilegios fuera de la base registrada; no se purgará.", 409)
		}
	}
	if databaseExists && !targetGrant {
		return nil, fail("DATABASE_OWNERSHIP", "La cuenta física ya no está limitada a la base registrada; no se purgará.", 409)
	}
	return J{"schema": strings.TrimSpace(schema.Stdout), "databaseExists": databaseExists, "account": strings.TrimSpace(account.Stdout), "accountExists": accountExists, "limited": accountExists}, nil
}
