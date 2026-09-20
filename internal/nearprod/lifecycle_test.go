package nearprod

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplicationArchiveRestorePreservesBindingsAndReservations(t *testing.T) {
	f := newFixture(t, false)
	app := f.app(t, "suite", "api", nil)
	instance := f.createInstance(t, "postgres")
	database := createDB(t, f, instance, "suite_dev")
	request := J{"target": app["id"], "database": database["id"], "services": A{"api"}, "mode": "dev"}
	bindingPreview, e := f.S.Infra.BindingPreview(context.Background(), request)
	must(t, e)
	request["confirm"], request["fingerprint"] = true, bindingPreview["fingerprint"]
	_, e = f.S.Infra.Bind(context.Background(), request)
	must(t, e)
	f.S.Refresh(context.Background())
	_, e = f.S.Remove(context.Background(), str(app["id"]), true)
	expectCode(t, e, "STACK_BOUND")
	must(t, f.S.Store.Update(func(state J) error {
		return editStack(state, str(app["id"]), func(stack J) error {
			stack["proxyApplied"] = J{"mode": "dev", "routes": A{J{"host": "archive-test.localhost", "service": "api", "port": 8080}}}
			return nil
		})
	}))
	must(t, f.S.Proxy.SyncRoutes())

	preview, e := f.S.ArchiveStackPreview(str(app["id"]))
	must(t, e)
	result, e := f.S.ArchiveStack(context.Background(), J{"target": app["id"], "confirm": true, "fingerprint": preview["fingerprint"]})
	must(t, e)
	if !truth(result["archived"]) || truth(result["runtimeChanged"]) {
		t.Fatal(result)
	}
	_, e = f.S.Store.Stack(str(app["id"]))
	expectCode(t, e, "STACK_NOT_FOUND")
	if len(arr(f.S.Store.Get()["archivedStacks"])) != 1 || len(arr(at(f.S.Store.Get(), "infra", "bindings"))) != 0 {
		t.Fatal("application or bindings were not moved atomically")
	}
	routes, e := os.ReadFile(str(f.S.Proxy.Paths()["dynamic"]))
	must(t, e)
	if strings.Contains(string(routes), "archive-test.localhost") {
		t.Fatal("archived application remained in proxy routes")
	}
	corrupt := f.S.Store.Get()
	obj(at(arr(corrupt["archivedStacks"])[0], "stack"))["name"] = "Alterada"
	expectCode(t, validateState(corrupt), "STACK_ARCHIVE_DIGEST")
	_, e = f.S.Infra.ArchiveDatabasePreview(str(database["id"]))
	expectCode(t, e, "DATABASE_BOUND")
	_, e = f.S.DeleteGroupPreview("suite")
	expectCode(t, e, "GROUP_NOT_EMPTY")
	_, e = f.S.RemoveRootPreview(f.Root)
	expectCode(t, e, "ROOT_IN_USE")
	_, e = f.S.Register(J{"product": "suite", "slug": "api", "projectName": "suite-api", "path": filepath.Join(f.Root, "suite", "api"), "modes": J{"dev": J{"files": A{"compose.yaml"}}}}, false)
	expectCode(t, e, "STACK_DUPLICATE")

	restorePreview, e := f.S.RestoreStackPreview(str(app["id"]))
	must(t, e)
	result, e = f.S.RestoreStack(J{"target": app["id"], "confirm": true, "fingerprint": restorePreview["fingerprint"]})
	must(t, e)
	if !truth(result["restored"]) || len(arr(at(f.S.Store.Get(), "infra", "bindings"))) != 1 {
		t.Fatal("application restore lost bindings")
	}
	routes, e = os.ReadFile(str(f.S.Proxy.Paths()["dynamic"]))
	must(t, e)
	if !strings.Contains(string(routes), "archive-test.localhost") {
		t.Fatal("restored application route was not synchronized")
	}
}

func TestApplicationArchiveRequiresStoppedOwnedRuntime(t *testing.T) {
	f := newFixture(t, false)
	app := f.app(t, "running", "api", nil)
	f.S.Refresh(context.Background())
	stoppedPreview, e := f.S.ArchiveStackPreview(str(app["id"]))
	must(t, e)
	f.trust(t, str(app["id"]), "dev")
	assertPass(t, f.act(t, str(app["id"]), "up", J{"mode": "dev"}))
	_, e = f.S.Remove(context.Background(), str(app["id"]), true)
	expectCode(t, e, "STACK_RUNNING")
	_, e = f.S.ArchiveStack(context.Background(), J{"target": app["id"], "confirm": true, "fingerprint": stoppedPreview["fingerprint"]})
	expectCode(t, e, "STACK_RUNNING")
	_, e = f.S.ArchiveStackPreview(str(app["id"]))
	expectCode(t, e, "STACK_RUNNING")
}

