# NearProd agent guide

NearProd manages a single user's local environment. Use the selected NEARPROD_HOME; all tools call its loopback agent, the sole writer. Tool availability is not user authorization. Never expose agent tokens, vaults, .env values, revealed credentials or private backups.

## Discover the contract

1. Call nearprod_guide (this document) and nearprod_capabilities. Filter capabilities by area and operation to obtain exact request schemas, confirmation rules and result expectations. No repository source inspection is needed to learn these contracts.
2. Call nearprod_overview and, when prerequisites or memory are uncertain, nearprod_diagnostics with operation doctor or metrics. Use actual host capabilities; do not assume Colima exists on every platform.
3. Tools with an operation selector receive {"operation":"...","parameters":{...}}. Existing named application/group tools retain their own input format. nearprod_api is an advanced JSON bridge: routes omit /api, GET parameters use the query, POST uses body (use {} for an empty request). Consult capabilities before calling it.
4. Tools return {"result":...}. Domain failures set isError and return result.error with code, message and optional details. Inspect details, not just the short message. Invalid schemas are MCP argument errors before dispatch.

## Compose compatibility and prerequisites

A Compose file accepted by Docker is not automatically compatible with NearProd. NearProd requires Docker Compose V2 or later, a supported configured Engine, a registered existing source directory, and explicitly selected Compose files for each mode. Tool maintenance belongs to the user-authorized maintenance workflow.

- Register the containing root using nearprod_roots/add. Discover files using nearprod_discover and inspect nearprod_project_options. These are offline hints, not a successful build or runtime test. Project options do not read .env values.
- The application has a stable group (product), application slug, absolute path and unique Docker projectName. The target ID is product/slug. Preserve an existing Docker identity; inspect adoption before claiming ownership.
- A dev mode is required. Each mode needs 1–16 distinct Compose files in merge order, up to 16 envFiles and 32 profiles. Paths must resolve within registered roots. Omitted envFiles allows automatic .env handling. Compose service env_file is a separate mechanism; ensure referenced local files exist. Do not invent missing variable values or credentials.
- NearProd rejects include, extends and escaped YAML keys for complete fingerprint review. List explicit compatible -f files or propose a compatible override/flattened configuration, then validate it. Files are limited to 2 MiB each. Do not silently remove required services or change application semantics to bypass validation.
- Development permits source bind mounts and development servers. Image verification (verify) blocks bind mounts, development commands and develop/watch. Build a suitable image/configuration for that mode; do not claim dev is equivalent to production.
- Real Compose resolution is performed by nearprod_application_review. Fix syntax, missing interpolation variables, invalid paths, service/profile errors and reported blockers before approval. Build/runtime dependencies must still be tested after resolution succeeds.
- Local HTTP access is optional. Routes need a unique *.localhost host, actual Compose service and internal container HTTP port; omit routes for workers/databases. Configure verify service/port separately when different. The process must listen on a container-reachable interface. Proxy/binding integration requires compatible Compose networks; follow actual review errors. A group does not connect applications or merge their Compose files.
- Healthchecks are optional but running is not ready. Use a real application readiness signal supported by tools present in the image. Missing checks must be reported as unconfigured, not healthy. HTTP proxy access, container health and authenticated database connectivity are separate checks.
- Compose Watch is optional and requires develop.watch. Source mounts and development servers can already provide reload without Watch.

## Diagnose, explain, edit, validate, apply

1. Inspect the existing files and NearProd metadata. Explain what is missing, why it matters and the smallest intended correction before editing.
2. Use existing user authorization. Ask only for a material unresolved choice, new scope or destructive consequences. Never infer consent from a preview or from an available tool.
3. Use the agent client's own filesystem tools to create/edit Compose, Dockerfile or .env templates. NearProd MCP configures catalog metadata; it is not a general file editor. If the client lacks filesystem access, say so and provide the exact file/change needed. Do not claim it was written.
4. Prefer a small local Compose override when it preserves the project's intended configuration. Select it after the base file in the relevant mode. Preserve unrelated changes, source code, service identities and data; never overwrite a user's .env or disclose its values. Do not change the original application merely to satisfy a warning without understanding it.
5. Inspect project options again as needed, register/edit the complete definition, then call application_review. Registration does not run Docker; edit replaces the full definition, so retain routes, links and modes that should remain.
6. Explain warnings, risks and blockers. Approve only the exact reviewed fingerprint within authorized scope. allowUnsafe accepts explicit non-blocking risks; it cannot bypass blockers. PREVIEW_CHANGED means inspect the new preview and obtain any newly required authorization; never silently substitute a fresh fingerprint into an old approval.
7. Call application_action up to start or apply changed configuration. rebuild forces a build. Select mode/service deliberately; confirmMode authorizes a mode switch and startRuntime authorizes starting the runtime. The web may disable redundant buttons, but MCP callers must inspect observed state before deciding an action is needed.
8. Save the returned operation id (field id), poll nearprod_operation until terminal state, and inspect error/results. Operation success is not proof of application readiness. Refresh observed status, check configured health, and check local HTTP access or database bindings as appropriate. Report untested business behavior and missing healthchecks accurately.

