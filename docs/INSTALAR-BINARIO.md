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

## Docker, Compose y Colima siguen siendo externos

El binario contiene NearProd, no Docker ni una VM. Para gestionar proyectos necesitas Docker CLI, Compose y un Engine Linux accesible. En macOS puedes seguir utilizando Colima. No necesitas Docker Desktop por el solo hecho de instalar este paquete. No ejecutes el controlador con `sudo`.

## Instalación Homebrew, cuando el tap esté publicado

Usa el comando que figure en el repositorio personal real:

```text
brew install OWNER/tap/nearprod
nearprod ui
```

`OWNER` es el titular real, no un nombre preconfigurado. La fórmula elige el binario adecuado. No vuelvas a ejecutar `nearprod install`: eso mezclaría dos instalaciones; NearProd lo rechaza al reconocer Homebrew.

Para actualizar:

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