func TestApplicationArchiveRejectsCancelledFreshObservation(t *testing.T) {
	f := newFixture(t, false)
	app := f.app(t, "cancelled", "api", nil)
	f.S.Refresh(context.Background())
	preview, e := f.S.ArchiveStackPreview(str(app["id"]))
	must(t, e)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = f.S.ArchiveStack(ctx, J{"target": app["id"], "confirm": true, "fingerprint": preview["fingerprint"]})
	expectCode(t, e, "REQUEST_CANCELLED")
	_, e = f.S.Store.Stack(str(app["id"]))
	must(t, e)
}

func TestApplicationRemoveRejectsCancelledFreshObservation(t *testing.T) {
	f := newFixture(t, false)
	app := f.app(t, "cancelled-remove", "api", nil)
	f.S.Refresh(context.Background())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e := f.S.Remove(ctx, str(app["id"]), true)
	expectCode(t, e, "REQUEST_CANCELLED")
	_, e = f.S.Store.Stack(str(app["id"]))
	must(t, e)
}

func TestDeleteEmptyGroupAndUnusedRootPreservesFilesystem(t *testing.T) {
	f := newFixture(t, false)
	_, e := f.S.Group(J{"id": "empty", "name": "Vacío"}, false)
	must(t, e)
	preview, e := f.S.DeleteGroupPreview("empty")
	must(t, e)
	_, e = f.S.DeleteGroup(J{"id": "empty", "confirm": true, "fingerprint": preview["fingerprint"]})
	must(t, e)
	_, e = f.S.DeleteGroupPreview("empty")
	expectCode(t, e, "GROUP_NOT_FOUND")

	unused := filepath.Join(f.Dir, "Unused")
	must(t, os.MkdirAll(unused, 0700))
	alias := filepath.Join(f.Dir, "UnusedAlias")
	must(t, os.Symlink(unused, alias))
	_, e = f.S.AddRoot(alias)
	must(t, e)
	rootPreview, e := f.S.RemoveRootPreview(alias)
	must(t, e)
	result, e := f.S.RemoveRoot(J{"root": alias, "confirm": true, "fingerprint": rootPreview["fingerprint"]})
	must(t, e)
	if truth(result["filesystemChanged"]) {
		t.Fatal("root removal reported a filesystem mutation")
	}
	if _, e = os.Stat(unused); e != nil {
		t.Fatal("root removal deleted the directory", e)
	}
}

