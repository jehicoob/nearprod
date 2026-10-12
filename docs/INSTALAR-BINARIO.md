# Instalar NearProd sin clonar ni compilar

Esta distribución incluye un ejecutable Go con la interfaz React/TypeScript incorporada. No necesitas Go, Node, npm, pnpm ni Python para ejecutar NearProd. Los términos de uso son los del archivo `LICENSE`; esta guía no los modifica.

## Elegir la descarga

En la sección **Releases** del repositorio real del proyecto, descarga el `.tar.gz` de tu plataforma y `SHA256SUMS.txt` de ESA misma versión:

| Equipo | Archivo |
|---|---|
| Mac Apple Silicon (M1/M2/…) | `nearprod_X.Y.Z_darwin_arm64.tar.gz` |
| Mac Intel | `nearprod_X.Y.Z_darwin_amd64.tar.gz` |
| Linux x86-64 / WSL2 x86-64 | `nearprod_X.Y.Z_linux_amd64.tar.gz` |
| Linux ARM64 / WSL2 ARM64 | `nearprod_X.Y.Z_linux_arm64.tar.gz` |

No hay ejecutable Windows nativo en esta etapa. Descargar desde un repositorio privado exige acceso a ese repositorio. Los snapshots `-dev` son pruebas locales, no releases para usuarios.

Ejemplo macOS ARM64, sustituyendo X.Y.Z por la versión descargada:

```bash
VERSION='X.Y.Z'
ARCHIVE="nearprod_${VERSION}_darwin_arm64.tar.gz"
# Ejecutar en la carpeta que contiene el archive y SHA256SUMS.txt.
# pipefail hace fallar la operación si no existe la entrada esperada.
set -o pipefail
grep -F "  $ARCHIVE" SHA256SUMS.txt | shasum -a 256 -c -
```

El resultado debe indicar `OK`. No ejecutes el archivo si hay diferencias o falta el checksum. SHA256 comprueba integridad respecto al manifiesto; no sustituye una firma de identidad del editor.

Extrae el archive en una carpeta nueva y abre una terminal allí:

```bash
./nearprod --version
./nearprod install
export PATH="$HOME/.local/bin:$PATH"
nearprod ui
```

`install` crea una instalación manual en tu HOME; no toca las bases, no instala Docker ni habilita inicio automático. Para conservar el PATH, configura tu shell. `install --configure-shell` es una alternativa específica de zsh que ya existe en NearProd; hace backup de `.zshrc`, pero crea una función que debes revisar al migrar a Homebrew.

## Actualizar una instalación manual

```bash
nearprod update --check
nearprod update
```

La comprobación consulta la última release estable sin descargar el ejecutable. La actualización muestra el cambio de versión y pide confirmación; `--yes` queda disponible para automatización explícita. Solo reemplaza el ejecutable de la instalación manual `~/.local/bin/nearprod`: un binario ejecutado desde Descargas o desde un checkout debe instalarse primero.

