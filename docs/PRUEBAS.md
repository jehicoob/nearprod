# Pruebas ejecutadas — 0.8.0 Go

## Resultado final

| Comprobación | Resultado | Alcance |
|---|---|---|
| Suite Go | **83 tests principales + 86 subtests aprobados (169 eventos PASS)** | FS, HTTP, Unix sockets, procesos, API, dominio, catálogo, plataformas y autoactualización; Docker/Colima/Homebrew/launchctl usan dobles salvo las aceptaciones opt-in. |
| Harness browser opt-in | 1 omitido en suite general; ejecutado aparte | No es una prueba faltante contada como éxito. |
| Race detector | **Pasa**, sin carreras notificadas en ejecución final | `go test -race -count=1`; se guarda el fallo inicial encontrado y la corrección. No prueba todas las intercalaciones posibles. |
| go vet / TypeScript / UI build | **Pasan** | Compilación y análisis, no escaneo de vulnerabilidades. |
| React + agente Go | **36 comprobaciones pasan en Chromium** | UI compilada vía HTTP/SSE con Engine/Traefik simulado; incluye aplicaciones, grupos, raíces y las opciones seguras/destructivas de bases. |
| Instalación/reinstalación nativa | **18 comprobaciones pasan** | Binario instalado realmente en Linux/WSL2; añade eliminación CLI de grupos vacíos y retirada de raíces sin dependencias. PATH sin Node, HTTP/assets, migración, backup y conservación de metadata. |
| Navegador nativo | **25 comprobaciones pasan en macOS arm64** | Chromium real contra agente Go aislado; no usa catálogo ni Docker reales. |
| Docker e infraestructura | **16 comprobaciones pasan en macOS/Colima** | PostgreSQL, MySQL y Redis reales con carpetas, credenciales, persistencia, backup/restore, vínculos y limpieza. |
| Aceptación web Docker | **Parcial: 4/5 pasos pasan** | Engine, Compose, Traefik y limpieza pasan; la ruta HTTP del fixture falla igual en 0.8.0 y en el binario oficial 0.7.1. No se atribuye al PR WSL2, pero sigue pendiente. |
| Darwin arm64 / amd64 | **arm64 ejecutado; ambos compilan** | Smoke, paquete y aceptación en Apple Silicon; no se probó Mach-O amd64 ni login launchd real. |
| Seguridad | **Revisión independiente aprobada, sin hallazgos medios/altos pendientes** | Purga con confirmación independiente, ownership físico fail-closed, grants limitados, observación Docker fresca y rollback verificable del vault. |

Cobertura instrumentada Go: **63.1% de sentencias**. No equivale a cobertura de React ni de los binarios externos de instalación, ni a cobertura de funcionalidad del usuario. No se suman los números como un porcentaje de garantía. Las 216 pruebas Node anteriores no se presentan como ejecutadas contra Go.

## Evidencia

- `RESULTADOS.json`: resumen de la validación 0.8.0; `coverage.out` y `coverage.txt`: cobertura instrumentada actual.
- `history/race-before-fix.txt`: conserva la carrera detectada y corregida en la entrega anterior; la ejecución final de `make check` volvió a pasar el detector.
- `native-package.json/txt`: checksum del binario realmente probado y cada paso.
- `browser-bridge.json/txt`: evidencia histórica del puente; las capturas tienen datos de runtime SIMULADOS.
- `browser-native.json`: 36 pasos aprobados, incluidos los específicos de Linux/WSL2, archivo/restauración de aplicaciones y bases, purga con/sin backup, grupos vacíos, raíces sin dependencias y ciclo de instancias; `go-infraestructura-archivada.png` y `go-infraestructura.png` muestran los estados archivado y restaurado; `browser-native-macos.json`: 25 pasos históricos aprobados en macOS.
- `acceptance-real/result.json`: fallo HTTP de la aceptación web 0.8.0; `acceptance-baseline-0.7.1/result.json`: mismo fallo con la release oficial anterior.
- `acceptance-folder/result.json`: 16 comprobaciones reales de PostgreSQL, MySQL y Redis aprobadas en macOS/Colima con 0.8.0.
- `build-*.txt`, manifiesto y hashes: compilador y destinos de binarios.

El puente de pruebas carga assets compilados y conecta fetch/EventSource con HTTP/SSE real del agente; no prueba transporte/cookies/CSP de una navegación nativa completa. HTTP, autenticación, Host/Origin y cabeceras sí se prueban además desde clientes HTTP reales en la suite Go. El puente y el FakeRunner están fuera del binario instalado.

## Reproducir pruebas desde fuente

Consulta COMPILAR.md y usa Go actualmente soportado.

```bash
go test ./... -count=1 -timeout=120s
go test -race ./... -count=1 -timeout=120s
go vet ./...
npm ci --ignore-scripts
npm run typecheck
npm run build:ui
sh scripts/build.sh
python3 -m pip install -r scripts/requirements-test.txt
python3 -m playwright install chromium
python3 scripts/e2e.py
python3 scripts/test-package.py
```

`python3 scripts/e2e.py --bridge` es la modalidad alternativa de revisión, **no el transporte nativo**. Los módulos de prueba simulan Docker; sus nombres y mensajes no deben utilizarse como pruebas de una API Laravel/Phoenix real.

## Aceptación nativa real del usuario

Cierra el agente anterior, migra y conserva backups. No ejecutes sobre un contexto remoto/VPS. Colima debe estar activo y los plugins Docker funcionales. No se arranca/reconfigura la VM durante la prueba.

```bash
nearprod self-test --yes --context colima
nearprod self-test --yes --context colima --infra-only --folder
```

Comprobaciones preparadas: proxy/routers de frontend y API, Host desconocido, CORS en fixture, logs, stop/restart y recuperación con YAML eliminado; Python/PHP reload y modo imagen; imagen Elixir; PG/MySQL credenciales separadas y acceso denegado a DB ajena, datos tras recreación, backup/restore a otra base, vínculos/variables, Redis persistente. Las instancias se prueban secuencialmente; no se certifica encender todo con 2GiB.

La prueba crea recursos temporales bajo HOME para montajes Colima, usa un catálogo y owner aleatorios, valida Engine/endpoint antes de limpiar y elimina solo sus contenedores, redes, volúmenes y carpetas temporales. No prune, no datos/proyectos/metadata del usuario, no DNS, no certificados ni recursos de Colima. Conserva imágenes/caché. Fallo de limpieza es fallo; conserva directorios para diagnóstico. No ejecuta Homebrew ni launchd.

Reporta `nearprod-acceptance-*/result.json`.0: casos ejecutados aprobados;77: bloqueado por herramienta;1: error real;2: uso incorrecto. Un PASS de fixtures no prueba los Compose privados del usuario ni el comportamiento del navegador. Abre las URLs de un proyecto propio, prueba endpoints/DB y recarga. Cierra/inicia sesión para validar startup, y mide memoria en el M1.

## Criterio antes de uso diario

Recompilar con parche vigente de Go, aceptación Docker real aprobada, ejecución macOS/login y un proyecto propio. No mover/borrar las carpetas anteriores hasta validar conservación, credenciales y backups. La migración no cambia versiones de motores ni mueve datos físicos; no hay una promesa universal de compatibilidad o seguridad.
