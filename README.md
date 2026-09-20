# NearProd 0.8.0 — Go + React/TypeScript

Controlador local de aplicaciones Docker Compose, URLs `proyecto.localhost` por Traefik e infraestructura PostgreSQL/MySQL/Redis. El CLI y el agente están implementados en Go; el panel React/TypeScript compilado se incluye dentro del binario. **No inicia Node ni necesita npm para funcionar.**

**Estado de entrega: candidata a aceptación, no certificada en macOS/Colima.** Las pruebas ejecutadas y los bloqueos están en [PRUEBAS](docs/PRUEBAS.md). No se reutilizan los 216 tests Node como si fueran pruebas de esta reescritura.

> **Toolchains de distribución:** las releases se construyen con las versiones exactas de `.go-version` y `.node-version`; el pipeline rechaza otras versiones y un checkout con cambios sin guardar. Los binarios históricos documentados en la auditoría no son publicables. Ver [COMPILAR](docs/COMPILAR.md). No se incluye firma Developer ID ni notarización Apple.

## Actualizar desde 0.6.1 sin registrar otra vez tus proyectos

No borres `~/.nearprod`, tus volúmenes ni carpetas de bases. Usa el mismo `NEARPROD_HOME` que antes; cambiarlo abre deliberadamente otro catálogo.

En el Mac M1, desde el archive descomprimido, confirma primero el binario que vas a instalar:

```bash
./nearprod --version
```

La UI ya está incluida; no hace falta Go ni Node para este paso. Después:

```bash
# 1. Cerrar el agente anterior, NO Colima ni tus contenedores.
nearprod agent stop

# 2. Examinar la migración. Esto no escribe ni ejecuta Docker.
./nearprod config migrate --dry-run

# 3. Instalar el ejecutable estable. No toca el catálogo.
./nearprod install --configure-shell
```

Abre otra terminal. Si el comando sigue apuntando a una instalación vieja, usa directamente `~/.local/bin/nearprod` en las siguientes instrucciones.

```bash
nearprod --version                  # 0.8.0
nearprod config migrate --yes       # backup + migración; exige agente anterior cerrado
nearprod ui
```

Si `nearprod` no era reconocido antes de instalar, ejecuta `./nearprod agent stop`. `AGENT_OFFLINE` significa que ya está cerrado. **No ignores `AGENT_RUNNING`, errores de permisos o migración conflictiva.**

La primera apertura también puede realizar la migración. La secuencia explícita anterior permite ver primero el cambio. Las aplicaciones, grupos, nombres Compose, dominios, IDs de volúmenes, credenciales y vínculos se conservan. Hace falta una **nueva revisión/aprobación de la configuración** antes de iniciar con el nuevo controlador; eso no es volver a registrar.

## Instalación nueva

Con el binario recompilado según el apartado anterior:

```bash
./bin/nearprod-darwin-arm64 install --configure-shell
# Terminal nueva:
nearprod ui
```

Mac Intel: utiliza `bin/nearprod-darwin-amd64`. Linux x86_64: `bin/nearprod-linux-amd64`; el autoarranque administrado es exclusivo de macOS. Los binarios Mac están compilados, pero no fueron ejecutados en un Mac en esta revisión. Si Gatekeeper impide abrir el binario, verifica origen y checksum y utiliza la autorización individual de macOS o compila localmente; no desactives Gatekeeper globalmente.

El instalador guarda `~/.local/bin/nearprod` y copias de versiones bajo `~/.local/share/nearprod/releases`. Hace backup de `.zshrc` al añadir su función/ruta; no sustituye aliases ajenos, Node ni gestores de paquetes. No utiliza `sudo`.

Para una instalación manual estable:

```bash
nearprod update --check
nearprod update
```

El segundo comando muestra la versión y solicita confirmación. Descarga desde la release pública oficial, verifica el digest de GitHub, `SHA256SUMS.txt`, el contenido del archive y la identidad del nuevo ejecutable antes y después de instalarlo. Conserva una copia anterior y limpia los temporales; no reinicia el agente que ya estaba en ejecución ni toca contenedores o datos. Las instalaciones Homebrew deben usar `brew upgrade nearprod`.

## Dónde permanece todo

```text
~/.nearprod/
├── config/
│   ├── catalog.json                # grupos, proyectos, URLs, preferencias, referencias
│   ├── migration.json              # journal de la migración desde schema 3
│   └── resources/<uid>/vault.json   # credenciales de NUEVAS instancias, archivo 0600
├── databases/                     # ubicación sugerida, SOLO para nuevas carpetas elegidas
│   ├── postgres/pg-main/data/
│   └── mysql/mysql-main/data/
├── backups/
│   ├── config/                    # metadata/credenciales, no archivos físicos de DB
│   └── databases/                 # exportaciones SQL manuales
├── infra/<uid>/                   # se conserva para instancias creadas en 0.6.x
├── proxy/                         # rutas/configuración del Traefik existente
├── catalog.json                   # marcador que impide que 0.6 escriba el catálogo migrado
└── agent.sock, agent.json, ...     # coordinación local, no documentos de proyecto
```

