# Decision record 0003: ciclo de vida y propiedad de datos

- **Jira:** NPROD-87 / LC-01
- **Estado:** Propuesta — pendiente de aprobación técnica y del Product Owner
- **Fecha:** 2026-08-01
- **Responsables de aprobación:** responsable técnico de NearProd y Jehicoob López como Product Owner
- **Matriz normativa:** [`../lifecycle/operations-matrix.md`](../lifecycle/operations-matrix.md)
- **Decisiones relacionadas:** [frontera de seguridad de la API](0002-privileged-api-security-boundary.md)

## Contexto

El baseline permite crear productos/componentes en `projects.yml`, generar copias `.bak-*` y ejecutar `start`, `stop`, `restart` y `down`. No existe un contrato implementado para editar, archivar, eliminar, importar, exportar, restaurar, deshacer o purgar. Tampoco existe un campo de ciclo de vida en el schema actual.

Este ADR define el **modelo objetivo**; no afirma que las operaciones estén implementadas. NPROD-88, NPROD-91, NPROD-92 y NPROD-93 no pueden tomar decisiones divergentes sobre estados, ownership o destrucción. El estado actual create-only permanece operativo como baseline, pero no autoriza inferir borrado de código, volúmenes o repositorios.

## Objetivos

1. Separar la visibilidad del registro, el estado de ejecución y la propiedad de datos.
2. Hacer reversibles por defecto las acciones de ciclo de vida.
3. Impedir que “eliminar producto” signifique eliminar código fuente o datos Docker.
4. Definir una fuente autoritativa, transiciones válidas y comportamiento ante incertidumbre/concurrencia.
5. Proporcionar export/import/restore verificables, idempotentes y auditables.
6. Aplicar la frontera de seguridad NPROD-15 a toda mutación.

## No objetivos

- Implementar endpoints, UI, migraciones o comandos.
- Definir una papelera del sistema operativo para repositorios del usuario.
- Gestionar backups de bases de datos de aplicaciones.
- Borrar volúmenes, bind mounts, repositorios o imágenes como efecto implícito.
- Resolver ownership de un runtime Docker compartido fuera de los labels/identidades verificables de NearProd.

## Terminología e identidad

- **Producto:** grupo lógico con un ID estable e inmutable y una revisión (`revision`).
- **Componente:** recurso registrado con ID estable, identidad de path/runtime y referencia al producto.
- **Registro activo:** objeto autoritativo visible y operable según sus demás gates (auth, trust, preflight).
- **Archivo:** snapshot inmutable y verificable de metadata/configuración para restauración; no es un directorio de código.
- **Tombstone:** registro mínimo de una eliminación lógica, suficiente para undo, colisiones, auditoría y purga.
- **Purga:** eliminación irreversible de metadata NearProd retenida; nunca incluye datos del usuario por defecto.
- **Recurso administrado:** recurso creado/gestionado por NearProd con identidad verificable, no simplemente un recurso que comparte nombre.

IDs no se reutilizan mientras exista un activo, archivo o tombstone retenido. Labels visibles, paths y URLs pueden cambiar; no sustituyen la identidad estable.

## Decisión 1: estados y transiciones

NearProd adopta exactamente estos estados de ciclo de vida para producto y, cuando corresponda, componente:

| Estado | Significado | Visible por defecto | Acciones permitidas | Acciones denegadas |
| --- | --- | --- | --- | --- |
| `active` | Registro vigente. Puede operar solo si auth, trust y preflight pasan | Sí | read, export, edit; runtime actions condicionadas; archive; delete mediante plan | restore; purge; import sobre el mismo ID sin política de conflicto |
| `archived` | Snapshot conservado y registro excluido de operación normal | No; visible en vista Archivo | read metadata, export, restore, delete lógico | edit normal; start/stop/restart/down; trust; logs/runtime; purge directo |
| `deleted` | Tombstone retenido después de eliminación lógica | No; visible en Papelera/auditoría | read tombstone, undo durante retención, purge después del gate | edit, runtime actions, archive, export completo, import sobre el mismo ID |

Transiciones válidas:

```text
create/import -> active
active -> archived
archived -> active       (restore)
active -> deleted        (delete lógico; archiva snapshot primero)
archived -> deleted      (delete lógico)
deleted -> active|archived (undo al estado anterior, si sigue siendo válido)
deleted -> purged        (fin de identidad retenida; no es otro estado observable)
```

