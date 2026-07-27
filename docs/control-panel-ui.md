# Local Infra Control Panel

Panel local para manejar productos/proyectos declarados en `~/local-infra/projects.yml`.

## Estado actual

La UI fue rediseñada con Stitch como referencia visual y luego implementada en React/Vite con componentes y tests.

URL:

```txt
http://infra.localhost
```

## Modelo mental

La UI ya no trabaja solo con una lista plana de Compose projects. Ahora separa:

```txt
Product / Group
├── Component: backend
├── Component: frontend
├── Component: worker
├── Component: external URL
└── Component: manual / pending Docker
```

Ejemplo:

```yaml
groups:
  pos_warehouses:
    label: "POS Warehouses"
    components:
      - pos_warehouses_backend
      - pos_warehouses_frontend
    quick_links:
      frontend: "http://pos-warehouses.localhost"
      backend_docs: "http://api.pos-warehouses.localhost/docs"
      api: "http://api.pos-warehouses.localhost"
```

## Features incluidas

- Dashboard agrupado por producto.
- Accesos rápidos por producto:
  - Frontend
  - Backend `/docs`
  - API
- Acciones por producto completo:
  - Start
  - Stop
  - Down
  - Restart
- Acciones por componente individual.
- Logs por componente Docker Compose.
- Estado por contenedor usando labels de Docker Compose.
- Wizard para agregar un nuevo producto.
- Detección básica de stack desde ruta local:
  - `docker-compose.yml`
  - `Dockerfile`
  - `package.json`
  - `vite.config.*`
  - `next.config.*`
  - `artisan`/`composer.json`
  - `pyproject.toml`/`requirements.txt`
- Registro de proyectos sin Docker como `manual`.
- Registro de URLs externas como `external`.
- Seguridad para repos de terceros: se registran primero y se revisan antes de ejecutarlos.

## Implementación

```txt
~/local-infra/ui/
├── server.mjs
├── Dockerfile
├── package.json
├── src/
│   ├── api/client.ts
│   ├── components/
│   ├── hooks/useDashboard.ts
│   ├── lib/
│   ├── test/setup.ts
│   ├── App.tsx
│   ├── main.tsx
│   └── styles.css
└── vitest.config.ts
```

## API local

- `GET /api/projects` — dashboard completo con grupos, proyectos, Docker status.
- `POST /api/groups/:id/actions/:action` — ejecuta acción por producto.
- `POST /api/projects/:id/actions/:action` — ejecuta acción por componente.
- `GET /api/projects/:id/logs?tail=180` — logs del componente.
- `POST /api/detect` — detecta stack desde path local.
- `POST /api/products` — agrega producto/componentes a `projects.yml` con backup automático.

## Comandos de desarrollo

```bash
cd ~/local-infra/ui
npm run test
npm run build
```

## Tests actuales

- `src/lib/__tests__/dashboard.test.ts`
- `src/lib/__tests__/wizard.test.ts`
- `src/components/__tests__/ProductCard.test.tsx`
- `src/components/__tests__/AddProjectWizard.test.tsx`

## Arranque

```bash
~/local-infra/bin/dev-start
```

El compose de `traefik/` construye la UI y la publica vía Traefik en `infra.localhost`.

## Roadmap recomendado

1. Health checks HTTP reales para cada quick link.
2. Métricas CPU/RAM por producto usando `docker stats --no-stream`.
3. Wizard con clonación Git real, pero siempre en modo registro/revisión antes de ejecutar.
4. Editor visual de `projects.yml` con diff antes de guardar.
5. Confirmación fuerte para `Down` y limpieza de volúmenes.
6. Perfiles Compose por componente, por ejemplo `workers`.
7. Historial persistente de jobs.
