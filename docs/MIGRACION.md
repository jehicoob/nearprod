# Migración, actualizaciones y conservación de datos

## El contrato

La versión del ejecutable es `0.9.0`. La versión de esquema es **5**. Son cosas diferentes: actualizar un binario compatible no crea otro catálogo ni renombra recursos. El catálogo pertenece al usuario/NEARPROD_HOME, no al checkout, al paquete instalado ni al directorio de descarga.

0.6.1 ya persistía `~/.nearprod/catalog.json`. Por ello, reinstalar no debería requerir registrar nuevamente. Cambiar de NEARPROD_HOME, borrar el catálogo, arrancar otra instalación o apuntar a otro Engine sí puede mostrar un entorno distinto. `nearprod config paths` muestra exactamente el catálogo elegido sin arrancar Docker.

## Antes de actualizar

Cierra el agente anterior, conserva el catálogo y haz backups de datos relevantes con herramientas apropiadas del motor. No copies directorios de DB en uso como si fueran backups consistentes. No ejecutes ambas versiones sobre el mismo HOME. Los contenedores pueden seguir ejecutándose: migrar metadata no llama a Docker ni los detiene.

```bash
nearprod agent stop
./bin/nearprod-darwin-arm64 config migrate --dry-run
./bin/nearprod-darwin-arm64 install --configure-shell
# Terminal nueva:
nearprod config migrate --yes
nearprod ui
```

Para un HOME personalizado, configura el mismo valor antes de todos esos comandos. No cambies ese valor para resolver un error de migración: abrirías otro catálogo.

## Qué hace schema 3 → 5

1. Adquiere el socket `agent.sock` usado por Node y un guard de bloqueo Go. Un agente antiguo vivo impide el cambio, sin matar un PID supuesto.
2. Valida JSON, versión exacta y estructura. Un registro corrupto, de una versión futura, duplicado o con referencias inválidas NO se sustituye por uno vacío.
3. Guarda un backup privado **byte por byte** en `backups/config` y un journal de migración.
4. Escribe de forma atómica `config/catalog.json`, conservando campos desconocidos y las identidades de recursos.
5. Sustituye el `catalog.json` raíz por un marcador de versión -1, que el código 0.6.x rechaza en lugar de escribir un segundo catálogo.
6. Marca la migración terminada. Un nuevo arranque no la repite; si quedó interrumpida, verifica hashes y reanuda o bloquea ante conflictos.

## Qué hace schema 4 → 5

1. Valida primero el catálogo canónico `config/catalog.json` y crea en `backups/config` una copia privada **byte por byte** del schema 4.
2. Registra en `config/migration.json` los hashes de origen, backup y destino antes de sustituir el catálogo de forma atómica.
3. Conserva todas las identidades y añade solo la metadata de migración necesaria para auditar el origen. No ejecuta Docker, SQL ni mueve datos físicos.
4. Si el proceso se interrumpe antes o después de escribir el destino, el siguiente arranque compara los hashes y reanuda o confirma la misma migración. Cualquier copia divergente bloquea con `MIGRATION_CONFLICT`.

El cambio de versión evita que un binario anterior que solo comprende schema 4 escriba nuevas identidades sobre un catálogo con snapshots archivados que no sabe validar.

Nunca elimina el original sin el backup; los archivos privados quedan 0600. Un journal no transforma una interrupción de Docker en un rollback. Las operaciones antiguas que quedaron running/queued pasan a interrumpidas para no afirmar que continúan ejecutándose bajo el nuevo proceso.

## Se conservan sin recalcular

Owner del catálogo, UID de cada stack, nombres Compose, UID/nombre de instancias, red y volumen, nombres de base/usuarios, endpoint e Engine ID, dominios, grupos, archivos Compose en orden, referencias env, perfiles, toolPaths y relaciones de consumidores. Las claves de routers/alias de Traefik son compatibles con las funciones reales de 0.6.1; hay fixtures comparativos generados por ese código.

