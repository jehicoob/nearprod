# CLI nativa

Ayuda extraída del binario de esta entrega. Todos los comandos de mutación comparten el coordinador del panel.

```text
NearProd 0.8 — controlador Go + panel React/TypeScript

Instalación y configuración (no requieren Docker):
  nearprod install [--configure-shell]
  nearprod update --check
  nearprod update                       solicita confirmación; --yes para automatización
  nearprod config paths
  nearprod config migrate --dry-run
  nearprod config migrate --yes
  nearprod config backup --yes          incluye credenciales, NO datos SQL
  nearprod startup enable|disable|status
  nearprod ui [--no-open]
  nearprod agent stop

Aplicaciones:
  nearprod init ~/Projects
  nearprod discover ~/Projects
  nearprod list
  nearprod status [grupo/aplicación]
  nearprod register --manifest definicion.json
  nearprod edit grupo/app --manifest definicion.json
  nearprod group-create --name "Mi grupo" [--id mi-grupo]
  nearprod review grupo/app [--mode dev|verify] [--approve --yes --allow-unsafe]
  nearprod adopt grupo/app [--yes]
  nearprod up|stop|restart|rebuild grupo[/app] [--mode dev|verify] [--wait]
               [--service api] [--confirm-mode] [--start-runtime] [--detach]
  nearprod logs grupo/app [--follow] [--service api] [--tail 100] [--since RFC3339]
  nearprod watch|watch-stop grupo/app
  nearprod remove grupo/app --yes        solo catálogo, conserva datos
  nearprod operation ID | nearprod cancel ID

Accesos locales / runtime:
  nearprod proxy status
  nearprod proxy start --port 80 [--yes]
  nearprod proxy stop --yes
  nearprod url grupo/app --host proyecto.localhost --service web --port 5173
  nearprod url-check grupo/app --host proyecto.localhost
  nearprod doctor
  nearprod runtime select --kind colima --context colima --profile default
  nearprod runtime status|start [--yes]
  nearprod runtime configure --memory 2 --cpus 2 [--yes]
  nearprod metrics

Infraestructura compartida:
  nearprod infra list
  nearprod infra ports --from 15432 --to 15442
  nearprod infra create --engine postgres|mysql|redis --id pg-main
    [--image postgres:18] [--memory 384] [--connections 30]
    [--persistence volume|folder|none] [--data-dir /ruta/dedicada] [--port 15432] [--yes]
  nearprod infra start --instance pg-main
  nearprod infra database --instance pg-main --name app_dev [--username app_user] --yes
  nearprod infra connection --database ID [--reveal]
  nearprod infra bind --target grupo/app --database ID --services api,worker
    [--mode dev] [--url-var DATABASE_URL] [--yes]
  nearprod infra check --database ID
  nearprod infra check-binding --binding ID
  nearprod infra unbind --binding ID --yes
  nearprod infra stop --instance pg-main [--yes --allow-active]
  nearprod infra backup --database ID [--directory /ruta/backups] --yes
  nearprod infra restore --database ID --file /ruta/archivo --trusted-backup --yes
  nearprod infra logs --instance pg-main [--follow]

Herramientas:
  nearprod tools list
  nearprod tools versions --tool docker|compose|buildx|colima
  nearprod tools install|upgrade|repair --tool compose [--formula docker-compose] [--yes]
  nearprod updates check

Aceptación real (crea/limpia SOLO recursos temporales de prueba):
  nearprod self-test --yes --context colima [--infra-only] [--folder]

--json emite JSON. NEARPROD_HOME selecciona un catálogo separado.
Actualizar el binario conserva ~/.nearprod/config y rutas/volúmenes existentes.
```

Una acción sin --yes muestra preview cuando corresponde. No se guardan contraseñas en argumentos. Los comandos responden JSON cuando se solicita --json; las operaciones esperan salvo --detach. --wait pertenece al comportamiento documentado de reconciliación, no certifica negocio.

`nearprod update --check` solo consulta la última release estable y el asset exacto de la plataforma. `nearprod update` funciona sobre la instalación manual estable en `~/.local/bin/nearprod`; solicita confirmación salvo `--yes`, verifica checksums e identidad, conserva la copia anterior y elimina temporales. No sobrescribe una instalación Homebrew ni un ejecutable abierto desde Descargas o un checkout. El agente ya activo conserva su versión hasta ejecutar, sin tareas en curso, `nearprod agent stop` y `nearprod ui`.
