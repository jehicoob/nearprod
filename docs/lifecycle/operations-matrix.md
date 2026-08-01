# Matriz normativa de operaciones de ciclo de vida

- **Jira:** NPROD-87 / LC-01
- **ADR:** [`../decisions/0003-product-lifecycle-data-ownership.md`](../decisions/0003-product-lifecycle-data-ownership.md)
- **Estado:** Propuesta — pendiente de aprobación técnica y del Product Owner
- **Regla global:** código fuente, repositorios, bind mounts y volúmenes del usuario nunca se eliminan por defecto

## Contrato transversal

Todas las mutaciones:

1. autentican actor y aplican Host/Origin/CSRF o scope CLI;
2. cargan estado autoritativo y validan `expectedRevision`;
3. rechazan jobs incompatibles y estado desconocido;
4. generan un plan con recursos afectados y explícitamente no afectados;
5. aplican el nivel de confirmación del ADR;
6. crean snapshot/backup requerido antes del primer reemplazo;
7. ejecutan una sola transición atómica;
8. escriben auditoría redactada y hacen read-after-write;
9. devuelven código estable, estado final y rollback/undo disponible.

`plan`/dry-run nunca muta, reserva IDs indefinidamente ni concede autorización futura. Planes L2/L3 son single-use, cortos y fijados por principal, sesión/factor, operación, target, revisions y digests. `execute` exige el mismo principal; L3 exige además la misma sesión/factor de reautenticación reciente. Cambio de principal devuelve `FORBIDDEN`; cambio, revocación o expiración de sesión/factor devuelve `PLAN_EXPIRED`; ambos fuerzan un plan nuevo.

## Códigos de error mínimos

| Código | Significado | Comportamiento |
| --- | --- | --- |
| `AUTH_REQUIRED` | Identidad ausente/inválida | 401; cero side effects |
| `FORBIDDEN` | Scope/CSRF/Origin/trust insuficiente | 403; cero side effects |
| `INVALID_INPUT` | Schema/ID/path/bundle inválido | 400; lista segura de campos |
| `NOT_FOUND` | Target no existe para actor autorizado | 404 |
| `REVISION_CONFLICT` | Estado cambió desde el plan | 409; recalcular plan |
| `ALREADY_IN_TARGET` | La meta ya estaba alcanzada | 200; cero cambios, revision/resultado actual y sin reejecutar side effects |
| `INVALID_TRANSITION` | Estado no permite operación | 409; indicar estados permitidos |
| `RESOURCE_BUSY` | Job/lock/transición incompatible | 409/423; retry seguro |
| `DEPENDENCY_BLOCKED` | Runtime/path/dependencia no verificable | 409/503; fail closed |
| `PAYLOAD_TOO_LARGE` | Bundle/payload excede límite | 413 antes de procesar completamente |
| `PLAN_EXPIRED` | Plan/token usado, vencido o no coincide | 409; nuevo dry-run |
| `SNAPSHOT_FAILED` | No se pudo crear/verificar snapshot | 500/503; no mutar |
| `WRITE_FAILED` | Commit atómico/read-after-write falló | 500; reconciliar last-known-good |
| `AUDIT_FAILED` | No se pudo registrar evento requerido | 503; L2/L3 no mutan |
| `UNSUPPORTED_SCHEMA` | Versión no migrable | 422; conservar entrada intacta |

## Matriz de estado y operación

Leyenda: ✅ permitida; ⚠️ permitida con gate especial; ⏺ meta ya alcanzada (`ALREADY_IN_TARGET`, 200 sin cambios); ❌ denegada (`INVALID_TRANSITION`, 409 salvo código más específico).

