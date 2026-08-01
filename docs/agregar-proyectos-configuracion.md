# Agregar proyectos a Local Infra editando archivos

Esta guía explica cómo registrar productos y componentes manualmente en `~/local-infra/projects.yml` y cómo preparar Docker Compose/Traefik para que el panel pueda iniciarlos, detenerlos y exponerlos por dominios `.localhost`.

---

## 1. Archivos involucrados

### Obligatorio

```txt
~/local-infra/projects.yml
```

Es el registro que lee el Control Panel.

### Normalmente necesario por cada proyecto

```txt
/ruta/al/proyecto/docker-compose.yml
/ruta/al/proyecto/docker-compose.override.yml
```

Allí se agregan labels de Traefik y la red `dev_proxy`.

### Infra compartida

```txt
~/local-infra/traefik/docker-compose.yml
```

Solo se toca si necesitas montar nuevas carpetas host dentro del contenedor `control-panel`.

---

## 2. Modelo de configuración

`projects.yml` tiene dos niveles:

```yaml
groups:
  producto:
    # metadata del producto

projects:
  producto_componente:
    # metadata y runner del componente
```

### `groups`

Define cómo se ve un producto en la UI y qué componentes agrupa.

Campos:

| Campo | Requerido | Descripción |
|---|---:|---|
| `label` | Recomendado | Nombre visible |
| `description` | No | Descripción corta |
| `components` | Sí | Lista de ids definidos en `projects:` |
| `quick_links` | No | Links destacados del producto |

### `projects`

Define cada componente.

Campos:

| Campo | Requerido | Descripción |
|---|---:|---|
| `label` | Recomendado | Nombre visible |
| `role` | Recomendado | `frontend`, `backend`, `worker`, `service`, etc. |
| `runner` | Sí | `docker-compose`, `manual`, `external` o `dockerfile` |
| `path` | Sí para Docker | Ruta local del proyecto |
| `project_name` | Sí para Docker | Nombre usado con `docker compose -p` |
| `type` | Recomendado | `react-vite`, `laravel-api`, `fastapi-api`, etc. |
| `urls` | No | URLs del componente |
| `source_type` | No | `local-folder`, `github`, `external-url`, etc. |
| `git_url` | No | URL repo si aplica |
| `trusted` | No | `true`/`false` si viene de tercero |

---

## 3. Checklist antes de agregar un proyecto

1. Confirmar ruta local del backend/frontend.
2. Confirmar si cada componente tiene Docker Compose.
3. Elegir dominios `.localhost`.
4. Elegir `project_name` único por componente.
5. Agregar labels de Traefik al compose del proyecto.
6. Conectar el servicio público a `dev_proxy`.
7. Registrar grupo/componentes en `projects.yml`.
8. Validar desde el panel.

---

## 4. Ejemplo completo: backend + frontend

Supongamos producto:

```txt
mi_producto
```

Dominios:

```txt
Frontend: http://mi-producto.localhost
API:      http://api.mi-producto.localhost
Docs:     http://api.mi-producto.localhost/docs
```

Rutas:

```txt
~/Trabajo/mi-producto/backend
~/Trabajo/mi-producto/frontend
```

### 4.1 Editar `projects.yml`

Agregar:

```yaml
groups:
  mi_producto:
    label: "Mi Producto"
    description: "Backend y frontend local de Mi Producto."
    components:
      - mi_producto_backend
      - mi_producto_frontend
    quick_links:
      frontend: "http://mi-producto.localhost"
      backend_docs: "http://api.mi-producto.localhost/docs"
      api: "http://api.mi-producto.localhost"

projects:
  mi_producto_backend:
    label: "Backend API"
    role: "backend"
    runner: "docker-compose"
    path: "~/Trabajo/mi-producto/backend"
    project_name: "mi_producto_backend"
    type: "api"
    urls:
      api: "http://api.mi-producto.localhost"
      docs: "http://api.mi-producto.localhost/docs"

  mi_producto_frontend:
    label: "Frontend App"
    role: "frontend"
    runner: "docker-compose"
    path: "~/Trabajo/mi-producto/frontend"
    project_name: "mi_producto_frontend"
    type: "react-vite"
    urls:
      frontend: "http://mi-producto.localhost"
      api: "http://api.mi-producto.localhost"
```

Si el archivo ya tiene `groups:` y `projects:`, no dupliques esas llaves raíz; agrega las entradas dentro de las secciones existentes.

---

## 5. Preparar Docker Compose para Traefik

### 5.1 Backend API

En el compose del backend:

