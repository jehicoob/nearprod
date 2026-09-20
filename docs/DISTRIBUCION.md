# Distribución de NearProd: GitHub Releases y tap personal

## Arquitectura concreta

Se conserva `npm run build:ui` (TypeScript + `scripts/copy-ui.mjs`), `internal/webui/dist`, el `go:embed` de `internal/webui/embed.go` y `cmd/nearprod/main.go`. React/ReactDOM ya están vendorizados con sus licencias; no se descarga un CDN al ejecutar el programa. No se añade Vite, pnpm, un servidor Node, un nuevo router SPA ni otra estructura de frontend.

`make check` valida el código. `scripts/build.sh` compila un binario. `scripts/release.py` reutiliza ese build para cuatro plataformas y empaqueta solo ejecutable, licencias, instrucciones y metadatos. Los archivos de frontend, fixtures y sus licencias necesarias quedan dentro del binario existente.

Se eligió **GitHub Actions + el build existente + Python estándar** en lugar de GoReleaser. Para cuatro targets Unix y un formato de archive, se evita añadir otro ejecutable/configuración de build. La generación de Homebrew se limita a una fórmula con URLs y hashes reales. No se mantiene un instalador remoto `curl | sh`; el actualizador integrado consume estos mismos assets y no publica releases.

## Versión y herramientas

- `VERSION`: única versión de la aplicación, inicialmente `0.7.0`.
- `.go-version`: toolchain exacto usado para releases.
- `.node-version`: Node exacto usado para construir TypeScript.
- `package-lock.json`: dependencias del frontend. El manifiesto privado de herramientas no necesita una versión de publicación npm.
- `go.mod`: mínimo sintáctico del código, no selección del Go con el que se distribuye.

El linker inyecta `nearprod/internal/nearprod.Version`. `nearprod version`, `--version`, `--identity` y `/api/health` usan esa variable. `go run`/`go test` sin el script informan `dev`; `make build` inyecta `VERSION`. No se copia la versión manualmente en Go, YAML ni fórmulas.

Los toolchains inicialmente fijados se verificaron en las páginas oficiales: Go 1.27.1 y Node 24.21.0. Antes de un release posterior, revisa sus parches soportados y actualiza esos archivos explícitamente. Un build normal de release rechaza toolchains distintos. Esto no significa que esos toolchains se hayan ejecutado en el entorno que preparó el kit: revisa `AUDITORIA.md` de la entrega.

El módulo local pasa a llamarse `nearprod`; no necesita un owner GitHub para generar binarios. Esto no configura `go install github.com/OWNER/REPO/...`: esa vía no es el objetivo. No cambia ningún ID de base, red, proyecto, catálogo ni LaunchAgent.

## Validar localmente primero

Desde la raíz del repositorio, con las versiones indicadas instaladas:

```bash
npm ci --ignore-scripts
make check
make smoke test-package
make release-local
```

La salida se guarda en `dist/0.8.0/`. No crea tags, commits, repositorios, releases ni instalaciones globales. Si el directorio ya existe, el empaquetador se detiene para no mezclar artefactos. Puedes elegir otro directorio:

```bash
python3 scripts/release.py --output dist/revision-2
```

La fórmula requiere el repositorio real que va a almacenar las releases. El ZIP no contiene el remoto Git. Consulta `git remote -v`; no se debe inventar ni inferir a partir del nombre de la carpeta. Para generar también la fórmula localmente, pasa el valor real:

```text
python3 scripts/release.py --repository OWNER/REPO --output dist/revision-3
```

En GitHub, `GITHUB_REPOSITORY` ya identifica el repositorio y no hay que editar URLs. No se crea una fórmula real con un owner ficticio. `scripts/homebrew-formula.py` también puede generar la fórmula desde archives existentes usando `--repository`, `--artifacts` y `--output`.

Para diagnósticos sin toolchain actualizado o sin poder descargar npm:

```bash
make release-snapshot
```

Ese modo utiliza los assets ya entregados y produce versión `0.8.0-dev`. Se marca `snapshot=true`, no genera fórmula Homebrew y **no es publicable** por este workflow. No equivale a validar una release limpia ni a probar TypeScript.

## Primer paso remoto: CI sin publicación

Integra y revisa los cambios en tu rama real. GitHub CI correrá al subir la rama o abrir un PR. Esto debe hacerlo el titular, o un agente con autorización explícita; el kit no ejecuta pushes.

Una vez el workflow de Release esté disponible en la rama predeterminada, `Actions → NearProd Release → Run workflow` construye todos los artefactos SIN publicarlos. Esa ejecución manual no necesita crear un tag.

## Licencia y visibilidad antes de publicar

NearProd se distribuye bajo licencia MIT. El texto completo está en `LICENSE`; los avisos y licencias de terceros se conservan en `THIRD_PARTY_NOTICES.md` y dentro de cada archive.

También hay que decidir la visibilidad. Este pipeline publica en **el mismo repositorio**: si es privado, la release será privada y un tap público no hará accesibles sus binarios. No cambies la visibilidad del código solo para facilitar la descarga. Mantener código privado y publicar binarios en otro repositorio público es posible, pero necesita un destino y permisos diferentes y no está activado aquí.

Antes de hacerlo público, revisa el historial completo, confirma que no contiene datos o credenciales y habilita Secret scanning y Dependabot alerts cuando GitHub los ofrezca para el repositorio. Los escáneres complementan la revisión; no convierten fixtures o evidencia histórica en datos publicables por defecto.

El bloqueo inicial es `NEARPROD_PUBLISH_RELEASES`: si no existe o no vale `true`, se construye pero NO se publica. Es una protección operativa, no una comprobación automática de suficiencia legal.

## Publicar la primera versión, únicamente tras aprobación