La carpeta de datos representa **la instancia del motor**, no una carpeta por base lógica SQL. Las instancias antiguas conservan sus rutas o volúmenes originales: la migración no mueve archivos de motores activos ni copia volúmenes Docker a macOS. Los volúmenes Docker siguen en la VM. No se modifica la persistencia de Compose importados.

```bash
nearprod config paths
nearprod config backup --yes
```

Ese backup es privado y contiene credenciales; no lo publiques. Tampoco sustituye un backup consistente de MySQL/PostgreSQL.

## Recorrido diario

1. **Herramientas:** detectar/conservar Docker CLI, Compose, Buildx, Colima. Homebrew macOS permite instalación/reparación confirmada. No se modifica Node.
2. **Runtime:** elegir el contexto local y el perfil Colima existente. El panel puede abrir con Engine detenido.
3. **Accesos locales:** activar un solo Traefik. Puerto 80 permite `http://tienda.localhost`; 8080 añade `:8080`. Los puertos internos web se configuran aparte.
4. **Descubrir aplicaciones:** seleccionar raíz, candidatos y grupo; elegir Compose/env/perfiles y URLs. Guardar no ejecuta nada.
5. **Revisar y aprobar → Iniciar:** estado, logs, URL y salud. El servidor HTTP debe escuchar en una interfaz alcanzable. NearProd no cambia CORS/HMR/APP_URL.
6. **Infraestructura:** crear instancia/base/usuario, vincular API o worker, revisar y volver a iniciar para aplicar. Las DB propias del proyecto se conservan.

Para Laravel dirige la URL a Caddy/Nginx HTTP, no a PHP-FPM. Para Elixir/Phoenix, el proyecto necesita un Compose y su servidor/release/puerto/origen correctos. Los ejemplos del ZIP cubren esos recorridos de prueba; no convierten una configuración arbitraria en válida.

## Inicio al entrar en macOS

```bash
nearprod startup enable
nearprod startup status
nearprod startup disable
```

Solo inicia el agente, no el navegador, Colima, Traefik, bases o proyectos. Tras actualizar desde Node, ejecuta nuevamente `startup enable` para actualizar el plist a la ruta del binario nativo. No es arranque antes de iniciar sesión; no usa KeepAlive para resucitar un agente detenido intencionalmente. Ver [STARTUP](docs/STARTUP.md).

## Validación con tu Docker real

```bash
# macOS / Colima
nearprod self-test --yes --context colima
nearprod self-test --yes --context colima --infra-only --folder

# Linux / WSL2 con Docker nativo
nearprod self-test --yes --context default
```

Pruebas opt-in: crean imágenes/contenedores/redes/volúmenes y bases TEMPORALES, comprueban rutas, SQL, separación, persistencia y recuperación; limpian solo sus recursos identificados. No cambian tu catálogo, datos, DNS o Colima; imágenes/caché permanecen. Revisa el informe `nearprod-acceptance-*/result.json`. Código 77 es bloqueo, no aprobado. La modalidad completa ejecuta motores secuencialmente, pero no promete caber junto a todas tus aplicaciones en 2 GiB.

## Documentación

[Inicio rápido](docs/INICIO-RAPIDO.md) · [Migración y almacenamiento](docs/MIGRACION.md) · [Infraestructura](docs/INFRAESTRUCTURA.md) · [Ciclo de vida](docs/CICLO-DE-VIDA.md) · [CLI](docs/CLI.md) · [Arquitectura/spec](docs/SPEC-0.7.0.md) · [Auditoría](docs/AUDITORIA-0.7.0.md) · [Pruebas](docs/PRUEBAS.md) · [Compilar](docs/COMPILAR.md)

No incluye TLS/DNS administrado, despliegue remoto, adopción automática de proxies externos, actualización mayor in situ de bases, borrado de datos, migración automática de DBs de proyectos ni soporte universal Compose `include`/`extends`. No garantiza ahorro de RAM de los contenedores: Go sustituye el controlador, no la VM Linux.

## Distribución desde GitHub y Homebrew

La preparación de releases, versionado, CI y tap personal se documenta en [docs/DISTRIBUCION.md](docs/DISTRIBUCION.md). Para descargar y ejecutar sin compilar, consulta [docs/INSTALAR-BINARIO.md](docs/INSTALAR-BINARIO.md). NearProd se distribuye bajo licencia MIT. La publicación está deshabilitada por defecto; confirma la visibilidad antes de habilitarla.