| Operación | `active` | `archived` | `deleted` | Nivel |
| --- | --- | --- | --- | --- |
| Read/list | ✅ | ✅ vista Archivo | ✅ tombstone/auditoría | L0 |
| Plan/dry-run | ✅ | ✅ | ✅ para undo/purge | L0 |
| Edit metadata | ✅ | ❌ | ❌ | L1 |
| Export | ✅ | ✅ snapshot/metadata | ❌ export completo; tombstone mínimo según auditoría | L0/L1 al escribir destino |
| Archive | ✅ | ⏺ | ❌ | L1 |
| Restore archived | ❌ | ✅ → active | ❌ | L1 |
| Delete lógico | ✅ → deleted | ✅ → deleted | ⏺ | L2 |
| Undo delete | ❌ | ❌ | ⚠️ → estado anterior | L2 |
| Purge metadata | ❌ | ❌ | ⚠️ tras retención/gate | L3 |
| Import nuevo | ✅ global si ID libre | ✅ global si ID libre | ❌ si colisiona tombstone | L1 |
| Import reemplazo | ⚠️ L2 | ⚠️ L2 | ❌ hasta undo/purge/remap | L2 |
| Restore backup | ⚠️ reemplaza metadata | ⚠️ reemplaza metadata | ❌ salvo recuperación administrada específica | L2 |
| Start/Stop/Restart/Down | Según trust/preflight | ❌ | ❌ | Fuera de lifecycle |
| Logs/jobs runtime | Según ownership/trust | ❌ | ❌ | Lectura privilegiada |

## Matriz detallada

| Operación | Precondiciones | Resultado observable | Rollback/undo | Error/estado seguro |
| --- | --- | --- | --- | --- |
| **Create** | ID libre sin active/archive/tombstone; schema válido; paths declarados seguros; actor autorizado | Producto `active`, componentes no confiables, revision inicial, auditoría | Delete lógico posterior; si commit falla no debe aparecer parcialmente | Colisión/invalid input → cero registro; write/audit failure → last-known-good |
| **Edit** | `active`; expectedRevision; sin transición concurrente; cambios válidos | Nueva revision y diff; trust se invalida si cambia identidad ejecutable | Snapshot previo permite restore mediante operación L2 | Conflict/invalid → estado previo intacto |
| **Archive** | `active`; sin jobs; runtime detenido/no creado o plan bloquea; snapshot verificable | Snapshot archived, registro fuera de dashboard operativo; recursos del usuario intactos; si ya está archived, `ALREADY_IN_TARGET` sin side effects | Restore archived L1 | Contenedores/jobs/unknown → bloquear; snapshot failure → active intacto; cualquier otro estado → `INVALID_TRANSITION` |
| **Restore archived** | `archived`; ID libre; snapshot/schema/digest válidos; paths/dependencias revisados | `active` con nueva revision; trust/jobs/sesiones no reviven | Archive nuevamente; snapshot previo permanece | Path/ID conflict → no sobrescribir, exigir remap/decisión |
| **Delete lógico** | `active` o `archived`; sin jobs; plan L2; snapshot + audit disponibles | Tombstone `deleted`, `previousState`, `deletedAt`, `eligiblePurgeAt`; no toca runtime/código/volúmenes; si ya está deleted, `ALREADY_IN_TARGET` sin side effects | Undo L2 durante retención | Runtime activo → bloquear y ofrecer operación separada; fallo → estado original; estado no equivalente → `INVALID_TRANSITION` |
| **Undo delete** | `deleted`; retención vigente; snapshot/digest válidos; IDs/paths sin conflicto; plan L2 | Restaura `previousState` o destino aprobado; nueva revision; trust inválido | Delete lógico nuevo | Conflicto/obsolescencia → mantener tombstone y ofrecer remap |
| **Purge metadata** | `deleted`; eligible o excepción aprobada; reauth; plan L3 exacto; auditoría durable | Snapshot/tombstone NearProd eliminados según retención; evento mínimo preservado | Ninguno; irreversible | Plan/revision/audit mismatch → no purgar |
| **Export preview** | `active`/`archived`; actor autorizado | Manifest de contenido/inclusiones/exclusiones y digest esperado | No aplica; no muta | Secreto/dato no clasificable → bloquear export |
| **Export write** | Preview vigente; destino permitido; espacio/permisos; confirmación de overwrite si aplica | Bundle metadata-only canónico con manifest/digest; archivo final ownership usuario | Eliminar solo temporal propio ante fallo; no borrar destino previo sin estrategia atómica | Write/rename failure → destino previo intacto y temporal limpiable |
| **Import preview** | Bundle limitado; parser seguro; manifest/digest/schema válidos | Plan de objetos, ID mapping, conflictos, paths declarados y trust descartado | No aplica; no muta | Traversal/symlink/secret/schema → rechazar bundle completo |
| **Import nuevo** | Preview vigente; IDs libres; plan L1; storage/audit disponibles | Objetos `active`, nueva revision, trust false, rutas revalidadas | Delete lógico; rollback transaccional si lote falla | Cualquier objeto inválido → lote completo sin aplicar |
| **Import reemplazo** | Target active/archived; plan L2; snapshot actual; estrategia explícita replace/remap | Metadata reemplazada/remapeada; trust/jobs invalidados; audit/diff | Restore snapshot anterior L2 | Nunca merge silencioso; conflicto → estado previo |
| **Restore backup** | Backup identificado como propio por manifest; digest/schema válidos; target permitido; plan L2 | Metadata reemplazada atómicamente; backup del estado anterior; trust/jobs invalidados | Restaurar snapshot previo L2 | Backup corrupto/incompatible → preservar y no mutar |
| **Rotate backups** | Manifest ownership; política/edad/cantidad; ningún backup pinned por operación/retención | Solo backups NearProd elegibles eliminados; auditoría/resumen | No garantizado; conservar mínimo last-known-good | Ownership/uso desconocido → no eliminar |
| **Runtime Down** | `active`; trust/preflight; plan de runtime; confirmación propia | Contenedores/redes privadas verificadas retiradas; volúmenes preservados por defecto | Start puede recrear runtime, no datos borrados | No forma parte de archive/delete; fallo parcial se reconcilia |

