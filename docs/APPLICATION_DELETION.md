# Application deletion

The application cards expose **Eliminar aplicación y recursos** for active and archived applications. Archiving and the existing CLI `remove` command retain their metadata-only semantics.

The deletion dialog first requests a read-only inventory. It displays physical resources, independent/external resources that remain, and blockers. Confirmation requires the exact application ID, an explicit irreversible data-loss acknowledgement, and the current preview fingerprint. Cancelling the preview changes nothing.

## Scope

The cleanup stops and removes owned/adopted containers, application-owned Compose networks, local named and anonymous volumes, application-built images, exclusively bound PostgreSQL/MySQL databases and their limited accounts/credentials, generated overlays, routes, watch state, bindings, trust, and the active or archived catalog record. The original checkout is always preserved.

Resources are identified by verified Engine identity, container ownership/adoption, Compose project labels, immutable Docker IDs and creation/metadata digests. There is no name-prefix deletion, forced removal of shared resources, or global prune. Anonymous volume identities are captured before containers are removed. Local volumes backed by external driver options are blocked because removing the Docker volume does not establish that their underlying data was erased.

The independently managed database instance, Traefik, externally declared resources, base images, global build caches, independently created backups and operation history are not deleted. Code-mounted checkout files are preserved. Writable host folders outside the checkout block deletion because NearProd cannot establish their ownership. A shared logical database, a shared application-owned volume/image/network, or Redis infrastructure credentials also block complete deletion: the current Redis credential model does not isolate data by application. No successful deletion is reported for these blocked cases.

The initial review needs readable Compose definitions to identify explicitly external resources. A saved retry uses its original reviewed identities even if the checkout subsequently becomes unavailable.

## Failure and retry

`POST /api/stacks/purge-preview` accepts `{ "target": "group/application" }` and returns the inventory and fingerprint.

`POST /api/stacks/purge` accepts `target`, `fingerprint`, `confirm: true`, `typedId` equal to `target`, and `acknowledgeDataLoss: true`. It returns a tracked `purge-stack` operation; acceptance of the request is not completion. Existing authentication and request-origin rules apply.

Before removing resources, the service saves a digest-checked plan in `stackPurges`, withdraws routes and detaches the target's bindings. It retains the application record until a fresh inventory and resource-by-resource absence checks succeed. SQL cleanup uses the existing resumable database/account/metadata phases. A failure or cancellation retains the plan, blocks normal application mutations and exposes **Reintentar eliminación**. Infrastructure mutations that could invalidate a pending plan are blocked; start/stop, diagnostics and backup remain available.

A retry checks the original resource identities. It skips resources already absent and refuses replacements or newly discovered application-owned resources. The UI shows the failed resource/reason and a new review before retrying. These checks do not provide a transaction across Docker, SQL and the filesystem; data already deleted cannot be rolled back.

## Validation

The Go regression cases cover active/archived deletion, confirmation and stale previews, shared resources, Engine drift, partial cleanup and anonymous-volume recovery, replacement refusal, PostgreSQL/MySQL sibling preservation, SQL-phase recovery, generated-file changes and route cleanup.

An explicitly opt-in test exercises real Docker with a uniquely named disposable project and catalog, an already-local `alpine:3.20`, a built image, named/anonymous volumes, a network and generated overlay:

```sh
NEARPROD_PURGE_DOCKER=1 NEARPROD_PURGE_CONTEXT=colima \
  go test ./internal/nearprod -run '^TestStackPurgeLiveDocker$' -v -count=1 -timeout=120s
```

The browser fixture supports `NEARPROD_BROWSER_PURGE=1` alongside `NEARPROD_BROWSER_TEST=1`; it seeds active, archived and retryable applications using the Docker test double. This UI evidence is separate from the opt-in real Docker test.
