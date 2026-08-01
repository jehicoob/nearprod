# Decision record 0002: frontera de seguridad de la API privilegiada

- **Jira:** NPROD-15
- **Estado:** Propuesta — pendiente de aprobación técnica y de Producto
- **Fecha:** 2026-08-01
- **Responsables de aprobación:** responsable técnico de NearProd y Jehicoob López como Producto
- **Threat model asociado:** [`../security/threat-model-privileged-api.md`](../security/threat-model-privileged-api.md)

## Contexto

La API actual comparte origen con la UI y ejecuta operaciones privilegiadas sin autenticación, verificación de Host/Origin, CSRF ni autorización efectiva de `trusted`. El servicio está detrás de Traefik en `infra.localhost`, pero el proceso Node escucha en `0.0.0.0` dentro de `dev_proxy`, monta rutas del host y posee acceso de escritura al socket Docker.

La solución debe funcionar offline en macOS y Linux, mantener una experiencia local razonable y fallar cerrada. Esta decisión define la frontera; NPROD-16, NPROD-17 y NPROD-18 implementarán controles separados.

Este ADR es **normativo para la implementación futura**, no evidencia de controles ya desplegados. El baseline actual permanece inseguro frente a los escenarios descritos y no se aprueba ninguna operación sin autenticación/Host/Origin/CSRF ni el booleano `trusted` actual como excepción transitoria. NPROD-15 no incluye modificar código, conforme a la condición de parada del Spike.

## Fuerzas de decisión

1. Resistir solicitudes de webs remotas, DNS rebinding y procesos/contenedores locales no autorizados.
2. No depender de un proveedor de identidad, egress ni servicios cloud.
3. No guardar secretos reutilizables en URL, `localStorage`, `projects.yml`, repositorios o logs.
4. Separar identidad del propietario, autorización de API y confianza de cada proyecto.
5. Mantener compatibilidad con navegador, macOS, Linux, Traefik y un futuro CLI.
6. Producir errores verificables sin tomar la rama permisiva cuando el estado sea desconocido.

## Alternativas consideradas

| Alternativa | Ventajas | Desventajas/riesgos | Resultado |
| --- | --- | --- | --- |
| Loopback + `.localhost` + Origin, sin autenticación | Implementación y UX mínimas | No detiene procesos locales, contenedores, DNS rebinding mal controlado ni un proxy expuesto; Origin no autentica al usuario | Rechazada |
| Token bearer estático compartido con la UI | Evita CSRF si solo usa `Authorization` | Distribución/rotación difíciles; suele acabar en `localStorage`, código, URLs o logs; XSS lo extrae; no da sesión/revocación limpia | Rechazada para navegador; permitido solo como token CLI separado y acotado |
| OAuth/OIDC externo | Identidad y revocación maduras | Requiere egress, configuración de proveedor y callback; complejidad desproporcionada para uso local/offline | Rechazada como requisito base |
| mTLS, Unix socket o integración exclusiva con identidad del OS | Frontera fuerte y sin cookies en algunos clientes | UX, certificados/proxy y portabilidad macOS/Linux complejas; el navegador no consume Unix sockets directamente | Diferida como hardening opcional |
| **Propietario local + sesión server-side + Host/Origin + CSRF + trust por recurso** | Offline, revocable, compatible con navegador y CLI separado; controles en capas | Requiere bootstrap, estado seguro, expiración y disciplina en cada endpoint | **Recomendada** |

## Decisión propuesta

### 1. Todo `/api/*` es privilegiado

Deny-by-default. La tabla autoritativa de endpoints y requisitos está en el threat model. Solo un health check futuro, explícitamente minimalista, puede declararse anónimo.

Los controles se aplican en un pipeline común antes del handler:

```text
request limits → canonical Host → route classification → identity → Origin → CSRF
→ resource authorization/trusted → payload/path validation at use → action → audit/response
```

Un handler no puede optar por omitir un control; la clasificación central define excepciones explícitas.

### 2. Identidad local y sesiones