### Registration example

Replace the illustrative paths, IDs, services and ports with observed values. First register /absolute/projects as a root and inspect the actual project. This definition does not create Compose files:

```json
{
  "action": "register",
  "definition": {
    "product": "example",
    "slug": "api",
    "name": "Example API",
    "path": "/absolute/projects/example-api",
    "projectName": "example-api",
    "modes": {
      "dev": {"files": ["compose.yaml", "compose.nearprod.yaml"], "envFiles": [], "profiles": []}
    },
    "routes": [{"host": "api.example.localhost", "service": "api", "port": 8000}]
  }
}
```

Then review {"target":"example/api","mode":"dev"}, approve the returned fingerprint if authorized, run up, poll its id, and inspect health/access. Do not select an override that has not actually been created.

## Operations matching the web

- Applications: registration/edit and atomic batch registration, review/trust, group/application start/stop/restart/rebuild, existing-resource adoption, Watch, archive/restore, full deletion and bounded logs. nearprod_applications exposes batch/adoption/Watch; nearprod_application_lifecycle handles archive/restore/remove/purge.
- Roots and groups: discovery scope and organizational metadata; deletion requires no dependents and a reviewed fingerprint.
- Runtime: settings, diagnosis, metrics, resource preview/configuration and start. Colima reconfiguration restarts the entire VM, affecting other projects too; describe the preview's affected containers first. Native Docker has no Colima VM allocation.
- Proxy: inspect, preview, start/stop and check configured local URLs. Stopping the shared proxy affects all its applications.
- Infrastructure: instances, SQL databases/users, dedicated Redis credentials, connection details, service bindings, checks, backups/restores, archive/restore and permanent SQL purge. Shared-resource ownership and consumers matter. Binding saves metadata; review/approve/up the application afterward to apply it. It does not migrate data or modify .env. Credential reveal must be deliberate and results kept private.
- Maintenance: inventory, versions, preview and installation/upgrade/repair of supported tools, startup status, update check and private configuration backup. These can affect host tools; use existing authorization and platform capabilities. Configuration backups include credentials and exclude physical SQL data.
- Logs: nearprod_application_logs and nearprod_infrastructure_logs return bounded, non-following snapshots. Output is capped in lines/bytes/time and indicates truncation. Do not print secrets from application-generated logs.
- The web's SSE /events is replaced by polling status/operations; /logs and /infra/logs need the dedicated log tools, not the JSON bridge. Browser login/logout/pairing and accordion/layout state are not application operations. Agent shutdown/startup setup/catalog migration execution remain CLI administrative workflows. The advanced bridge still reaches local JSON administrative endpoints, but is not needed for normal agent workflows.

## Deletion and confirmation

Archive preserves resources and data. Legacy remove is metadata-only and also preserves resources/data; do not describe it as complete cleanup. For complete removal use application_lifecycle action=purge without confirm to inspect the preview. Explain resources/data to delete, preserved source/shared resources, blockers and effects. To apply, pass confirm=true, the exact reviewed fingerprint, typedId equal to target, and acknowledgeDataLoss=true under explicit deletion authorization. Poll the returned operation and verify its result; partial failure leaves a pending purge to inspect/retry. Never replace blocked cleanup with broad Docker prune or manual deletion of unidentified resources.

For group deletion and application archive/restore, confirm=true also requires the caller's reviewed fingerprint. For infrastructure, use the corresponding preview then apply with its exact fields and fingerprint; database creation, backups, restore and selected stop operations have their own documented confirmations. A preview does not authorize mutation.

## Resource diagnosis and corrective guidance

For OOM, inspect actual host memory, Colima allocation/guest availability, container limits, oom/exit status and build peaks. Image disk size is not equivalent to runtime RAM. Distinguish a container limit from exhausted VM memory and host pressure. Propose workload/build concurrency or configuration changes based on evidence; do not remove all limits or restart Colima automatically. Diagnose unavailable prerequisites through doctor and maintenance inventory.

Common corrections: MODE_REQUIRED/FILES_INVALID -> complete the dev/files definition; ROOT/PATH errors -> register/select an existing permitted path; COMPOSE_TRANSITIVE -> explicit compatible files; COMMAND_FAILED -> inspect redacted Compose diagnostic and correct the referenced file/variable; VERIFY_BLOCKED -> repair verify configuration; RISK_APPROVAL_REQUIRED -> explain the reported risks; PREVIEW_CHANGED -> review changed state; OWNERSHIP/BINDING conflicts -> inspect identity/adoption/consumers; missing healthchecks -> report unchecked and propose an authorized readiness check. Never claim a diagnostic guarantees a successful deployment.