Las contraseñas no se regeneran al migrar. Los vaults antiguos siguen bajo `infra/<uid>`, porque mover secretos/config mientras contenedores los tienen montados puede alterar sus rutas. Nuevos recursos usan `config/resources/<uid>`. No hay dos copias canónicas en conflicto.

La huella de aprobación cambia porque el generador del controlador cambió. La UI conserva los proyectos, pero requiere **Revisar y aprobar** antes de una nueva mutación. Reiniciar procesos existentes tampoco equivale a aplicar nueva configuración.

## Datos físicos

`databases/<motor>/<instancia>/data` es una ruta sugerida para NUEVAS instancias cuando eliges **Carpeta local**. No obliga a abandonar volúmenes Docker. La ruta puede ser otra carpeta explícita y dedicada; se verifica acceso/marcador/solapamientos. En macOS debe estar visible en los montajes de Colima.

Un PostgreSQL con varias bases comparte un único directorio de datos. No se crean tablespaces ni carpetas físicas por schema. Para separar físicamente necesitas instancias diferentes. Para separar datos de desarrollo/prueba dentro de una instancia puedes crear bases diferentes.

Una instancia ya inicializada cuyo volumen o directorio ha desaparecido **se bloquea**. No se crea un volumen vacío con el mismo nombre para que el panel aparente funcionar: revisa el Engine/VM, recupera el volumen o restaura deliberadamente en otra instancia.

Ni `install`, ni `config migrate`, ni quitar una aplicación del catálogo mueve o elimina datos. No cambies manualmente nombres de volumen/UID/Engine para forzar una adopción.

## Backups de configuración

`nearprod config backup --yes` genera un tar.gz 0600 con catálogo canónico, journal/marcador y vaults referenciados, más un manifiesto de integridad. No incluye bases físicas, imágenes, proyectos completos, `.env` de repositorios ni toda la configuración de tu cuenta macOS. La configuración del proxy y overlays puede regenerarse desde metadata al revisar/iniciar.

Guarda estos backups en un lugar privado externo si necesitas recuperación frente a pérdida del disco. Contienen contraseñas en claro protegidas por permisos, no por cifrado. La app no implementa un gestor de secretos empresarial.

## Volver a una versión anterior

No hay un botón de downgrade. **Un binario anterior no debe escribir schema 5.** Los binarios que solo admiten schema 4 lo rechazan; 0.6.x además queda bloqueado por el marcador del catálogo raíz. Conservar el ejecutable anterior no basta para volver atrás. Un rollback de metadata solo se considera con ambos agentes cerrados, backup íntegro y sin cambios posteriores de recursos que hagan obsoleta la copia anterior.

Antes de una restauración manual, archiva el HOME actual completo sin copiar en vivo bases como si fueran consistentes, conserva ambos catálogos y verifica hashes. Restaura solo metadata compatible en un procedimiento offline; no borres volúmenes ni datos. El documento no propone `rm -rf ~/.nearprod` ni borrar config para ocultar el marcador. Para recuperar información creada después de migrar se necesita un plan específico; prefiere corregir hacia delante con 0.7.

## Señales de bloqueo

- `AGENT_RUNNING`: cierra el agente vivo; no borres su socket.
- `DUAL_CATALOG`: ambos catálogos parecen activos; conserva ambos y resuelve el origen.
- `MIGRATION_CONFLICT`: cambió origen/destino desde el journal; no se adivina cuál gana.
- `CATALOG_MISSING`: hay rastros de infraestructura pero no catálogo; recupera metadata, no inicialices vacío.
- `SCHEMA_UNSUPPORTED`: formato futuro/corrupto o marcador de downgrade; no lo conviertas a mano.
- `INSTANCE_DATA_MISSING`/error de almacenamiento: revisar/recuperar datos antes de iniciar, no recrear vacíos.

Si el agente viejo no responde, verifica el proceso y usa el comando de detención de esa instalación. No mates procesos de Node indiscriminadamente; pueden pertenecer a tus proyectos.