## Recursos afectados por operación

| Operación | Metadata | Backups/snapshots | Contenedores | Redes privadas | `dev_proxy` | Volúmenes | Código/checkout | Remoto GitHub |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Archive | mover/snapshot | crear | no | no | no | no | no | no |
| Delete lógico | tombstone | crear/retener | no | no | no | no | no | no |
| Undo | restaurar | retener | no | no | no | no | no | no |
| Purge | borrar elegible | borrar elegible | no | no | no | no | no | no |
| Export | leer | opcional metadata aprobada | no | no | no | no | no, solo referencias | no |
| Import | crear/reemplazar | snapshot previo | no | no | no | no | no | no |
| Restore backup | reemplazar | crear previo | no | no | no | no | no | no |
| Runtime Down | no | no | retirar verificados | retirar solo privadas verificadas | no | **no por defecto** | no | no |

## Recuperación durable ante crash

Toda mutación multi-artefacto usa el journal normativo `PREPARED -> SNAPSHOT_WRITTEN -> METADATA_COMMITTED -> AUDIT_WRITTEN -> DONE`, fijado al principal, target, revision, plan y digests. Antes de aceptar otra mutación del target, el arranque/reconciliador aplica exactamente una regla:

| Última fase durable | Acción de recuperación |
| --- | --- |
| `PREPARED` | Abortar; eliminar solo temporales propios verificados; estado autoritativo previo intacto. |
| `SNAPSHOT_WRITTEN` | Abortar; conservar o rotar snapshot según manifest; metadata previa intacta. |
| `METADATA_COMMITTED` | Completar auditoría desde journal/read-back y avanzar; no revertir a ciegas metadata visible. |
| `AUDIT_WRITTEN` | Verificar metadata + evento, persistir respuesta terminal y marcar `DONE`. |
| Fase/digest/identidad ambiguos | `DEPENDENCY_BLOCKED`; intervención segura, cero mutaciones adicionales. |