A partir de `0.9.3`, el actualizador admite versiones de desarrollo identificadas, como `0.9.2-dev.5-ui.2`: puede pasar a `0.9.2` o a una estable posterior. Sigue la precedencia de [SemVer](https://semver.org/): una versión previa precede a su estable y los metadatos `+build` no cambian el orden. No realiza retrocesos; por ejemplo, `0.10.0-dev.1` no se reemplaza por `0.9.2`. Una compilación sin versión comparable, como `dev`, conserva `status: "development-build"` y no habilita la instalación automática.

Esta capacidad requiere un ejecutable que incluya la corrección del actualizador. Los binarios anteriores que bloquean versiones de desarrollo necesitan reemplazarse una vez; publicar una release o cambiar de rama no modifica el código del ejecutable ya instalado.

NearProd exige el asset exacto para macOS/Linux y ARM64/AMD64, compara el digest publicado por GitHub con `SHA256SUMS.txt`, rechaza rutas, enlaces y archivos especiales dentro del archive, ejecuta `--identity` antes y después de instalar, conserva la versión anterior bajo `~/.local/share/nearprod/releases` y elimina todos los temporales de la operación. En Linux/WSL2, la consulta HTTPS requiere el paquete de certificados CA del sistema. SHA-256 y TLS comprueban integridad respecto a GitHub, pero esta distribución sigue sin firma de código del editor.

Con el actualizador que incorpora reinicio automático, confirmar `nearprod update` basta: si el agente estaba activo y libre, instala y arranca la nueva versión en el mismo puerto. La interfaz compatible conserva su sesión y recarga sus recursos al reconectar. No detiene Docker ni los contenedores, ni modifica los datos de tus aplicaciones. Si el agente estaba detenido, permanece detenido y la próxima apertura utiliza el nuevo ejecutable.

Si hay operaciones o seguimientos Watch activos, la actualización se rechaza antes de reemplazar el binario; no los cancela. Cuando terminen, puedes repetir el mismo comando. El resultado indica `agentRestarted` y `agentStatus`. Si la instalación terminó pero falló el arranque (`UPDATE_AGENT_RESTART`, `activation-pending`), consulta `agent.log` y repite `nearprod update`: también reintenta la activación cuando ya tienes la última versión instalada. `--check` sigue siendo una consulta sin reinicios.

**Transición desde versiones anteriores:** el reinicio automático necesita estar incluido en el actualizador que estás ejecutando. `0.9.3` y anteriores no lo incorporan; para esa primera transición todavía debes cerrar y abrir el agente una vez con `nearprod agent stop` y `nearprod ui`, sin tareas activas. Un agente o una pestaña antiguos tampoco pueden transferir la sesión o recargar recursos con un protocolo que aún no conocen. Tras abrir la interfaz nueva, las siguientes actualizaciones compatibles no requieren esos pasos.

## Docker, Compose y Colima siguen siendo externos

El binario contiene NearProd, no Docker ni una VM. Para gestionar proyectos necesitas Docker CLI, Compose y un Engine Linux accesible. En macOS puedes seguir utilizando Colima. No necesitas Docker Desktop por el solo hecho de instalar este paquete. No ejecutes el controlador con `sudo`.

## Instalación Homebrew, cuando el tap esté publicado

Usa el comando que figure en el repositorio personal real:

```text
brew install OWNER/tap/nearprod
nearprod ui
```

`OWNER` es el titular real, no un nombre preconfigurado. La fórmula elige el binario adecuado. No vuelvas a ejecutar `nearprod install`: eso mezclaría dos instalaciones; NearProd lo rechaza al reconocer Homebrew.

Para actualizar una instalación Homebrew:

```bash
brew update
brew upgrade nearprod
```

Un agente que ya estaba ejecutándose sigue siendo el proceso anterior. En un momento sin tareas activas de NearProd:

```bash
nearprod agent stop
nearprod ui
```

La parada del agente puede cerrar seguimientos Watch; no detiene los contenedores de tus proyectos.

## Migrar desde la instalación manual

Primero ejecuta `type -a nearprod`. La instalación anterior puede haber añadido una función o alias a `.zshrc` y un ejecutable en `~/.local/bin`, ambos capaces de ocultar Homebrew. Revisa esas entradas y haz backup antes de editarlas. No borres `~/.nearprod`, vaults, catálogos, volúmenes ni carpetas de datos.

Comprueba la instalación nueva directamente con `"$(brew --prefix nearprod)/bin/nearprod" --version`. El inicio automático macOS usa la ruta estable `opt/nearprod/bin/nearprod`; se activa solo cuando tú ejecutas `nearprod startup enable`. No se configura `brew services` en paralelo.

## Seguridad de macOS

Estos artefactos no incluyen firma Developer ID ni notarización Apple. La ejecución de una descarga puede requerir revisión/aprobación del sistema. No desactives Gatekeeper globalmente. El build cruzado y un smoke en CI no sustituyen probar la descarga e instalación real en tu Mac.
