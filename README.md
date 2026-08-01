# NearProd

Despliegues locales similares a producción con Colima, Docker Compose, Traefik y dominios `.localhost`.

## Configuración inicial

```bash
cp .env.example .env
cp projects.example.yml projects.yml
```

Ajusta en `.env` las rutas de tu usuario y la carpeta donde guardas tus proyectos.

## URLs

```txt
Control panel:     http://infra.localhost
Traefik dashboard: http://localhost:8080/dashboard/
```

## Arranque base

```bash
./bin/dev-start
```

Este comando:

1. Inicia Colima si no está corriendo.
2. Cambia el contexto Docker a `colima`.
3. Crea la red compartida `dev_proxy` si falta.
4. Levanta Traefik.
5. Levanta el panel local de control como contenedor detrás de Traefik.

## Control panel

El panel vive en `ui/`, corre como servicio `control-panel` dentro del compose de `traefik/`, está implementado en React/Vite y lee `projects.yml`.

Acciones disponibles por producto y por componente:

- `Iniciar`: `docker compose -p <project_name> up -d --build --remove-orphans`
- `Stop`: `docker compose -p <project_name> stop`
- `Down`: `docker compose -p <project_name> down --remove-orphans`
- `Restart`: `docker compose -p <project_name> restart`
- `Logs`: `docker compose -p <project_name> logs --tail 180`
- `Add project`: wizard para registrar producto, componentes, rutas, URLs y runner

Comandos directos:

```bash
~/local-infra/bin/ui-start
~/local-infra/bin/ui-stop
```

## Proyectos registrados

Editar:

```bash
~/local-infra/projects.yml
```

Formato base:

```yaml
projects:
  nombre_proyecto:
    path: "~/Trabajo/ruta/al/proyecto"
    project_name: "nombre_compose"
    type: "react-vite"
    urls:
      app: "http://mi-proyecto.localhost"
      api: "http://api.mi-proyecto.localhost"
```

## Traefik

Traefik usa Docker provider para proyectos y para el control panel.

```bash
cd ~/local-infra/traefik
docker compose -p local-infra up -d --build
```

Más detalles:

- `docs/manual-uso-aplicativo.md`
- `docs/agregar-proyectos-configuracion.md`
- `docs/control-panel-ui.md`
- `docs/colima-dominios-locales.md`

## Desarrollo UI

```bash
cd ~/local-infra/ui
pnpm run test
pnpm run build
```

La UI está componentizada en `ui/src/components`, la lógica de red vive en `ui/src/api`, y los hooks en `ui/src/hooks`.