func TestDatabaseArchiveRestoreAndPurgeModes(t *testing.T) {
	f := newFixture(t, false)
	instance := f.createInstance(t, "postgres")
	database := createDB(t, f, instance, "archive_db")
	before := len(f.F.History())
	preview, e := f.S.Infra.ArchiveDatabasePreview(str(database["id"]))
	must(t, e)
	_, e = f.S.Infra.ArchiveDatabase(J{"database": database["id"], "confirm": true, "fingerprint": preview["fingerprint"]})
	must(t, e)
	for _, call := range f.F.History()[before:] {
		if contains(call.Args, "exec") || contains(call.Args, "run") || contains(call.Args, "create") || contains(call.Args, "rm") {
			t.Fatalf("database archive changed runtime: %v", call.Args)
		}
	}
	_, e = f.S.Infra.Database(str(database["id"]))
	expectCode(t, e, "DATABASE_NOT_FOUND")
	corrupt := f.S.Store.Get()
	obj(at(arr(at(corrupt, "infra", "archivedDatabases"))[0], "database"))["name"] = "alterada"
	expectCode(t, validateState(corrupt), "DATABASE_ARCHIVE_DIGEST")
	_, e = f.S.Infra.CreateDatabase(context.Background(), J{"instance": instance["id"], "name": database["name"], "confirm": true}, nil)
	expectCode(t, e, "DATABASE_RESERVED")
	_, e = f.S.Infra.ArchiveInstancePreview(context.Background(), str(instance["id"]))
	expectCode(t, e, "INSTANCE_ARCHIVED_DATABASES")
	restorePreview, e := f.S.Infra.RestoreDatabasePreview(str(database["id"]))
	must(t, e)
	_, e = f.S.Infra.RestoreDatabase(J{"database": database["id"], "confirm": true, "fingerprint": restorePreview["fingerprint"]})
	must(t, e)

	withoutBackup := createDB(t, f, instance, "purge_now")
	purgePreview, e := f.S.Infra.PurgeDatabasePreview(context.Background(), J{"database": withoutBackup["id"], "mode": "purge"})
	must(t, e)
	_, e = f.S.Infra.PurgeDatabase(context.Background(), J{"database": withoutBackup["id"], "mode": "purge", "confirm": true, "fingerprint": purgePreview["fingerprint"]})
	expectCode(t, e, "PURGE_CONFIRMATION")
	result, e := f.S.Infra.PurgeDatabase(context.Background(), J{"database": withoutBackup["id"], "mode": "purge", "confirm": true, "fingerprint": purgePreview["fingerprint"], "typedId": withoutBackup["id"], "acknowledgeDataLoss": true})
	must(t, e)
	if !truth(result["purged"]) || truth(result["dataPreservedInBackup"]) {
		t.Fatal(result)
	}
	_, e = f.S.Infra.Database(str(withoutBackup["id"]))
	expectCode(t, e, "DATABASE_NOT_FOUND")

	withBackup := createDB(t, f, instance, "purge_backup")
	directory := filepath.Join(f.Dir, "backups")
	purgePreview, e = f.S.Infra.PurgeDatabasePreview(context.Background(), J{"database": withBackup["id"], "mode": "backup-purge", "directory": directory})
	must(t, e)
	result, e = f.S.Infra.PurgeDatabase(context.Background(), J{"database": withBackup["id"], "mode": "backup-purge", "directory": directory, "confirm": true, "fingerprint": purgePreview["fingerprint"]})
	must(t, e)
	backup := obj(result["backup"])
	if !truth(result["purged"]) || !truth(result["dataPreservedInBackup"]) || str(backup["file"]) == "" {
		t.Fatal(result)
	}
	if _, e = os.Stat(str(backup["file"])); e != nil {
		t.Fatal("verified backup missing", e)
	}
	if _, e = os.Stat(str(backup["file"]) + ".nearprod.json"); e != nil {
		t.Fatal("backup manifest missing", e)
	}

	failedBackup := createDB(t, f, instance, "backup_must_pass")
	badDirectory := filepath.Join(f.Home, "databases", "inside-active-data")
	badPreview, e := f.S.Infra.PurgeDatabasePreview(context.Background(), J{"database": failedBackup["id"], "mode": "backup-purge", "directory": badDirectory})
	must(t, e)
	_, e = f.S.Infra.PurgeDatabase(context.Background(), J{"database": failedBackup["id"], "mode": "backup-purge", "directory": badDirectory, "confirm": true, "fingerprint": badPreview["fingerprint"]})
	expectCode(t, e, "PURGE_BACKUP_FAILED")
	_, e = f.S.Infra.Database(str(failedBackup["id"]))
	must(t, e)
}

func TestDatabasePurgeRejectsRedisAndBindings(t *testing.T) {
	f := newFixture(t, false)
	redis := f.createInstance(t, "redis")
	credential := createDB(t, f, redis, "cache_app")
	_, e := f.S.Infra.PurgeDatabasePreview(context.Background(), J{"database": credential["id"], "mode": "purge"})
	expectCode(t, e, "PURGE_ENGINE")
}

func TestDatabasePurgeRejectsPhysicalOwnershipDrift(t *testing.T) {
	f := newFixture(t, false)
	instance := f.createInstance(t, "postgres")
	database := createDB(t, f, instance, "owned_dev")
	f.F.DBs[str(database["name"])] = "foreign_owner"
	_, e := f.S.Infra.PurgeDatabasePreview(context.Background(), J{"database": database["id"], "mode": "purge"})
	expectCode(t, e, "DATABASE_OWNERSHIP")
	if f.F.DBs[str(database["name"])] != "foreign_owner" {
		t.Fatal("ownership drift must not execute DROP DATABASE")
	}
}