- En el primer arranque se genera un secreto de enrolamiento de al menos 256 bits con CSPRNG. Se muestra una sola vez por un canal local controlado (terminal/CLI), nunca en URL, respuesta anónima o logs persistentes.
- El propietario usa ese secreto para crear/verificar una credencial local. Solo se persiste un verifier resistente a fuerza bruta (por ejemplo, `scrypt` con salt y parámetros versionados), no el secreto en claro.
- El login crea un ID de sesión opaco de al menos 256 bits. La sesión vive server-side, tiene TTL absoluto e inactivo, rota después de autenticación/cambios sensibles y puede revocarse.
- El navegador recibe una cookie `HttpOnly; SameSite=Strict; Path=/`; se añade `Secure` cuando el origen use HTTPS. No se usan tokens persistentes en `localStorage`/`sessionStorage`.
- Se limita la tasa de login y bootstrap; errores no revelan si una credencial o sesión concreta existe.
- Un cliente CLI usa un token distinto, revocable y con scopes. Se transmite únicamente con `Authorization: Bearer`; nunca reutiliza cookies/CSRF del navegador.

El estado de autenticación vive en un directorio dedicado fuera del repositorio y de `projects.yml`, con permisos mínimos. Si permisos, formato, aleatoriedad o estado no pueden verificarse, el servicio no habilita operaciones privilegiadas.

### 3. Host, Origin y CORS

- El origen canónico se configura explícitamente (inicialmente `http://infra.localhost`) y el Host se compara de forma exacta después de parsing seguro; no se aceptan sufijos, subcadenas ni reflejo del cliente.
- Solicitudes con Host inesperado se rechazan antes de autenticar o leer cuerpos.
- Toda mutación con sesión navegador exige `Origin` exactamente igual al origen canónico. Origin ausente, `null`, inválido o múltiple se rechaza.
- CORS está deshabilitado por defecto. No se responde con `Access-Control-Allow-Origin: *`, no se refleja Origin y no se permiten credenciales cross-origin.
- `X-Forwarded-Host`, `X-Forwarded-Proto` y similares solo se aceptan de un proxy explícitamente confiable; el origen de seguridad no se deriva de cabeceras arbitrarias.

### 4. CSRF

- Cada sesión navegador recibe un token CSRF aleatorio ligado a esa sesión, entregado solo después de autenticar y conservado en memoria de la página.
- `POST`, `PUT`, `PATCH` y `DELETE` exigen `X-NearProd-CSRF`; la comparación es constante y el token rota con la sesión.
- `SameSite=Strict` y Origin son capas adicionales, no sustitutos del token.
- GET/HEAD no mutan estado. Si un endpoint GET desarrolla un side effect, cambia de método y hereda controles de mutación.
- Clientes bearer sin cookie no usan CSRF, pero sí autenticación, Host y scopes. Una petición que mezcla cookie y bearer se rechaza para evitar selección ambigua de identidad.

### 5. Significado de `trusted`

`trusted` **no es identidad de usuario ni una etiqueta descriptiva**. Significa que el propietario autenticado revisó y aprobó una identidad concreta de proyecto para operaciones que alcanzan filesystem/Docker.

- Ausente, `false`, ilegible, obsoleto o no verificable equivale a no confiable.
- Solo el propietario autenticado lo establece mediante una operación dedicada que exige Origin y CSRF, después de mostrar el preflight.
- Un proyecto recién creado siempre inicia no confiable, aunque el payload solicite `trusted: true`.
- La aprobación registra al menos: versión de schema, `approvedBy`, `approvedAt`, realpath/canonical path, runner, `project_name`, archivo Compose usado y fingerprint de los datos ejecutables relevantes.
- Cambiar path, symlink/target, runner, `project_name`, fuente, archivo Compose o fingerprint invalida la confianza antes del siguiente uso.
- La validación ocurre nuevamente en el punto de uso; no se autoriza una ruta y luego se vuelve a resolver sin comprobar identidad.

Operaciones que exigen confianza vigente:

1. start, stop, down y restart de proyecto;
2. cualquier acción de grupo para **todos** sus componentes antes de ejecutar el primero;
3. lectura de logs del proyecto;
4. futuras operaciones de build, exec, pull, delete, backup, restore o escritura en rutas del proyecto.