No existen transiciones implícitas basadas en Docker. Un contenedor detenido no archiva el producto; `down` no elimina el registro; borrar manualmente `projects.yml` no constituye una transición válida.

### Invariantes de transición

1. Cada mutación exige `expectedRevision`; conflicto o estado desconocido produce 409 y cero side effects.
2. El snapshot/plan durable se crea y valida antes de reemplazar el registro autoritativo.
3. Una transición afecta producto y componentes como una unidad lógica; no deja referencias huérfanas.
4. Un producto debe estar sin jobs activos para archive/delete/restore/purge.
5. Estado `archived` o `deleted` se hace cumplir server-side; ocultar botones no es autorización.
6. Undo nunca sobreescribe un ID/path/configuración que apareció después; se resuelve el conflicto explícitamente.
7. Fallo de auditoría, snapshot, fsync/replace, identidad o autorización detiene la operación.

### Idempotencia y estado ya alcanzado

Repetir una solicitud con la misma idempotency key y el mismo plan confirmado devuelve el resultado terminal registrado sin reejecutar side effects. Una solicitud nueva cuya meta ya está alcanzada devuelve `ALREADY_IN_TARGET` (200, cero cambios y revision actual); no se trata como autorización para omitir precondiciones. Cualquier otra transición no permitida devuelve `INVALID_TRANSITION` (409). Por ejemplo, `archive(archived)` y `delete(deleted)` son “ya alcanzado”; `restore(active)` y `purge(active)` son transiciones inválidas.

## Decisión 2: propiedad de datos

La propiedad determina qué puede borrar NearProd automáticamente. “NearProd puede operar” no significa “NearProd es propietario”.

| Recurso | Propietario por defecto | NearProd puede crear/modificar | NearProd puede eliminar por defecto | Regla |
| --- | --- | --- | --- | --- |
| Registro de producto/componente y metadatos de lifecycle | NearProd | Sí | Sí, mediante retención/purga | Fuente autoritativa versionada; sin secretos. |
| Snapshots, tombstones y auditoría de lifecycle | NearProd | Sí | Solo por política de retención/purga | Retención independiente de archivos de usuario. |
| Backups `.bak-*` generados por NearProd | NearProd | Sí | Sí, por rotación NPROD-49 | Validar antes de restaurar; nunca asumir que todo `.bak-*` es propio sin manifest. |
| Jobs/logs internos de NearProd | NearProd | Sí | Sí, por TTL/retención | Redactados, con ownership y límites. |
| Código fuente/checkout local | Usuario | Solo con operación explícita futura y acotada | **No** | Path registrado o clone creado por NearProd sigue siendo código del usuario. |
| `.git`, ramas, cambios no committeados | Usuario | No por lifecycle | **No** | Estado sucio bloquea cualquier operación futura que pretenda tocar el repo. |
| `.env`, credenciales y secretos de aplicación | Usuario | No | **No** | No exportar ni auditar contenido. |
| Bind mounts y archivos bajo paths del proyecto | Usuario | No por lifecycle | **No** | Ni archive, delete, purge o uninstall los eliminan. |
| Volúmenes Docker nombrados/anónimos | Usuario/aplicación | Puede adjuntar por Compose | **No** | Requieren operación separada, inventario, backup/política y confirmación destructiva futura; fuera de delete por defecto. |
| Contenedores Compose con identidad verificada | Runtime administrado; datos efímeros | Sí | Solo por `down` explícito, no por delete lifecycle | No usar coincidencia de nombre; verificar labels/daemon/context. |
| Redes Compose privadas con identidad verificada | Runtime administrado | Sí | Solo por `down` explícito si no son compartidas | `dev_proxy` es compartida y nunca se elimina por producto. |
| Red compartida `dev_proxy` | Instalación/usuario compartido | NearProd puede crear si falta | **No** por lifecycle de producto | Solo uninstall de plataforma con inventario independiente puede considerarla. |
| Imágenes y build cache Docker | Runtime/usuario compartido | Puede crear/pull/build | **No** | Purga es operación de mantenimiento separada y explícita. |
| Repositorio remoto GitHub | Usuario/proveedor remoto | No | **No** | NearProd no borra branches, releases ni repos remotos. |
| Export bundle | Usuario al crearlo | NearProd genera contenido acotado | Solo destino temporal propio | El usuario controla el archivo final. |

### Regla de origen

NearProd solo trata un artefacto como propio cuando existe un manifest durable con ID, tipo, versión, creador, timestamp, digest y relación con el objeto. Path, prefijo de filename, label visible o nombre Compose por sí solos no demuestran ownership.

