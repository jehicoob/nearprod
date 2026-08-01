# Threat model: API privilegiada de NearProd

- **Jira:** NPROD-15
- **Revisión analizada:** `617a43d8676af0032a7ff358dc1789ecd3a78a47`
- **Estado:** Aprobado como modelo objetivo — controles aún no operativos
- **Última actualización:** 2026-08-01
- **Evidencia de aprobación:** autorización explícita para fusionar el PR #2 el 2026-08-01

## Resumen ejecutivo

El proceso `ui/server.mjs` no es un servidor web convencional: puede leer rutas del host, modificar `projects.yml`, consultar logs y controlar Docker mediante un socket montado con escritura. Por tanto, todo `/api/*` es privilegiado aunque la aplicación se anuncie como “local”.

La protección por loopback, `.localhost` o `SameSite` no basta por separado. La frontera debe combinar autenticación, Host/Origin exactos, CSRF, autorización por recurso, validación en el punto de uso y el estado explícito `trusted` antes de ejecutar o inspeccionar un proyecto.

## Estado actual y estado objetivo

Este documento es un **artefacto de diseño**, no una afirmación de que los controles ya estén operativos. En la revisión analizada:

- las siete rutas `/api/*` son accesibles sin autenticación;
- no existe validación de Host/Origin ni protección CSRF;
- `trusted` se acepta y persiste como booleano, pero no autoriza ni protege operaciones;
- los jobs no tienen ownership ni TTL y sus identificadores no son criptográficos;
- las rutas no están confinadas con una identidad estable frente a symlinks/TOCTOU;
- el control panel escucha en `0.0.0.0` dentro de la red Docker y monta el socket Docker con escritura.

Por ello, el estado actual **no satisface esta frontera y no debe considerarse seguro para exposición o uso no supervisado**. NPROD-15 termina con el diseño y su gate humano; NPROD-16/17/18/20/21/23 son responsables de convertirlo en estado operativo. Hasta que esos controles estén implementados y verificados, no se acepta liberar la API como segura ni se concede una excepción temporal implícita.

## Alcance y flujo de datos

```text
[Propietario local]
       |
       | navegador HTTP, cookie de sesión + CSRF
       v
[infra.localhost / Traefik] ---- red dev_proxy ----> [control-panel Node]
                                                        |      |       |
                                                        |      |       +--> memoria de jobs/logs
                                                        |      +----------> projects.yml y backups
                                                        +-----------------> Docker socket
                                                                            |
                                                                            +--> contenedores/redes/volúmenes

[Web maliciosa] ------ solicitudes cross-site / DNS rebinding ------X frontera web
[Proceso local] ------ acceso directo a puerto/socket --------------X autenticación
[Proyecto no confiable] -- path/compose/config hostil --------------X autorización trusted
```

## Actores

| Actor | Confianza inicial | Capacidades legítimas |
| --- | --- | --- |
| Propietario local autenticado | Parcial; debe autenticarse y conservar una sesión válida | Registrar proyectos, aprobar confianza, ejecutar acciones y consultar diagnósticos. |
| Navegador del propietario | Parcial; puede sufrir XSS, extensiones o navegación cross-site | Presentar la UI y enviar solicitudes same-origin protegidas. |
| Cliente CLI local | No confiable hasta presentar un token separado y acotado | Automatización explícita sin reutilizar cookies del navegador. |
| Proyecto registrado | No confiable por defecto | Ser detectado y descrito; no ejecutar Docker ni leer logs hasta aprobar `trusted`. |
| Página web remota/maliciosa | No confiable | Ninguna. Debe fallar aunque pueda alcanzar direcciones loopback desde el navegador. |
| Proceso o contenedor local ajeno | No confiable | Ninguna sobre la API salvo credencial explícita. |
| Traefik y runtime Docker | Infraestructura privilegiada | Encaminamiento local y ejecución de contenedores según la decisión de aislamiento de NPROD-20/NPROD-21. |
| Administrador/root del host | Fuera de la frontera defendible por la aplicación | Puede leer memoria, estado y socket; se considera control total del host. |

## Activos

1. Socket y autoridad del daemon Docker.
2. Archivos del host montados en el contenedor, especialmente `projects.yml`, rutas de proyectos y estado de autenticación.
3. Disponibilidad e integridad de contenedores, redes y volúmenes.
4. Logs y salida de jobs, que pueden contener rutas, datos de aplicación o secretos accidentales.
5. Credenciales de propietario, sesiones y tokens CSRF/CLI.
6. Registro de confianza y la identidad exacta del proyecto aprobado.
7. Integridad de la UI y de respuestas API usadas para tomar decisiones.

## Fronteras de confianza

