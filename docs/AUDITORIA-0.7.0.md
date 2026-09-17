# Auditoría de NearProd 0.7.0

**Dictamen: candidata implementada y probada en Linux; aceptación macOS/Colima pendiente.** No se certifica al 100 %. No se ha utilizado el catálogo privado actual ni las bases del usuario. No se ejecutaron Docker, SQL, Redis, Elixir, Traefik, Homebrew o launchd reales en este entorno.

## Alcance de la revisión

Se revisó la fuente de 0.6.1: catálogo de esquema 3, CLI/API, frontend, proxy e infraestructura. Se implementó el controlador en Go, no un lanzador que siga ejecutando Node. Se conservaron React/TypeScript y los recorridos existentes, adaptando la información de runtime y persistencia. Las pruebas Go son nuevas; no se contabilizan las pruebas Node de una versión anterior.

La revisión abarcó filesystem, identidad de Docker, generación de Compose/SQL, credenciales, operaciones concurrentes, HTTP, streaming, instalación, inicio de sesión, migración, backups, puertos, documentación y artefactos. Los mapas JSON conservan campos desconocidos; las entradas se validan. No se afirma que todo el modelo dinámico esté demostrado mediante tipos estáticos.

## Hallazgos, correcciones y regresiones

| ID | Riesgo | Tratamiento comprobado |
|---|---|---|
| G01 | Recrear identidades al cambiar de release. | Catálogo separado del ejecutable, migración 3 → 4 y comparación con fixtures generados por los módulos reales de 0.6.1. |
| G02 | Dos controladores escriben catálogos distintos. | Socket Unix compartido, guard de exclusión nativa y marcador que bloquea la escritura desde 0.6. Se rechaza un catálogo dual. |
| G03 | Migración interrumpida o datos ausentes. | Backup byte por byte, hashes, journal y escritura atómica. Reanudación verificada; no se inicializa un catálogo vacío sobre rastros de recursos. |
| G04 | Cambiar routers, alias y redes del proxy. | Funciones Go comparadas con resultados del código JavaScript anterior. Se conservan owner, UID, nombres y puertos. |
| G05 | Mover secretos o datos montados por contenedores activos. | Los vaults antiguos mantienen su ruta bajo infra; los nuevos se guardan en config/resources. Los volúmenes y carpetas de motores no se mueven. |
| G06 | Recrear vacío un volumen perdido. | La identidad del almacenamiento se verifica antes de declarar el recurso listo, incluso si el contenedor aparece saludable. Una instancia inicializada sin datos se bloquea. |
| G07 | El motor no puede leer su secreto después de cambiar de UID. | Archivo de secreto específico legible desde el contenedor dentro de una carpeta privada; vault 0600. La configuración está probada; falta ejecutar los entrypoints reales. |
| G08 | Editar un vínculo crea otra ID. | Se conserva el identificador anterior de la relación stack/mode/database. Regresión con el formato de ID legado. |
| G09 | Un checkout inválido bloquea aplicaciones independientes. | Validación localizada y resultados parciales por grupo, sin rollback inventado. |
| G10 | No se puede detener ni leer logs porque el YAML se rompió. | Se utilizan recursos observados cuya propiedad e identidad de Engine se comprueban, sin depender del archivo actual para recuperación. |
| G11 | Construir no aplica la nueva imagen o configuración. | Rebuild usa construcción y recreación explícitas. Restart continúa reiniciando la instancia existente. |
| G12 | Puerto 80 rechazado por permisos del host. | Se conserva el fallback de 0.6.1: distingue EACCES/EPERM, listener real y resultado incierto. La publicación definitiva corresponde a Docker. |
| G13 | Mapa de operación modificado mientras se serializa una respuesta. | **El detector de carreras reprodujo DATA RACE.** Se corrigió con una respuesta inicial independiente y snapshots finales protegidos. Se incluye evidencia anterior y posterior. |
| G14 | Se anuncia una operación terminada antes de liberar su exclusión. | El resultado terminal se publica después de persistir y liberar el objetivo. Caché final acotada para errores de persistencia. |
| G15 | Inicio y cierre concurrentes de operaciones/Watch. | Registro de workers bajo bloqueo, marca de cierre y drenado de operaciones antes de esperar seguimientos. Probado con detector de carreras. |
| G16 | El comando stop termina antes de liberar el catálogo. | La CLI espera el cierre del socket compartido con timeout. No mata procesos por un PID supuesto. |
| G17 | Buffers de procesos, logs o backups sin límites. | Límites de salida, SSE con backpressure y cancelación, seguidores acotados y streams binarios a disco. |
| G18 | Exponer Docker al navegador o al proxy. | Agente HTTP local autenticado; Traefik usa archivos, sin socket Docker ni dashboard expuesto. |
| G19 | Contraseñas en argumentos, catálogo o vista predeterminada. | Vault privado, SQL por stdin, archivos de cliente, revelado explícito y pruebas de redacción. La redacción no garantiza ocultar cualquier secreto en logs arbitrarios. |
| G20 | Mezclar permisos entre bases. | Bases y cuentas por proyecto, permisos limitados y Redis dedicado. Se probaron los contratos; la comprobación de aislamiento real está preparada en self-test. |
| G21 | Una restauración pisa datos en uso. | Manifiesto/hash y destino vacío no vinculado; cuenta limitada. Un error puede dejar objetos parciales y se informa sin fingir rollback. |
| G22 | El instalador de herramientas sustituye configuración ajena. | Opciones reales del proveedor, lista permitida y confirmación. Reparación por merge con backup; no upgrade global ni sustitución de instalaciones desconocidas. |
| G23 | NearProd sigue dependiendo del Node seleccionado. | Binario nativo estable con assets embebidos. Instalación y ejecución probadas sin Node en PATH; reinstalación conserva el catálogo. |
| G24 | El inicio de sesión duplica agentes o mantiene Node. | Label del catálogo conservado y plist nativo, con exclusión compartida. XML y filesystem reales; launchctl simulado. |
| G25 | La migración rompe los campos y recorridos de la UI. | React y agente Go reales mediante puente HTTP/SSE; pruebas de registro, URLs, logs, infraestructura, vínculos y configuración permanente. |
| G26 | Redondear campos JSON desconocidos durante la migración. | Decodificación conservando números JSON y regresión con un entero mayor que la precisión de float64. |
| G27 | Historial serializado demasiado grande para volver a abrir el catálogo. | Presupuesto de 4 MiB, conservación de operaciones activas y truncado explícito de resultados individuales excesivos. Se mide JSON serializado, no solo el número de líneas. |
| G28 | Toolchain fuera de soporte. | **Pendiente:** solo estuvo disponible Go 1.23.2. La descarga de un compilador vigente falló por conectividad. Se exige recompilar con Go soportado antes de promover los binarios. |