Detección y registro no requieren confianza previa, pero están autenticados, limitados a raíces configuradas y no ejecutan el proyecto. Consulta de dashboard requiere autenticación y minimiza datos; no convierte un proyecto en confiable.

El booleano actual puede conservarse temporalmente para migración visual, pero no satisface por sí solo este contrato. NPROD-17 debe introducir un registro versionado o un fingerprint equivalente.

### 6. Autorización, errores y auditoría

- El MVP tiene un rol `owner`; no se infieren permisos por network, Host, correo o presencia en configuración.
- Jobs se vinculan a la sesión/identidad que los creó; IDs son aleatorios, expiran y no otorgan acceso por sí solos.
- La respuesta externa distingue 401, 403, 400/409/413/429 y 503 según el threat model sin filtrar secretos, hashes, tokens, comandos sensibles o stack traces.
- Eventos de login, revocación, trust/untrust y acciones privilegiadas se auditan con timestamp, actor, recurso, decisión y resultado, redactando credenciales y payloads sensibles.
- Estado desconocido de Docker, configuración, auth o trust nunca se transforma en éxito o permiso.

## Consecuencias

### Positivas

- Se bloquea el acceso anónimo y se reducen CSRF, DNS rebinding y abuso desde procesos locales.
- Identidad de propietario y confianza de proyecto quedan separadas y revocables.
- La clasificación central evita controles copiados de forma inconsistente entre endpoints.
- Browser y CLI pueden coexistir sin compartir secretos ni modelos CSRF.

### Costes y riesgos

- Se añade enrolamiento/login a una herramienta local y estado persistente sensible.
- HTTP en loopback no permite exigir `Secure` de forma universal; TLS local queda como hardening recomendado.
- La sesión no mitiga una XSS same-origin; se requiere CSP, salida segura y revisión de UI.
- La API seguirá teniendo impacto potencial de host mientras conserve el socket Docker con escritura; NPROD-20/NPROD-21 deben reducirlo.
- El fingerprint de confianza debe diseñarse para evitar falsos permisos y revocaciones confusas entre macOS/Linux.

## Plan de implementación posterior

1. **NPROD-16:** middleware central, bootstrap/login/sesiones, Host/Origin, CSRF, CORS cerrado, rate limits y pruebas negativas.
2. **NPROD-17:** autorización `trusted`, registro/fingerprint, invalidación y preflight atómico de grupos.
3. **NPROD-18:** schema de payloads y paths seguros en el punto de uso.
4. **NPROD-20/NPROD-21:** decisión e implementación de aislamiento Docker; no asumir que la frontera HTTP vuelve seguro el socket.
5. **NPROD-23:** TTL, timeout, ownership y limpieza de jobs.

## Gate humano

Para aprobar, el responsable técnico y Producto deben confirmar:

- [ ] Todo `/api/*` es privilegiado y deny-by-default.
- [ ] Se acepta sesión server-side para navegador y token separado/scoped para CLI.
- [ ] Mutaciones navegador exigen Host/Origin exactos y CSRF.
- [ ] `trusted` queda ligado a identidad/fingerprint y se invalida ante cambios.
- [ ] Se acepta el riesgo residual de HTTP loopback hasta evaluar TLS local.
- [ ] NPROD-20 conserva el gate independiente sobre acceso al daemon Docker.

**Estado actual:** bloqueado para implementación hasta aprobación o hasta que se asignen preguntas/cambios concretos al responsable técnico y Producto.

## Fuentes primarias

- RFC 6454 — The Web Origin Concept: <https://www.rfc-editor.org/rfc/rfc6454>
- OWASP — Authentication Cheat Sheet: <https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html>
- OWASP — CSRF Prevention Cheat Sheet: <https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html>
- OWASP — Session Management Cheat Sheet: <https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html>
- OWASP — Docker Security Cheat Sheet: <https://cheatsheetseries.owasp.org/cheatsheets/Docker_Security_Cheat_Sheet.html>
- Node.js — `crypto.scrypt`: <https://nodejs.org/api/crypto.html#cryptoscryptpassword-salt-keylen-options-callback>