```yaml
services:
  api:
    build:
      context: .
      dockerfile: Dockerfile
    expose:
      - "8000"
    labels:
      - traefik.enable=true
      - traefik.docker.network=dev_proxy
      - traefik.http.routers.mi-producto-api.rule=Host(`api.mi-producto.localhost`)
      - traefik.http.routers.mi-producto-api.entrypoints=web
      - traefik.http.services.mi-producto-api.loadbalancer.server.port=8000
    networks:
      - backend
      - dev_proxy

networks:
  backend:
    driver: bridge
  dev_proxy:
    external: true
```

### 5.2 Frontend Vite

En el compose del frontend:

```yaml
services:
  frontend:
    build:
      context: .
      dockerfile: Dockerfile
    working_dir: /app
    environment:
      - VITE_API_BASE_URL=http://api.mi-producto.localhost
      - VITE_DEV_HOST=mi-producto.localhost
      - VITE_DEV_CLIENT_PORT=80
      - VITE_DEV_PROTOCOL=ws
    volumes:
      - .:/app
      - frontend-node-modules:/app/node_modules
    expose:
      - "5173"
    labels:
      - traefik.enable=true
      - traefik.docker.network=dev_proxy
      - traefik.http.routers.mi-producto-frontend.rule=Host(`mi-producto.localhost`)
      - traefik.http.routers.mi-producto-frontend.entrypoints=web
      - traefik.http.services.mi-producto-frontend.loadbalancer.server.port=5173
    networks:
      - dev_proxy
    command: pnpm run dev -- --host 0.0.0.0 --port 5173

volumes:
  frontend-node-modules:

networks:
  dev_proxy:
    external: true
```

El `Dockerfile` del frontend debe instalar la versión de pnpm declarada en
`packageManager` y ejecutar `pnpm install --frozen-lockfile` durante el build;
no asumas que una imagen base de Node ya incluye el binario ni instales
dependencias durante el arranque del servicio.

---

## 6. Validar manualmente

Después de editar archivos:

```bash
~/local-infra/bin/dev-start
```

Abrir:

```txt
http://infra.localhost
```

Buscar el producto y probar:

1. `Iniciar` producto completo.
2. Abrir `Frontend`.
3. Abrir `Backend /docs`.
4. Abrir `Logs`.
5. Activar `Live` si necesitas seguimiento.

Validaciones CLI:

```bash
docker context show
docker network inspect dev_proxy >/dev/null
cd ~/Trabajo/mi-producto/backend
docker compose -p mi_producto_backend ps
cd ~/Trabajo/mi-producto/frontend
docker compose -p mi_producto_frontend ps
```

---

## 7. Proyectos sin Docker

Si el proyecto aún no tiene Docker:

```yaml
projects:
  mi_producto_frontend:
    label: "Frontend App"
    role: "frontend"
    runner: "manual"
    path: "~/Trabajo/mi-producto/frontend"
    project_name: "mi_producto_frontend"
    type: "react-vite"
    urls:
      frontend: "http://localhost:5173"
```

El panel lo mostrará, pero no podrá iniciar/detener automáticamente.

---

## 8. Servicios externos

Para registrar una URL que no corre localmente:

```yaml
projects:
  mi_producto_docs_externas:
    label: "Docs externas"
    role: "external"
    runner: "external"
    path: ""
    project_name: "mi_producto_docs_externas"
    type: "external-url"
    urls:
      app: "https://example.com/docs"
```

---

## 9. Rutas fuera de `~/Trabajo`

El contenedor `control-panel` debe poder ver las rutas que registras.

Actualmente monta normalmente:

```txt
~/local-infra
~/Trabajo
```

Si agregas proyectos fuera de esas rutas, edita:

```txt
~/local-infra/traefik/docker-compose.yml
```

En el servicio `control-panel`, agrega:

```yaml
volumes:
  - /ruta/host:/ruta/host
```

Luego:

```bash
~/local-infra/bin/dev-start
```

---

## 10. Errores comunes

### El panel muestra `Sin crear`

Causas comunes:

- `project_name` no coincide con el usado por Docker Compose.
- Nunca se ejecutó `up` para ese project name.
- La ruta `path` no existe dentro del contenedor `control-panel`.

### Dominio no abre

Causas comunes:

- Falta `dev_proxy` en el servicio.
- Falta `traefik.enable=true`.
- El puerto de `loadbalancer.server.port` no coincide con el puerto interno.
- El router tiene un Host incorrecto.
- El servicio usa `ports` pero no `expose`/labels correctos.

### Logs vacíos

Causas comunes:

- El componente no es `docker-compose`.
- No hay contenedores creados.
- El `project_name` no coincide.

---

## 11. Convenciones finales

- Un producto = un `group`.
- Un backend/frontend/worker = un `project`.
- Usa `project_name` único.
- Usa `.localhost`.
- Usa `expose`, no `ports`, cuando Traefik puede enrutar.
- Todo servicio público debe estar en `dev_proxy`.
- Registra links rápidos en `quick_links`.
- Revisa proyectos de terceros antes de ponerlos como ejecutables.
