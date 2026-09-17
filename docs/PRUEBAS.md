# Pruebas ejecutadas — 0.7.0 Go

## Resultado final

| Comprobación | Resultado | Alcance |
|---|---|---|
| Suite Go | **48 tests principales + 75 subtests aprobados (123 eventos PASS)** | FS, HTTP, Unix sockets, procesos, API, dominio, catálogo y contratos reales; Docker/Colima/Homebrew/launchctl usan dobles. |
| Harness browser opt-in | 1 omitido en suite general; ejecutado aparte | No es una prueba faltante contada como éxito. |
| Race detector | **Pasa**, sin carreras notificadas en ejecución final | `go test -race -count=1`; se guarda el fallo inicial encontrado y la corrección. No prueba todas las intercalaciones posibles. |
| go vet / TypeScript / UI build | **Pasan** | Compilación y análisis, no escaneo de vulnerabilidades. |
| React + agente Go | **24 comprobaciones pasan** | UI real vía puente HTTP/SSE con Engine/Traefik simulado. |
| Instalación/reinstalación nativa | **16 comprobaciones pasan** | Binario Linux instalado realmente, PATH sin Node, HTTP/assets, migración de catálogo3, backup y conservación de metadata. |
| Navegador nativo | **Bloqueado** | Chromium ERR_BLOCKED_BY_ADMINISTRATOR. No se cambió la política. |
| Docker y carpeta de datos | **Bloqueados, código77** | TOOL_MISSING; cero comprobaciones de motor/proxy real aprobadas. |
| Darwin arm64 / amd64 | **Compilan** | No se ejecutó Mach-O ni launchd/Colima en macOS. |
| Seguridad de dependencias/compiler | **Pendiente** | Compilador Go1.23.2 antiguo; no govulncheck/scan online. |

Cobertura instrumentada Go: **57.6% de sentencias**. No equivale a cobertura de React ni de los binarios externos de instalación, ni a cobertura de funcionalidad del usuario. No se suman los números como un porcentaje de garantía. Las216 pruebasNode anteriores no se presentan como ejecutadas contra Go.

## Evidencia

- `go-tests.jsonl`, `go-tests.exit`, `coverage.out`, `coverage.txt`: resultados exactos por caso.
- `race.txt`, `race.exit`: última comprobación; `history/race-before-fix.txt` conserva la carrera detectada y corregida.
- `native-package.json/txt`: checksum del binario realmente probado y cada paso.
- `browser-bridge.json/txt`:24 pasos de UI; capturas tienen datos de runtime SIMULADOS.
- `browser-native.json/txt`: bloqueo del navegador, sin transformarlo en aprobación.
- `acceptance-real/result.json` y `acceptance-folder/result.json`: intentos reales bloqueados, no mocks.
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
