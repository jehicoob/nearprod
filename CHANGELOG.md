# 0.9.0

Añade gestión segura del ciclo de vida de grupos, raíces, aplicaciones e infraestructura. Las operaciones destructivas usan vistas previas, confirmaciones específicas y revalidación; las aplicaciones y bases pueden archivarse y restaurarse, y las purgas SQL se reanudan de forma segura después de interrupciones.

Incorpora acceso MCP local por `stdio` mediante `nearprod mcp serve`, con doce herramientas que reutilizan el agente loopback como único escritor. El panel muestra la configuración necesaria sin exponer credenciales ni añadir usuarios o tokens MCP.

Migra el catálogo a schema 5 con backup exacto y recuperación fail-closed, conserva identidades y datos existentes, sincroniza rutas Traefik al archivar/restaurar y exige observación Docker fresca antes de retirar recursos.

# 0.8.0

Añade capacidades de host autoritativas para macOS, Linux, WSL y WSL2. Linux utiliza Docker nativo sin intentar administrar Colima, Homebrew o launchd; macOS conserva Colima y Docker nativo. PostgreSQL 18 con carpeta se bloquea en Docker nativo Linux cuando no puede garantizarse la compatibilidad de permisos.

Incorpora `nearprod update --check` y `nearprod update` para instalaciones manuales. La actualización requiere confirmación, descarga únicamente el asset exacto de la última release estable, contrasta el digest de GitHub con `SHA256SUMS.txt`, valida el archive y la identidad del ejecutable, conserva la versión anterior y elimina temporales. Homebrew sigue siendo dueño de sus instalaciones.

# 0.7.1

Corrige los saltos de layout provocados por indicadores temporales durante refrescos de Infraestructura, Runtime, métricas y configuración de rutas/perfiles. El estado de carga permanece accesible mediante regiones de estado fuera del flujo visual y los controles conservan etiquetas estables.

# 0.7.0

Backend y CLI Go nativos, panel React/TypeScript conservado y compilado dentro del ejecutable. Esquema4 bajo ~/.nearprod/config/catalog.json, migración explícita/automática desde3 con backup exacto, journal y protección contra un escritor antiguo. No reconfiguración de proyectos ni movimiento de volúmenes.

Directorios sugeridos para nuevas carpetas de datos ~/.nearprod/databases/<motor>/<instancia>. Vaults antiguos conservados, nuevos bajo config/resources. Backup privado de metadata/credenciales separado de SQL. LaunchAgent apunta al binario nativo, sinNode.

Correcciones auditadas: impedir volumen vacío tras pérdida de almacenamiento inicializado; permisos de archivo-secreto para entrypoint con UID de motor; conservar binding IDs antiguos al editar; rebuild realmente recrea; espera de cierre de agente antes de migrar; mapa de operación no compartido concurrentemente con respuesta y estado terminal tras liberar exclusión. Preserva arreglo de puerto80 y funciones anteriores.

Fuente y tests Go nuevos: cifras separadas de la suiteNode anterior. Compilador adjuntoGo1.23.2 antiguo, construcción vigente, macOS/Colima/Docker/SQL/Homebrew/launchd reales pendientes. Ver AUDITORIA/PRUEBAS antes de adoptar.
