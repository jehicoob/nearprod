# Construcción reproducible y frontera de seguridad

> Para el nuevo pipeline de distribución utiliza [DISTRIBUCION.md](DISTRIBUCION.md), `VERSION`, `.go-version` y `.node-version`. La sección siguiente describe los binarios históricos del ZIP original, no un nuevo release verificado.

## Binarios adjuntos

Se compiló con **Go 1.23.2 linux/amd64** porque era el único toolchain disponible. La descarga del Go vigente falló por resolución/conectividad del entorno. Go 1.23 ya está fuera de soporte; un binario incluye su biblioteca estándar. No se realizó `govulncheck` online. No confundir `go vet` y tests de carreras con un escaneo de vulnerabilidades.

La página oficial consultada en esta revisión publica Go 1.27.1 estable. Usa una rama soportada y su parche vigente para una compilación que vayas a promover a uso habitual. Consulta `https://go.dev/dl/` y `https://go.dev/doc/devel/release` en el momento de construir. El mínimo técnico de sintaxis es 1.23, no una recomendación de seguridad.

## Recompilar en tu Mac sin Node

La UI compilada y las fixtures de aceptación están en el código fuente. No necesitas npm si no cambias TypeScript:

```bash
go version
sh scripts/build.sh
./bin/nearprod-darwin-arm64 --identity
./bin/nearprod-darwin-arm64 install --configure-shell
```

El script utiliza tu SO y arquitectura, `CGO_ENABLED=0`, `-trimpath` y el linker de Go. En un Mac Intel el nombre final es `nearprod-darwin-amd64`. No realiza instalaciones globales ni descarga Go por su cuenta. El módulo Go no tiene dependencias de terceros ni `go.sum` que resolver.

Compilación cruzada explícita:

```bash
GOOS=darwin GOARCH=arm64 sh scripts/build.sh
GOOS=darwin GOARCH=amd64 sh scripts/build.sh
GOOS=linux GOARCH=amd64 sh scripts/build.sh
```

Un cross-build correcto no significa que launchd, Colima, montajes, DNS o Gatekeeper hayan sido probados en ese destino. Binarios de esta revisión sin firma de identidad Apple ni notarización. Nunca se pide desactivar las protecciones globales del equipo.

## Modificar React/TypeScript

```bash
npm ci --ignore-scripts
npm run typecheck
npm run build:ui
sh scripts/build.sh
```

Node/npm son herramientas de desarrollo del frontend. No arrancan con NearProd. `package-lock.json` fija TypeScript/@types; sus integridades provienen del lock de la entrega anterior. La UI usa React/ReactDOM 18.3.1 de producción, con licencias incluidas en `internal/webui/dist/vendor`; no un CDN. No se reinstalaron esas dependencias desde Internet ni se auditó su seguridad online aquí.

Mantén `examples/` y el embed de aceptación sincronizados; `build.sh` copia las fixtures. No incluyas `node_modules`, binarios test, `.env` privados o datos en tus releases.

## Pruebas de desarrollo

```bash
go test ./... -count=1 -timeout=120s
go test -race ./... -count=1 -timeout=120s
go vet ./...
npm run typecheck
python3 -m pip install -r scripts/requirements-test.txt
python3 -m playwright install chromium
python3 scripts/e2e.py
python3 scripts/test-package.py
```

El test de navegador compila un agente exclusivo de prueba con Docker simulado; no instala ni reemplaza el NearProd del usuario. `--bridge` es un modo alternativo explícito que no valida transporte/cookies/CSP nativos. No lo confundas con el E2E normal. Los scripts generan evidencia bajo `evidence/`.

`nearprod self-test` usa Docker real y no acepta una sustitución por mocks. Ejecutarlo necesita aprobación y un Engine Linux local. Consulta PRUEBAS.md antes.