1. **Navegador → Traefik:** tráfico controlado por el cliente; Host, Origin, cookies y cuerpos son hostiles hasta validarse.
2. **Traefik → Node:** la red Docker no constituye autenticación. No se deben confiar cabeceras reenviadas salvo una lista y un proxy configurados explícitamente.
3. **Node → filesystem:** rutas, symlinks, ownership y cambios entre validación/uso son hostiles; la comprobación se repite o se mantiene un handle seguro en el punto de uso.
4. **Node → Docker:** una operación correcta puede otorgar control del host; la autenticación web no reduce el impacto del socket montado con escritura.
5. **Configuración → ejecución:** `projects.yml` es entrada, no autoridad. Un booleano `trusted` aislado puede quedar obsoleto si cambia path, runner, compose o identidad.
6. **Job → lector:** el identificador de job no es autorización; cada job pertenece a una sesión/propietario y hereda la decisión de confianza que permitió crearlo.

## Inventario y clasificación de endpoints actuales

| Método y ruta | Efecto/activo | Clase | Autenticación | Origin + CSRF | `trusted` | Controles adicionales |
| --- | --- | --- | --- | --- | --- | --- |
| `GET /api/projects` | Lee configuración, estado Docker y rutas | Lectura sensible | Obligatoria | No para GET | No | Minimizar rutas absolutas y datos internos; `Cache-Control: no-store`. |
| `POST /api/detect` | Inspecciona una ruta del host | Lectura privilegiada con entrada | Obligatoria | Obligatorios | No; es pre-registro | Limitar a raíces configuradas, resolver sin escape por symlink y restringir tipo/tamaño de inspección. |
| `POST /api/products` | Escribe configuración y backups | Mutación privilegiada | Obligatoria | Obligatorios | Crea recursos **no confiables** | Validación estricta, escritura atómica y concurrencia controlada. |
| `POST /api/groups/:group/actions/:action` | Ejecuta Docker sobre varios componentes | Ejecución crítica | Obligatoria | Obligatorios | Todos los componentes, sin excepción | Preflight atómico del grupo antes de iniciar el primero; allowlist de acciones. |
| `POST /api/projects/:id/actions/:action` | Ejecuta Docker sobre un proyecto | Ejecución crítica | Obligatoria | Obligatorios | Obligatorio y vigente | Revalidar identidad en el punto de uso; allowlist de acciones; límites de tiempo/recursos. |
| `GET /api/projects/:id/logs` | Lee logs del runtime | Lectura sensible | Obligatoria | No para GET | Obligatorio y vigente | Acotar `tail`, redactar secretos, no aceptar opciones Docker arbitrarias. |
| `GET /api/jobs/:jobId` | Lee comandos, salida y estado | Lectura sensible | Obligatoria | No para GET | Heredado del job | ID no adivinable, vínculo con sesión/propietario, expiración y redacción. |

Ningún endpoint actual es público. Un futuro `GET /api/health` podría ser anónimo solo si devuelve estado mínimo y no expone rutas, versiones sensibles, configuración ni Docker.

## Amenazas y tratamiento

| ID | Amenaza / fallo esperado | Impacto | Tratamiento requerido | Issue dueño |
| --- | --- | --- | --- | --- |
| T1 | CSRF desde una web remota que ordena `down`, `start` o crea productos | Control de Docker/configuración | Sesión autenticada, Origin exacto, token CSRF y `SameSite=Strict`; fallar con 403. | NPROD-16 |
| T2 | DNS rebinding o Host manipulado hacia loopback | Eludir la intención “solo local” | Allowlist exacta de Host/Origin; no reflejar Origin ni habilitar CORS por defecto. | NPROD-16 |
| T3 | API sin autenticación alcanzada por otro proceso/contenedor local | Control total de funciones privilegiadas | Credencial de propietario y sesiones server-side; token CLI separado. | NPROD-16 |
| T4 | `trusted: true` usado como permiso eterno tras cambiar path/runner/compose | Ejecutar un recurso distinto al revisado | Registro de confianza ligado a identidad/fingerprint; invalidación automática y revalidación al usar. | NPROD-17 |
| T5 | Path traversal, symlink escape o check-then-use | Lectura/ejecución fuera de raíces permitidas | Canonicalización segura, raíces allowlist, handles o validación en punto de uso; fail closed. | NPROD-18 |
| T6 | Payload/YAML hostil, corrupción o carreras de escritura | Pérdida o inyección de configuración | Parser seguro, schema estricto, tamaño límite, lock y replace atómico. | NPROD-18/NPROD-24 |
| T7 | Acción Docker o project name manipulado | Ejecución/afectación de recursos ajenos | `spawn` sin shell, argumentos allowlist, identidad confiable y preflight. | NPROD-17/NPROD-21 |
| T8 | Logs/jobs filtran secretos o son enumerables | Exposición de credenciales/datos | IDs criptográficos, ownership, TTL, límites y redacción estructurada. | NPROD-23 |
| T9 | Payload grande, jobs ilimitados o acciones repetidas | DoS local | Límites de cuerpo, concurrencia, rate limit, timeout, cancelación y TTL. | NPROD-23 |
| T10 | XSS roba CSRF o actúa con la sesión | Control con privilegios del propietario | CSP estricta, salida codificada, sin HTML no confiable, cookie HttpOnly; CSRF no sustituye prevenir XSS. | NPROD-16 y backlog UI |
| T11 | Socket Docker con escritura permite escape/host compromise | Compromiso del host | Aislamiento/mediación del daemon, mínimo privilegio y decisión explícita; auth web solo reduce exposición, no impacto. | NPROD-20/NPROD-21 |
| T12 | Cabeceras `X-Forwarded-*` falsificadas | Bypass de esquema/host/origen | Confiar proxy solo por topología/allowlist y derivar origen canónico de configuración, no de cabeceras arbitrarias. | NPROD-16 |