## Decisión 3: confirmación proporcional

| Nivel | Operaciones | Confirmación |
| --- | --- | --- |
| L0 lectura | list/read/plan/dry-run/export preview | Sin confirmación destructiva; sí auth/autorización. |
| L1 reversible no destructiva | edit validado, archive, restore desde archived | Resumen de cambios y una confirmación explícita; revision pin. |
| L2 destructiva reversible | delete lógico de active/archived, restore de backup que reemplaza metadata | Dry-run obligatorio, lista de recursos afectados/no afectados, nombre/ID a confirmar, snapshot verificado y ventana de undo. |
| L3 irreversible | purge de tombstone/archivo/backup NearProd | Dry-run separado, reautenticación reciente, escribir ID exacto y frase `PURGE`, segundo paso con token de plan corto y revision/digest fijados. |
| Fuera de lifecycle | borrar volúmenes, código, repos, bind mounts, imágenes o red compartida | **No permitido por estas operaciones**; requeriría feature/issue, contrato y aprobación humana independientes. |

Confirmaciones no se agrupan entre productos. Un token de plan contiene principal estable, sesión o factor de autenticación, operación, IDs, revisiones, digests, recursos, expiración y nonce; es single-use. La ejecución L2/L3 exige el mismo principal autenticado que creó el plan y, para L3, la misma sesión/factor de reautenticación reciente. Cambio de principal devuelve `FORBIDDEN`; sesión/factor cambiado, vencido o revocado devuelve `PLAN_EXPIRED`. En ambos casos se descarta el token y se genera un plan completo nuevo. Si cualquier otro dato cambia, el plan expira y se recalcula.

## Decisión 4: retención, undo y purga

- Delete lógico conserva snapshot + tombstone durante **30 días** por defecto.
- Archivos no se purgan automáticamente por antigüedad en 1.0.0; el Product Owner debe decidir una política distinta mediante configuración explícita.
- Undo está disponible durante la retención si identidad, schema, paths y dependencias siguen válidos.
- La ventana se calcula con reloj UTC server-side; cambios de reloj detectados no acortan silenciosamente la retención.
- Purge antes de 30 días requiere L3 y registra el motivo; nunca purga auditoría mínima necesaria para demostrar la operación según la política aprobada.
- Al vencer la retención, el tombstone queda **eligible**, no automáticamente purgado, hasta que exista un job de retención probado y observable.
- Un backup inválido/corrupto no se usa; se preserva para diagnóstico hasta su política de retención.

**Hipótesis LC-H1:** 30 días equilibran recuperación y acumulación local. **Propietario:** Product Owner. **Siguiente acción:** aprobar o fijar otro valor antes de NPROD-90/92.

## Decisión 5: archive, delete y runtime

Archive/delete operan sobre el **registro**, no sobre Docker ni filesystem del proyecto:

1. Se exige que no existan jobs activos.
2. Si hay contenedores en ejecución, el plan bloquea y ofrece ejecutar Stop/Down como operación separada; no lo hace implícitamente.
3. `archive` crea snapshot y retira el registro de operación normal.
4. `delete` crea snapshot y tombstone, y retira el registro activo/archivado.
5. Ninguna de estas operaciones ejecuta `docker compose down`, `--volumes`, `rm`, `git`, unlink/rmdir sobre paths del proyecto ni eliminación remota.

Esto evita que un fallo parcial combine metadata eliminada con runtime/data todavía mutados. El usuario puede decidir runtime primero, verificarlo y luego cambiar lifecycle.

## Decisión 6: export, import y restore

### Export

- Exporta schema/version, IDs, lifecycle permitido, metadata/configuración no secreta, quick links, referencias de path declaradas (no contenido), digests y manifest.
- Excluye `.env`, credenciales, cookies/tokens, logs crudos, código, `.git`, datos/volúmenes y secretos detectados.
- Es determinista/canónico para poder comparar digest; escribir fuera de un destino temporal propio requiere path/confirmación explícitos.

### Import

- Siempre ejecuta parse + schema + límites + plan/dry-run antes de mutar.
- No confía en `trusted`, ownership, actor, timestamps ni estado `active` del bundle; recursos importados inician no confiables y requieren revalidación local.
- Rechaza path traversal, symlinks en bundle, duplicados, IDs reservados, referencias huérfanas y versiones incompatibles.
- Conflictos nunca hacen merge/overwrite automático: opciones explícitas son abortar, remapear IDs o reemplazar metadata mediante L2.