func TestDatabasePurgeResumesAfterDatabaseDroppedButRoleFailed(t *testing.T) {
	f := newFixture(t, false)
	instance := f.createInstance(t, "postgres")
	database := createDB(t, f, instance, "partial_purge")
	req := J{"database": database["id"], "mode": "purge", "acknowledgeDataLoss": true, "typedId": database["id"]}
	preview, e := f.S.Infra.PurgeDatabasePreview(context.Background(), req)
	must(t, e)
	req["confirm"], req["fingerprint"] = true, preview["fingerprint"]
	f.F.SQLFailContains = "DROP ROLE"
	_, e = f.S.Infra.PurgeDatabase(context.Background(), req)
	expectCode(t, e, "PURGE_PARTIAL")
	current, e := f.S.Infra.Database(str(database["id"]))
	must(t, e)
	if str(current["state"]) != "purging" || str(at(current, "purge", "phase")) != "account" {
		t.Fatal(current)
	}
	if _, exists := f.F.DBs[str(database["name"])]; exists || f.F.Roles[str(database["username"])] == "" {
		t.Fatal("fixture did not preserve the expected partial physical state")
	}

	retryPreview, e := f.S.Infra.PurgeDatabasePreview(context.Background(), req)
	must(t, e)
	req["fingerprint"] = retryPreview["fingerprint"]
	result, e := f.S.Infra.PurgeDatabase(context.Background(), req)
	must(t, e)
	if !truth(result["purged"]) || f.F.Roles[str(database["username"])] != "" {
		t.Fatal(result)
	}
	_, e = f.S.Infra.Database(str(database["id"]))
	expectCode(t, e, "DATABASE_NOT_FOUND")
}

func TestDatabasePurgeRevalidatesBetweenPhysicalPhases(t *testing.T) {
	f := newFixture(t, false)
	instance := f.createInstance(t, "postgres")
	database := createDB(t, f, instance, "purge_race")
	req := J{"database": database["id"], "mode": "purge", "acknowledgeDataLoss": true, "typedId": database["id"]}
	preview, e := f.S.Infra.PurgeDatabasePreview(context.Background(), req)
	must(t, e)
	req["confirm"], req["fingerprint"] = true, preview["fingerprint"]
	f.F.AfterDropDatabase = func() {
		f.F.AfterDropDatabase = nil
		f.F.DBs["foreign_db"] = str(database["username"])
	}
	_, e = f.S.Infra.PurgeDatabase(context.Background(), req)
	expectCode(t, e, "PURGE_PARTIAL")
	if f.F.Roles[str(database["username"])] == "" {
		t.Fatal("account was removed after acquiring another database")
	}
	current, e := f.S.Infra.Database(str(database["id"]))
	must(t, e)
	if str(at(current, "purge", "phase")) != "account" {
		t.Fatal(current)
	}
}

func TestMySQLDatabasePurgeVerifiesLimitedAccount(t *testing.T) {
	f := newFixture(t, false)
	instance := f.createInstance(t, "mysql")
	database := createDB(t, f, instance, "mysql_purge")
	req := J{"database": database["id"], "mode": "purge", "acknowledgeDataLoss": true, "typedId": database["id"]}
	preview, e := f.S.Infra.PurgeDatabasePreview(context.Background(), req)
	must(t, e)
	req["confirm"], req["fingerprint"] = true, preview["fingerprint"]
	_, e = f.S.Infra.PurgeDatabase(context.Background(), req)
	must(t, e)
	if _, ok := f.F.DBs[str(database["name"])]; ok {
		t.Fatal("mysql schema was not purged")
	}
	if f.F.Roles[str(database["username"])] != "" {
		t.Fatal("mysql limited account was not purged")
	}
}

func TestCLIPurgeWithoutBackupRequiresIndependentAcknowledgement(t *testing.T) {
	a, e := parseCLI([]string{"infra", "purge-database", "--database", "db-123", "--without-backup", "--yes"})
	must(t, e)
	_, e = cliInfra(context.Background(), a, nil, nil, nil, func(string, string, J) (any, error) {
		t.Fatal("preview must not run without independent acknowledgement")
		return nil, nil
	}, nil, nil)
	expectCode(t, e, "PURGE_CONFIRMATION")

	a, e = parseCLI([]string{"infra", "purge-database", "--database", "db-123", "--without-backup", "--acknowledge-data-loss", "db-123", "--yes"})
	must(t, e)
	called := false
	_, e = cliInfra(context.Background(), a, nil, nil, nil, func(_, _ string, req J) (any, error) {
		called = true
		if str(req["typedId"]) != "db-123" || !truth(req["acknowledgeDataLoss"]) {
			t.Fatal(req)
		}
		return J{}, nil
	}, nil, nil)
	must(t, e)
	if !called {
		t.Fatal("preview was not called with explicit acknowledgement")
	}
}