## Evidencia y límites

Las cifras finales, cobertura y comandos están en PRUEBAS.md. No se sustituyen pruebas reales por mocks para aumentar el número de resultados verdes. El detector de carreras pasó después de corregir los defectos encontrados, pero no demuestra todas las intercalaciones posibles.

El navegador nativo quedó bloqueado por una política administrativa. El puente ejercita React y HTTP/SSE del agente, pero no certifica cookies, CSP, resolución ni navegación del navegador del usuario. Esas cabeceras y la autenticación sí tienen pruebas HTTP independientes.

La cobertura Go instrumentada no incluye el proceso externo de instalación ni la UI y no cubre todas las ramas de aceptación real. El FakeRunner no implementa toda la semántica de Compose o de la autorización de los motores.

No se ejecutaron escáneres online de dependencias ni imágenes. Go 1.23.2 ya no está soportado; go vet no reemplaza una revisión de vulnerabilidades. Los binarios Mac compilados no fueron ejecutados ni notarizados. La distribución es una candidata de validación, no un paquete certificado para producción.

## Antes de usarla diariamente

Compilar con un parche vigente de Go, ejecutar la aceptación Docker real, validar el arranque al login y abrir un proyecto propio con su base. Comprobar URLs, CORS, recarga, persistencia, credenciales y restauración. Medir recursos en el M1; el cambio de lenguaje no elimina el consumo de Colima ni de las aplicaciones.

Las limitaciones incluyen Compose include/extends, network_mode para redes gestionadas, cambios mayores de motor, traslado automático de datos, proxies externos, TLS/DNS administrado y soporte universal de Phoenix/Laravel. No se certifica el contenido de repositorios privados ni un filesystem frente a cualquier fallo físico.

## Distribución

Se incluyen fuente Go, frontend TypeScript y assets, ejemplos, pruebas y documentación; binarios Mac ARM64, Mac Intel y Linux x86_64. No se distribuye el ejecutable del agente de prueba con FakeRunner, ni node_modules, fuentes tipográficas o datos personales. Los hashes permiten comprobar integridad, no autenticidad o seguridad.

La instalación no borra metadata ni datos y la migración no ejecuta Docker. Las pruebas reales sí crean y limpian sus propios recursos temporales, verificando contexto, Engine y owner; no realizan limpieza global.
