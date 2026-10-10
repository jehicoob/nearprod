# MCP guidance and web parity implementation plan

## Outcome
An agent can discover NearProd requirements, configure and operate applications through the same domain services as the web UI, and explain and resolve configuration gaps without guessing request fields or reading NearProd source.

## Implementation
1. Publish an embedded agent guide as an MCP resource and a read-only tool. Cover actual Compose restrictions, development/image verification, registration, authorized file edits, review/trust, asynchronous operation IDs, health, resource diagnosis, and deletion semantics.
2. Define explicit application schemas and an operation catalog with request schemas, descriptions, result expectations and confirmation requirements. Publish the catalog through MCP. Preserve existing tool names; confirmed lifecycle operations must use the fingerprint the caller reviewed.
3. Expose missing web operations through domain tools: roots, application resources and batch registration, full purge, runtime, proxy, infrastructure and host-tool maintenance. Add bounded infrastructure logs; explain polling equivalents for web events and exclude browser authentication/presentation from operational parity.
4. Preserve structured domain errors and reject streaming routes through the JSON bridge with actionable guidance. Keep all mutations inside the existing loopback agent and its domain validations.
5. Test published schemas and guidance, representative configuration/action/error flows, stale confirmations, logs, and route/action coverage against the HTTP implementation. Run the repository checks and a read-only installed stdio smoke test.
6. Build and update the existing local installation with a recoverable binary backup; preserve applications, containers, data and catalog. Report verified behavior and remaining limits.

## Constraints
- Do not create or change user application Compose files during this implementation.
- Agent instructions require explaining necessary changes and using existing authorization; additional approval is needed only for new scope or destructive consequences.
- MCP does not grant filesystem tools to clients. Clients without local file access must report that prerequisite.
- No production resource mutations, container recreation, data deletion, package installation or publication. Local commits were subsequently authorized by the user.
- Documentation follows the established language of each document; this plan and embedded agent contract are English.

## Acceptance evidence
- Resources and tools expose actionable setup guidance and field-level schemas.
- Every HTTP JSON route and dispatched domain action has a documented MCP mapping; intentional browser-only/streaming differences are explicit and tested.
- Missing fields fail before dispatch, exact reviewed fingerprints reach domain services, and domain error details survive MCP transport.
- Tests use temporary fixtures/fake engines; live checks only inspect metadata/guidance/schemas and health.

## Status
Completed and installed locally as `0.9.2-dev.5` on 2026-10-10.

- Published 22 tools and 2 resources; the operation catalog documents 83 operations across 62 distinct JSON method/route pairs. SSE logs have bounded adapters, web events use polling, and browser/CLI exceptions are explicit.
- `make check` passed: TypeScript checking/build, 8 UI state tests, Go vet, the full Go suite with and without race detection, and 10 release tests.
- All 13 focused MCP tests passed, including the final parity checks for application, infrastructure, runtime, proxy, maintenance and Watch actions. Fixtures use fake Engines and temporary homes; they do not prove real workload behavior.
- Native binary smoke and package installation tests passed in isolated homes. The package test now exercises stdio initialization, tool/resource discovery, embedded guide reading and the purge schema. Evidence: `evidence/native-package.json`.
- The installed binary passed a real read-only stdio check for instructions, tools, resources, full operation schemas and local health. The catalog SHA-256 was unchanged; all 15 existing containers retained their IDs and start times. No project Compose files or application resources were modified.
- Installation evidence: `/private/tmp/nearprod-mcp-install-result.json`. The native installer retained the previous executable for recovery. Reconnect existing MCP clients to load the new server/tool definitions.
- Implementation was prepared in `nearprod-complete-delete` on `feat/complete-app-deletion`; pre-existing work in both worktrees was preserved. The temporary dependency symlink was removed.

## Operational limits

Filesystem editing requires tools and permissions supplied by the agent client. Published guidance cannot force an arbitrary client/model to follow the workflow. Logs are bounded snapshots and web events are polled. Destructive and host-maintenance operations were exercised only with fixtures in this task; no live application data was deleted or host toolchain modified apart from the NearProd executable update.