En el repositorio correcto, tras confirmar la visibilidad y pasar CI:

1. Crea una variable de repositorio en **Settings → Secrets and variables → Actions → Variables**: `NEARPROD_PUBLISH_RELEASES=true`.
2. Confirma que el checkout está limpio, que los cambios revisados están guardados en Git y que `VERSION` tiene la versión elegida. El empaquetador rechaza una release normal desde un worktree sucio. No recrees ni muevas un tag existente.
3. Desde el commit aprobado, ejecuta explícitamente:

```bash
git tag -a v0.8.0 -m "NearProd 0.8.0"
git push origin v0.8.0
```

`v*` es un filtro glob. El script verifica además formato estable X.Y.Z, coincidencia con `VERSION` y commit del tag. No se simula una expresión regular dentro del filtro YAML.

El pipeline corre tests nativos en Linux y macOS; empaqueta darwin/arm64, darwin/amd64, linux/amd64 y linux/arm64 desde Linux; verifica SHA256; prueba HTTP desde el archive Linux; genera `nearprod.rb`; sube artifacts temporales. Otro job, sin checkout del código, verifica el manifiesto y publica una release primero como draft y luego visible. Solo ese job recibe `contents: write`; para descargar artifacts del mismo run usa `actions: read`.

Se emplea `GITHUB_TOKEN` para el propio repositorio. No hace falta un token personal para esta fase. Las acciones externas están fijadas a commits completos; Dependabot puede proponer actualizaciones de esas referencias.

Una release o draft preexistente no se sobreescribe. Si un upload falla, inspecciona el draft; decide explícitamente si eliminarlo y reintentar el job o publicar otra versión. No se borran releases automáticamente.

## Habilitar Homebrew después

Primero comprueba la descarga manual en tu Mac. Después crea un repositorio personal `homebrew-tap` con un commit inicial (por ejemplo un README). Para instalación pública debe ser legible públicamente y sus releases de origen también deben ser accesibles. No requiere pertenecer a una organización.

Puedes incorporar `nearprod.rb` de la release como `Formula/nearprod.rb` en ese tap. Para que se actualice automáticamente en versiones posteriores:

- Variable del repositorio NearProd: `NEARPROD_HOMEBREW_TAP=OWNER/homebrew-tap`, con tu owner real.
- Secret del repositorio NearProd: `NEARPROD_HOMEBREW_TOKEN`. Usa un token fine-grained restringido al repositorio tap y permiso **Contents: Read and write**. No necesita permisos de administración ni de Actions. Debe ser una credencial tuya autorizada; nunca pegarla en el código, prompt, logs ni fórmula. Un GitHub App token es otra opción para una fase posterior.

El `GITHUB_TOKEN` de NearProd no sustituye ese permiso de escritura sobre otro repositorio. Si la variable del tap está vacía, se omite ese paso y la release normal funciona sin ese secret. El workflow reusable `homebrew.yml` solo actualiza la fórmula, verifica su hash y rechaza actualizar desde una release que ya no sea la última. Si el tap tiene reglas que exigen PR, el push directo será rechazado: deja ese paso desactivado y adapta el flujo a PR antes de habilitarlo; no relajes protecciones automáticamente.

Si publicaste 0.7.0 antes de crear el tap, incorpora manualmente su fórmula para la instalación inicial. Desde la siguiente release aprobada, la actualización queda automatizada.

El usuario tendrá:

```text
brew install OWNER/tap/nearprod
nearprod ui
brew update
brew upgrade nearprod
```

La fórmula NO instala Docker/Colima, no modifica el shell, no arranca servicios y no escribe tus datos. NearProd usa una ruta Homebrew `opt` estable para `startup enable` en macOS; no fija la ruta de una versión del Cellar. También rechaza `nearprod install` al ejecutarse desde Homebrew para evitar instalaciones duplicadas.

## Actualizaciones y migración desde 0.7.0 manual

La función `nearprod()` que podía crear el instalador antiguo, un alias o el orden del PATH pueden seguir apuntando a `~/.local/bin/nearprod`. Revisa `type -a nearprod` y `.zshrc` antes de dar por completada la migración. El kit no los borra. Tampoco migra ni limpia el catálogo como parte del empaquetado.

`nearprod update --check` consulta la última release pública estable. `nearprod update` solo administra la instalación manual estable `~/.local/bin/nearprod`; Homebrew sigue siendo dueño de su ejecutable y debe usar `brew upgrade nearprod`. El actualizador valida digest de GitHub, `SHA256SUMS.txt`, archive e identidad de plataforma/versión, conserva una copia anterior y limpia temporales. Actualizar el archivo no reinicia un proceso ya activo. Consulta la guía del binario para detener/reabrir únicamente el agente cuando no tenga tareas activas. No se ejecuta `nearprod self-test` (Docker real) de forma automática.

## Límites de las validaciones

Compilar cuatro targets no es ejecutar los cuatro sistemas. Las pruebas simuladas de `launchctl` no equivalen a probar `launchd` real. No se certifica Gatekeeper, firma Apple ni notarización. El primer `brew install`/`brew upgrade`, el inicio de sesión macOS y la migración desde tu shell deben validarse en tu equipo. Conserva una copia del binario anterior y no borres datos para probar una actualización.

## Fuentes oficiales

Consultadas para preparar este flujo (las versiones deben revisarse de nuevo al publicar):

- https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax
- https://docs.github.com/en/actions/tutorials/authenticate-with-github_token
- https://cli.github.com/manual/gh_release_create
- https://cli.github.com/manual/gh_run_download
- https://docs.brew.sh/Formula-Cookbook
- https://docs.brew.sh/How-to-Create-and-Maintain-a-Tap
- https://go.dev/dl/?mode=json
- https://nodejs.org/en/download