### Restore

- Restaura metadata desde un snapshot NearProd validado por manifest/digest y compatible con schema.
- Crea un snapshot del estado actual antes de reemplazarlo.
- No restaura código, `.env`, volúmenes ni runtime.
- Un restore no revive trust, jobs o sesiones; quedan invalidados y requieren nuevos gates.

**Hipótesis LC-H2:** el bundle 1.0.0 debe contener solo metadata portable y no snapshots de runtime/datos. **Propietario:** responsable técnico + Product Owner. **Siguiente acción:** aprobar antes de NPROD-91/92.

## Decisión 7: auditoría mínima

Cada intento de mutación registra un evento append-only redactado:

| Campo | Contenido |
| --- | --- |
| `eventId` | ID aleatorio y único |
| `timestamp` | UTC server-side |
| `actor` | ID de propietario/sesión o token CLI; nunca credencial |
| `operation` | create/edit/archive/restore/delete/undo/purge/import/export/restore-backup |
| `target` | tipo + ID estable + revision esperada/observada |
| `planDigest` | Digest del plan confirmado, cuando aplica |
| `result` | started/succeeded/failed/denied/conflict |
| `errorCode` | Código estable y seguro; detalle sensible solo en log interno protegido |
| `artifacts` | IDs de snapshot/tombstone/export, no paths secretos ni contenido |

No se registran secretos, bodies completos, cookies/tokens, `.env`, contenido de logs, credenciales Git, query strings sensibles ni stack traces al cliente. Un evento `started` sin terminal se reconcilia al reiniciar antes de aceptar otra mutación sobre el mismo objetivo.

## Decisión 8: almacenamiento y atomicidad

El parser/serializer manual y `writeFile` directo del baseline no satisfacen este contrato. La implementación debe:

1. usar schema versionado y parser seguro;
2. mantener active/archive/tombstone/audit en storage con ownership claro;
3. aplicar lock o compare-and-swap por revision;
4. escribir a archivo temporal seguro en el mismo filesystem, fsync según soporte y rename atómico;
5. preservar last-known-good y verificar el read-after-write;
6. serializar operaciones por producto y recuperar intentos incompletos;
7. no seguir si atomicidad, permisos o identidad del storage son desconocidos.

### Protocolo durable de transición

Cada mutación multi-artefacto conserva un journal durable, fijado al target/revision/plan, con fases monotónicas:

```text
PREPARED -> SNAPSHOT_WRITTEN -> METADATA_COMMITTED -> AUDIT_WRITTEN -> DONE
```

- `PREPARED`: autorización, plan, lock/CAS y precondiciones revalidados; aún no hay cambio autoritativo.
- `SNAPSHOT_WRITTEN`: snapshot/manifest escrito, fsync/read-back y digest verificados.
- `METADATA_COMMITTED`: cambio autoritativo reemplazado atómicamente y leído de vuelta.
- `AUDIT_WRITTEN`: evento terminal append-only durable enlazado al plan y revisión resultante.
- `DONE`: resultado terminal publicado; el journal ya puede rotarse según retención.

Al arrancar, NearProd bloquea nuevas mutaciones de cualquier target con journal no terminal. `PREPARED` y `SNAPSHOT_WRITTEN` sin commit se revierten limpiando solo temporales propios y conservando el estado previo. `METADATA_COMMITTED` se completa escribiendo/reconciliando la auditoría desde el journal y el read-back; nunca se revierte a ciegas después de hacer visible metadata nueva. `AUDIT_WRITTEN` se marca `DONE` tras verificar metadata y evento. Si identidad, digest o fase no permiten decidir de forma determinista entre rollback y completion, el target queda `DEPENDENCY_BLOCKED` para intervención segura, sin aceptar otra mutación. El journal también conserva la respuesta terminal para retries idempotentes.

NPROD-24 conserva ownership sobre parser YAML/escrituras atómicas. Lifecycle no debe duplicar un segundo mecanismo paralelo.

## Seguridad y abuso

- Todo plan/mutación requiere auth; mutaciones navegador requieren Host/Origin/CSRF según ADR-0002.
- IDs, filenames, labels, paths y bundles son entrada hostil.
- Archive/delete/purge/import/restore usan autorización server-side y no confían en flags del cliente.
- Los endpoints de plan no deben filtrar existencia de recursos a un actor no autorizado.
- Import/export tienen límites de tamaño, recuento, profundidad, tiempo y destino.
- Auditoría falla cerrada para operaciones L2/L3.
- Un symlink/TOCTOU nunca convierte metadata propia en permiso para tocar datos del usuario.

