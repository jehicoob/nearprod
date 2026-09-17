# Especificación de NearProd 0.7.0

## Meta y exclusiones

Migrar el controlador/CLI a Go, conservar panel React/TypeScript y continuidad del catálogo 0.6.1, no agregar otra VM ni un servicio de DB para que NearProd funcione. No es un wrapper Go que arranca Node. El binario incorpora assets y fixtures de aceptación; Docker/Compose/Colima siguen siendo procesos externos explícitos.

Mantener aplicaciones, grupos, acciones, salud, logs, Watch, proxy local, runtime, herramientas, infraestructura, credenciales, backups, startup y seguridad de transporte. El cambio de lenguaje no certifica un motor que no se haya ejecutado ni elimina el consumo de los proyectos.

## Arquitectura

`cmd/nearprod` inicia CLI o servicio. `internal/nearprod` contiene fronteras HTTP, runner, catálogo, coordinador y adaptadores. `internal/webui` incorpora assets compilados. `internal/acceptanceassets` incluye ejemplos pequeños para aceptación real. No hay código JavaScript de backend ni proceso auxiliar Node.

La metadata utiliza mapas JSON con validación explícita y conservación de campos desconocidos, no structs rígidos que descarten propiedades 0.6. Las operaciones, runner, servicios de infraestructura, buses y componentes de sistema son tipos Go. Eso no significa que cada entrada dinámica sea estáticamente demostrada: se prueban validadores, límites y relaciones.

### ProcessRunner

Argumentos sin host-shell, cwd/entorno explícitos, timeout/cancelación de grupo de procesos, buffer acotado, stream FD para SQL y dumps binarios. Secretos no viajan en argv ni se guardan por defecto en el historial. La redacción es mitigación, no garantía de que cualquier log de aplicación esté libre de secretos.

### Catálogo y migración

Schema 4 bajo config/catalog.json. Unix socket compatible con 0.6.x más flock de exclusión nativa. JSON validado antes de escribir; backup hash/journal, atomic rename y permisos privados. Reanudar un cambio interrumpido exige coincidencia del origen/destino; ninguna ambigüedad crea un catálogo vacío. No tocar volúmenes ni mover carpetas/vaults de recursos existentes.

Las identidades de contenedor/red/volumen/proyecto/Traefik dependen de owner/UID preservados. El release y schema son independientes. Huellas de confianza Go invalidan aprobación antigua una vez; registro y configuración se mantienen.

### Coordinación y observación

Locks por objetivo, un build pesado concurrente, lock global para mantenimiento. El estado terminal de una operación se publica después de persistir y liberar su exclusión; la copia de respuesta no comparte un mapa mutable con una goroutine. Resultados de un grupo son parciales, nunca rollback inventado. El historial persistido tiene un presupuesto de 4 MiB de JSON serializado para que los logs no impidan volver a abrir el catálogo.

Snapshot, eventos de Docker y reconciliación periódica. Logs seleccionados SSE, límite de seguidores y backpressure; no entrega exactamente una vez. Cerrar una vista no para su aplicación. Watch se delega a Compose con capacidades comprobadas, sin poda automática de imágenes.

### Docker/Compose/Traefik

Contexto local Unix, Engine Linux e identidad verificados. No cambiar contexto global. Modelos efectivos de Compose vía CLI JSON, archivos/env/perfiles explícitos y ordenados. El scanner offline da pistas, no pretende implementar YAML/Compose completo. No admite include/extends remotos o la integración administrada de network_mode.

Start reconcilia; restart no aplica configuración; rebuild construye y recrea explícitamente. Stop/logs/estado de recursos propios no depende de que el YAML siga siendo válido. Recursos ajenos de solo lectura hasta adopción explícita.

Traefik compartido por catálogo, file provider, sin socket Docker/dashboard, HTTP loopback, dominios cortos .localhost. Conserva ports del proyecto; publica únicamente routers de servicios realmente conectados. EACCES del bind Node/host anterior no equivale a imposibilidad del Docker; el fallback distingue listener ocupado y resultado desconocido antes de delegar la publicación final.

### Infraestructura

PostgreSQL 17/18 y MySQL 8.4, bases/usuarios por proyecto; Redis 7.4/8 dedicado. Imagen pin/arquitectura, permisos por cuenta, red de datos por instancia y consumidores seleccionados. Datos de DB propia del proyecto no se sustituyen. Persistencia por volumen o carpeta dedicada; por defecto las nuevas carpetas se sugieren bajo databases/<engine>/<id>.

Vaults privados e inicialización idempotente sin rotar credenciales. Si un recurso inicializado pierde datos, se bloquea; no se recrean vacíos por comodidad. SQL aprovisionado por stdin, probes limitados y backup/restore con archivos y hashes. Restauración solo a destino vacío no vinculado, bajo credencial limitada. No upgrade mayor ni traslado in situ de almacenamiento.

### Herramientas/startup

Detectar ejecutables y gestores; usar Homebrew macOS para operaciones permitidas/previsualizadas. No administrar versiones Node desde NearProd ni eliminar instalaciones ajenas. Reparación de plugins por merge con backup de Docker config.

LaunchAgent de usuario con ruta del ejecutable nativo, registro heredado identificable, RunAtLoad sin KeepAlive. Solo arranca el controlador al login. Disable no detiene apps ni el agente actual. No registrar rutas efímeras ni credenciales de apps.

### HTTP

Loopback, autenticación local, código de un uso de duración limitada, cookie Strict/HttpOnly, comprobaciones Host/Origin/FetchSite y CSP. La CLI usa bearer desde metadata privada. Detención y emparejamiento no quedan a disposición de una página externa. No hay un panel en el contenedor con socket Docker.

## Aceptación

Migrar un catálogo realista de 0.6 sin modificar sus identidades, reinstalar y reabrir viendo los mismos proyectos, mantener vaults y rutas, y bloquear agente antiguo. Ejecutar agente/CLI/assets reales sin Node en PATH. E2E de UI contra Go y pruebas de concurrencia con race detector. Ejecutar además Docker/Colima/Traefik/SQL reales en el equipo objetivo; un bloqueo debe quedar como bloqueo. No sustituirlo por un mayor número de mocks.

Fronteras públicas y comandos se describen en CLI.md y GUIA-DE-USO.md. Los defaults de UI no escriben datos ni ejecutan proyectos hasta una acción explícita aprobada.
