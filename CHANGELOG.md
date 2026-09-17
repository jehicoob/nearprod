# 0.7.1

Corrige los saltos de layout provocados por indicadores temporales durante refrescos de Infraestructura, Runtime, métricas y configuración de rutas/perfiles. El estado de carga permanece accesible mediante regiones de estado fuera del flujo visual y los controles conservan etiquetas estables.

# 0.7.0

Backend y CLI Go nativos, panel React/TypeScript conservado y compilado dentro del ejecutable. Esquema4 bajo ~/.nearprod/config/catalog.json, migración explícita/automática desde3 con backup exacto, journal y protección contra un escritor antiguo. No reconfiguración de proyectos ni movimiento de volúmenes.

Directorios sugeridos para nuevas carpetas de datos ~/.nearprod/databases/<motor>/<instancia>. Vaults antiguos conservados, nuevos bajo config/resources. Backup privado de metadata/credenciales separado de SQL. LaunchAgent apunta al binario nativo, sinNode.

Correcciones auditadas: impedir volumen vacío tras pérdida de almacenamiento inicializado; permisos de archivo-secreto para entrypoint con UID de motor; conservar binding IDs antiguos al editar; rebuild realmente recrea; espera de cierre de agente antes de migrar; mapa de operación no compartido concurrentemente con respuesta y estado terminal tras liberar exclusión. Preserva arreglo de puerto80 y funciones anteriores.

Fuente y tests Go nuevos: cifras separadas de la suiteNode anterior. Compilador adjuntoGo1.23.2 antiguo, construcción vigente, macOS/Colima/Docker/SQL/Homebrew/launchd reales pendientes. Ver AUDITORIA/PRUEBAS antes de adoptar.
