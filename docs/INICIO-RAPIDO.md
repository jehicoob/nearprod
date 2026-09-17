# Primeros pasos

## Actualizar sin perder lo registrado

No borres la carpeta .nearprod. Cierra el agente anterior, examina `config migrate --dry-run`, instala el binario nativo y ejecuta `config migrate --yes`. Sigue la secuencia exacta del README; `AGENT_OFFLINE` es benigno, los demás errores requieren revisión. Abrir un HOME diferente no migra el anterior.

## Herramientas y runtime

Abre `nearprod ui`. Introduce el código temporal que muestra el CLI. En Herramientas confirma Docker CLI, Compose y Buildx; Colima cuando usas Mac. Puedes conservar herramientas existentes o instalar/reparar usando Homebrew con revisión. No necesitas Node para el controlador.

En Runtime elige contexto `colima` y perfil `default` solo si son los tuyos. Diagnostica e inicia el perfil existente cuando sea necesario; no crees otra VM para cada app. El botón de inicio depende del estado real del perfil, no de un texto fijo.

## URLs

Accesos locales → puerto80 → revisar/confirmar Traefik. Si otro proxy ocupa el puerto, NearProd lo identifica pero no lo detiene. Alternativa8080 produce enlaces con :8080. Un error EACCES de una comprobación del host no es un motivo para ejecutar sudo; Docker tiene la decisión final de publicación si no se observa un listener.

Descubrir aplicaciones → raíz → seleccionar frontend/backend → grupo nuevo/existente → archivos y entorno → URLs → resumen. Las URLs pueden ser tienda.localhost y api-tienda.localhost. Selecciona **servicio y puerto interno**, por ejemplo frontend:5173 o caddy:80; `81:80` en Compose significa que el destino interno sigue siendo80.

Registrar no inicia contenedores. Revisar y aprobar muestra riesgos, volúmenes, comandos y cambios de red. Iniciar aplica; Reiniciar no actualiza env. Una etiqueta healthy no prueba negocio, CORS ni autenticación.

## Archivos y desarrollo

Elige Compose base + overrides en orden. No combines Compose independientes de dos repositorios. El selector automático de .env sigue las reglas de Compose; seleccionar archivos explícitos no inyecta por sí solo todas sus variables al contenedor. Los perfiles activan servicios opcionales definidos por tu proyecto, no grupos ni perfiles Colima.

Desarrollo usa montajes/recarga del proyecto. Prueba de imagen exige una configuración sin bind mounts de fuente/watch/dev reconocido; no certifica producción ni separa datos sola. Se puede vincular una base verify distinta. Watch solo se activa si Compose declara develop.watch y las capacidades de la CLI lo soportan.

## Datos

Infraestructura → instancia → volumen o carpeta dedicada → revisar → base/usuario → credenciales → vínculo consumidores → revisión de app → iniciar. Las carpetas nuevas se sugieren bajo ~/.nearprod/databases. Los datos existentes no se mueven. Consulta INFRAESTRUCTURA.md para permisos, backup y conflictos.

## Uso diario

```bash
nearprod list
nearprod status
nearprod up tienda
nearprod logs tienda/api --follow
nearprod stop tienda
nearprod config paths
nearprod config backup --yes
nearprod startup enable
```

Stop no borra datos. Cerrar NearProd no significa apagar la VM. Un modo de verificación es opcional. Para liberar memoria, detén solo las apps e instancias innecesarias; no uses una limpieza global de volúmenes.