## Alternativas consideradas

| Alternativa | Ventaja | Riesgo | Resultado |
| --- | --- | --- | --- |
| Borrar registro inmediatamente | Simple | Sin undo, colisiones/errores difíciles de recuperar | Rechazada |
| `archived: true`/`deleted: true` independientes | Cambio pequeño | Estados contradictorios y transiciones ambiguas | Rechazada |
| Estado enum + snapshots + tombstones | Invariantes claros, reversible y auditable | Mayor almacenamiento/schema | **Seleccionada** |
| `down` automático al archivar/eliminar | “Limpieza” conveniente | Side effects parciales y confusión metadata/runtime | Rechazada |
| Eliminar clone/volúmenes “creados por NearProd” | Recupera espacio | Código y datos siguen siendo del usuario; ownership difícil | Rechazada por defecto |
| Exportar proyecto completo | Portabilidad aparente | Secretos, tamaño, licencias y datos | Rechazada para 1.0.0 |

## Consecuencias

### Positivas

- Delete es reversible y separado de destrucción de datos.
- Ownership y operaciones pueden probarse con matrices deterministas.
- Import/export no se convierten en canal de secretos o trust.
- La UI/CLI futura puede explicar exactamente qué cambia y qué no.

### Costes/riesgos

- Requiere migrar el schema create-only y diseñar storage versionado.
- Snapshots/tombstones/auditoría necesitan retención, límites y recuperación.
- Paths/configuración pueden quedar obsoletos durante archive/delete; undo puede requerir remapeo en vez de restauración automática.
- La política de 30 días y bundle metadata-only requieren aprobación humana.

## Gates e hipótesis abiertas

- **LC-H1:** retención default 30 días — Product Owner.
- **LC-H2:** export/import metadata-only — responsable técnico + Product Owner.
- **LC-H3:** storage concreto (archivos separados, SQLite u otro) no se decide aquí. **Propietario:** responsable técnico. **Siguiente acción:** spike con atomicidad, concurrencia, backup y migración antes de NPROD-88.
- **LC-H4:** duración de auditoría después de purge. **Propietario:** Product Owner + seguridad. **Siguiente acción:** fijar retención mínima antes de NPROD-90.
- **LC-H5:** límites operativos iniciales propuestos: bundle/payload recibido ≤ 32 MiB, contenido expandido acumulado ≤ 128 MiB, ≤ 1.000 entradas, profundidad estructural ≤ 32, ≤ 100 objetos NearProd por import, `planTTL` de 5 minutos, reautenticación L3 ≤ 5 minutos y máximo 3 retries con backoff exponencial acotado a 1/2/4 segundos solo para fallos transitorios previos a un resultado terminal. **Propietario:** responsable técnico + seguridad. **Siguiente acción:** validar por pruebas de recursos/abuso y aprobar o reemplazar valores antes de NPROD-88/91/92; hasta entonces son defaults de diseño, no soporte operativo.

## Gate de aprobación

La apertura del PR no equivale a aprobación. Responsable técnico y Product Owner deben confirmar:

- [ ] Apruebo los estados `active`, `archived`, `deleted` y sus transiciones.
- [ ] Apruebo que archive/delete/purge solo afectan metadata NearProd por defecto.
- [ ] Confirmo que código, repositorios, bind mounts, volúmenes, imágenes y red compartida nunca se eliminan por defecto.
- [ ] Apruebo confirmaciones L0–L3 y planes single-use fijados por revision/digest.
- [ ] Apruebo LC-H1 (30 días) o registro un valor alternativo.
- [ ] Apruebo LC-H2 (bundle metadata-only) o registro el contenido alternativo.
- [ ] Asigno LC-H3 y LC-H4 con siguiente acción antes de implementar dependencias.
- [ ] Apruebo o reemplazo los límites LC-H5 antes de implementar import/export y ejecución de planes.
- [ ] Apruebo la matriz de operaciones asociada como contrato normativo.

**Condición de parada:** NPROD-87 entrega diseño y preguntas/gates; no implementa UI/endpoints. Hasta resolver el checklist, las tareas dependientes pueden refinar pruebas/prototipos reversibles, pero no fijar una política distinta ni ejecutar migraciones/destrucción real.