## Invariantes verificables

1. Si identidad, Origin, CSRF, confianza o validación no pueden confirmarse, la operación no se ejecuta.
2. Ninguna ruta de mutación acepta una sesión cookie sin token CSRF válido y Origin exacto.
3. Una credencial CLI no se acepta desde query strings, cookies ni cuerpos genéricos; usa `Authorization` y alcance explícito.
4. `trusted` ausente, inválido, obsoleto o parcialmente comprobado equivale a `false`.
5. Una acción de grupo no produce cambios parciales por una comprobación de confianza tardía: todo el preflight ocurre antes del primer proceso.
6. El proyecto que se valida es el mismo que se usa; no se vuelve a resolver una ruta sin verificar su identidad.
7. Jobs y logs nunca se autorizan solo por conocer un ID.
8. Errores de seguridad son genéricos para el cliente y diagnósticos detallados no contienen secretos.

## Respuestas esperadas

- `401 Unauthorized`: no existe identidad válida; limpiar/rechazar sesión inválida.
- `403 Forbidden`: identidad válida pero Origin, CSRF, scope o `trusted` fallan.
- `400 Bad Request`: Host, path, payload o parámetros son inválidos sin intentar la operación.
- `409 Conflict`: identidad/fingerprint cambió, escritura concurrente o estado incompatible.
- `413 Payload Too Large`: cuerpo excede el límite antes de parsear completamente.
- `429 Too Many Requests`: exceso de login, mutaciones o jobs.
- `503 Service Unavailable`: Docker/configuración no puede verificarse; nunca reportar éxito degradado.

## Riesgo residual y fuera de alcance

- Un administrador/root o atacante con control del mismo usuario del host puede leer estado y controlar Docker; la aplicación no promete aislamiento frente a control total del host.
- HTTP sobre loopback evita exposición de red externa, pero impide exigir cookie `Secure`; TLS local es preferible y debe evaluarse. Mientras siga HTTP, el bind a `127.0.0.1`, Host/Origin exactos y sesiones cortas son obligatorios.
- La frontera web no resuelve el riesgo estructural de montar `/var/run/docker.sock` con escritura. NPROD-20 debe decidir la arquitectura y NPROD-21 implementarla.
- Este documento no cubre supply chain, firma de releases ni vulnerabilidades del daemon/OS.

## Casos mínimos de aceptación futura

- Web cross-site con cookie pero Origin incorrecto → 403 y cero side effects.
- Mutación sin Origin, con `Origin: null` o sin CSRF → 403.
- Host inesperado o cabeceras forward falsas → 400/403.
- Proyecto sin `trusted`, con trust vencido o fingerprint distinto → 403 antes de iniciar Docker.
- Grupo con un componente no confiable → 403 y ningún componente ejecutado.
- Path que escapa por `..`, symlink o sustitución entre check/use → rechazo.
- Job de otra sesión, expirado o enumerado → 404/403 sin salida.
- Docker no verificable → 503; jamás respuesta de éxito.

## Fuentes

- RFC 6454 — The Web Origin Concept: <https://www.rfc-editor.org/rfc/rfc6454>
- OWASP — CSRF Prevention Cheat Sheet: <https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html>
- OWASP — Session Management Cheat Sheet: <https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html>
- OWASP — Docker Security Cheat Sheet: <https://cheatsheetseries.owasp.org/cheatsheets/Docker_Security_Cheat_Sheet.html>