Retries con la misma idempotency key reciben la respuesta terminal preservada. Una key nueva se evalúa contra el estado y retorna `ALREADY_IN_TARGET` o `INVALID_TRANSITION` según corresponda.

## Defaults operativos propuestos (LC-H5)

Hasta aprobación técnica y de seguridad, estos valores son hipótesis de diseño versionadas, no capacidades soportadas: bundle/payload recibido ≤ 32 MiB; contenido expandido acumulado ≤ 128 MiB; ≤ 1.000 entradas; profundidad ≤ 32; ≤ 100 objetos NearProd por import; `planTTL` 5 minutos; reautenticación L3 5 minutos; máximo 3 retries con backoff 1/2/4 segundos únicamente para fallos transitorios antes de conocer un resultado terminal. Los límites se aplican durante streaming/parsing, antes de reservar memoria o escribir destinos; tamaño recibido y expandido se controlan independientemente.

## Mensajes mínimos para la persona

Cada plan/resultado muestra:

- operación y estado actual → objetivo;
- producto/componentes por ID y label;
- revisión/digest fijados y expiración del plan;
- recursos que cambiarán;
- bloque explícito **“No se eliminará”** con código, repos, bind mounts, volúmenes, imágenes, `dev_proxy` y remotos;
- ventana y fecha exacta de undo/purge en UTC;
- snapshot/artefacto creado y cómo verificarlo;
- código de error, causa probable y siguiente acción segura;
- resultado final leído de la fuente autoritativa, no inferido de que el request terminó.

Ejemplo de delete lógico:

```text
Eliminar del registro: Example App (example_app)
Estado: active -> deleted
Retención/undo: hasta 2030-01-31T18:00:00Z
Cambiará: metadata NearProd; se crearán snapshot y tombstone.
No se eliminará: código, checkout Git, .env, contenedores, redes, volúmenes, imágenes o repositorio remoto.
Para confirmar, escribe: example_app
```

## Casos de prueba obligatorios

1. Archive con 0/1/múltiples componentes y runtime stopped/running/unknown.
2. Delete desde active y archived; jobs running; snapshot/audit failure; revision conflict.
3. Undo antes/después de retención; ID/path libre, ocupado o cambiado; schema migrable/no migrable.
4. Purge exacto, plan vencido/usado, actor distinto, retención no cumplida y fallo de auditoría.
5. Export con secretos simulados, paths, valores justo debajo/en/sobre cada límite LC-H5, destino existente y rename fallido.
6. Import vacío/1/múltiples, IDs duplicados, traversal, symlink, expansión comprimida, valores justo debajo/en/sobre límites LC-H5, schema futuro y lote parcialmente inválido.
7. Restore backup válido, corrupto, ajeno sin manifest, antiguo, concurrente y read-after-write fallido.
8. Demostrar en archive/delete/purge que código, `.git`, bind mounts, volúmenes, imágenes y `dev_proxy` permanecen intactos.
9. Retry de cada operación después de timeout/respuesta perdida en cada fase durable: misma key devuelve resultado terminal; key nueva produce `ALREADY_IN_TARGET` o `INVALID_TRANSITION` sin duplicar side effects.
10. Auditoría sin secretos en success, denied, conflict, failure y recovery.
11. Ejecutar planes L2/L3 con mismo/distinto principal, misma/distinta sesión/factor, revocación y expiración; verificar `FORBIDDEN`/`PLAN_EXPIRED` y regeneración completa.

## Gates pendientes

Esta matriz se vuelve normativa cuando se aprueba ADR-0003. Permanecen decisiones explícitas LC-H1–LC-H4. Ninguna implementación puede reducir confirmación, retención o recursos protegidos sin nueva decisión humana y actualización coordinada del ADR, matriz y pruebas.
