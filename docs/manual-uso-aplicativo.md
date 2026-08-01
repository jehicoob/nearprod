# Manual de uso — Local Infra Control Panel
Este documento explica cómo usar el aplicativo local de administración de proyectos Docker/Colima publicado en:

```txt
http://infra.localhost
```

El objetivo del panel es permitir trabajar con varios proyectos locales al mismo tiempo sin pelear con puertos, usando una infraestructura compartida basada en:

- Colima como runtime Docker local.
- Docker CLI y Docker Compose para ejecutar los proyectos.
- Traefik como reverse proxy local.
- Dominios `.localhost` por proyecto.
- `~/local-infra/projects.yml` como registro central de productos/componentes.

---

## 1. Conceptos principales

### 1.1 Producto o grupo

Un **producto** representa una solución completa. Por ejemplo:

- `maximo_puntaje`
- `pos_warehouses`

Un producto puede agrupar varios componentes:

```txt
Producto
├── Backend API
├── Frontend App
├── Worker
├── Scheduler
└── Servicio externo/manual
```

En `projects.yml`, los productos viven en la sección `groups:`.

### 1.2 Componente

Un **componente** es una unidad ejecutable o registrable dentro de un producto.

Ejemplos:

- backend Laravel
- frontend React/Vite
- API FastAPI
- worker Celery
- servicio manual
- URL externa

En `projects.yml`, los componentes viven en la sección `projects:`.

### 1.3 Runner

El campo `runner` define si el panel puede ejecutar acciones automáticamente.

Valores actuales recomendados:

| Runner | Significado | Acciones desde UI |
|---|---|---|
| `docker-compose` | Proyecto con `docker-compose.yml`, `compose.yml` o equivalente | Sí |
| `manual` | Proyecto registrado, pero sin runner automatizado | No |
| `external` | URL externa o servicio fuera de Docker local | No |
| `dockerfile` | Detectado como Dockerfile suelto, todavía sin compose | No, hasta crear compose |

---

## 2. Arranque del aplicativo

Desde terminal:

```bash
~/local-infra/bin/dev-start
```

Este script hace lo siguiente:

1. Inicia Colima si no está corriendo.
2. Cambia el contexto Docker a `colima`.
3. Crea la red compartida `dev_proxy` si no existe.
4. Levanta Traefik.
5. Construye/levanta el Control Panel detrás de Traefik.

URLs principales:

```txt
Control panel:     http://infra.localhost
Traefik dashboard: http://localhost:8080/dashboard/
```

Para detener solo la UI:

```bash
~/local-infra/bin/ui-stop
```

Para iniciar solo la UI:

```bash
~/local-infra/bin/ui-start
```

---

## 3. Pantalla principal

La pantalla principal muestra:

- Topbar con búsqueda, estado Docker y acciones globales.
- Layout principal sin sidebar para aprovechar más espacio horizontal.
- KPIs generales.
- Tarjetas por producto.
- Panel de actividad de comandos.

### 3.1 KPIs

Los KPIs muestran:

- Productos activos.
- Contenedores activos/totales.
- Warnings.
- Componentes registrados.

### 3.2 Buscador global

El buscador superior filtra por:

- nombre de producto
- slug
- descripción
- rol
- tipo
- ruta local
- URLs

Ejemplo de búsquedas útiles:

```txt
pos
backend
frontend
fastapi
maximo
api.pos
```

---

## 4. Tarjetas de producto

Cada producto muestra:

- Nombre.
- Descripción.
- Estado general.
- Accesos rápidos.
- Acciones del producto completo.
- Componentes internos.

### 4.1 Estados

| Estado | Significado |
|---|---|
| `Activo` | Todos los componentes Docker Compose del producto están corriendo |
| `Parcial` | Algunos componentes están corriendo y otros no |
| `Detenido` | Hay contenedores creados, pero no corren |
| `Sin crear` | No existen contenedores todavía |
| `Manual` | No hay componentes ejecutables automáticamente |
| `Externo` | Servicio externo |
| `Desconocido` | Docker no pudo consultar correctamente |

### 4.2 Accesos rápidos

Los accesos rápidos salen de `groups.<producto>.quick_links` o se derivan desde las URLs de los componentes.

Ejemplos:

- `Frontend` → `http://mi-proyecto.localhost`
- `Backend /docs` → `http://api.mi-proyecto.localhost/docs`
- `API` → `http://api.mi-proyecto.localhost`

---

## 5. Acciones disponibles

### 5.1 Acciones por producto

Las acciones de producto aplican a todos sus componentes `docker-compose`.

| Acción UI | Comando base |
|---|---|
| `Iniciar` | `docker compose -p <project_name> up -d --build --remove-orphans` |
| `Detener` | `docker compose -p <project_name> stop` |
| `Reiniciar` | `docker compose -p <project_name> restart` |
| `Bajar` | `docker compose -p <project_name> down --remove-orphans` |

Para `Detener` y `Bajar`, el panel ejecuta los componentes del grupo en orden inverso.

### 5.2 Acciones por componente

Cada componente Docker Compose tiene sus propios botones:

- `Iniciar`
- `Detener`
- `Reiniciar`
- `Bajar`
- `Logs`

Si el componente es `manual`, `external` o `dockerfile`, las acciones Docker quedan deshabilitadas.

---

## 6. Panel de actividad

El panel inferior muestra el progreso de comandos como:

- iniciar producto
- detener componente
- reiniciar servicio
- bajar contenedores

Los logs ya no se muestran en este panel. Los logs tienen su propio modal.

---

## 7. Modal de logs

Al hacer click en `Logs` en un componente, se abre un modal dedicado.

Incluye:

- buscador interno
- resaltado de coincidencias
- contador `actual/total`
- navegación a coincidencia anterior/siguiente
- filtro `Solo coincidencias`
- copiar logs
- recargar manualmente
- modo `Live`
- hora de última lectura

### 7.1 Buscar dentro de logs

Escribe en el buscador del modal. El contador muestra algo como:

```txt
1/8 coincidencias
```

Usa los botones con flechas para moverte entre coincidencias.

### 7.2 Solo coincidencias

Activa `Solo coincidencias` para ocultar líneas que no contienen el texto buscado.

### 7.3 Modo Live

El toggle `Live` activa actualización automática cada ~2 segundos.

Importante:

- Es near real-time por polling.
- No es streaming puro con WebSocket/SSE.
- Solo corre mientras el modal está abierto y `Live` está activo.
- Si estás buscando texto, el modal no fuerza scroll al final para no interrumpirte.

---

## 8. Agregar proyectos desde UI

El botón `Agregar` abre un wizard.

El wizard permite registrar:

- carpeta local
- proyecto clonado de GitHub
- Docker Compose existente
- proyecto sin Docker
- URL externa

Flujo recomendado:

1. Elegir tipo de fuente.
2. Completar datos del producto.
3. Agregar uno o varios componentes.
4. Usar detección de stack si aplica.
5. Revisar configuración.
6. Guardar.

Cuando guarda, el backend actualiza `~/local-infra/projects.yml` y crea backup automático:

```txt
~/local-infra/projects.yml.bak-YYYYMMDDTHHMMSSZ
```

---

## 9. Agregar proyectos editando archivos

El archivo central es:

```txt
~/local-infra/projects.yml
```

Tiene dos secciones:

```yaml
groups:
  producto_slug:
    label: "Nombre visible"
    description: "Descripción corta"
    components:
      - producto_backend
      - producto_frontend
    quick_links:
      frontend: "http://producto.localhost"
      backend_docs: "http://api.producto.localhost/docs"
      api: "http://api.producto.localhost"

projects:
  producto_backend:
    label: "Backend API"
    role: "backend"
    runner: "docker-compose"
    path: "~/Trabajo/ruta/backend"
    project_name: "producto_backend"
    type: "fastapi-api"
    urls:
      api: "http://api.producto.localhost"
      docs: "http://api.producto.localhost/docs"

  producto_frontend:
    label: "Frontend App"
    role: "frontend"
    runner: "docker-compose"
    path: "~/Trabajo/ruta/frontend"
    project_name: "producto_frontend"
    type: "react-vite"
    urls:
      frontend: "http://producto.localhost"
      api: "http://api.producto.localhost"
```

Más detalles: `docs/agregar-proyectos-configuracion.md`.

---

## 10. Requisitos para que Traefik enrute un proyecto

Cada servicio que quieras exponer por dominio debe:

1. Estar conectado a la red externa `dev_proxy`.
2. Usar `expose`, no necesariamente `ports`.
3. Tener labels de Traefik.
4. Declarar `dev_proxy` como external network.

Ejemplo frontend Vite:

```yaml
services:
  frontend:
    build:
      context: .
      dockerfile: Dockerfile
    working_dir: /app
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

Ejemplo API:

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

---

## 11. Convenciones recomendadas

### 11.1 Slugs

Usa minúsculas, números, guiones o underscores.

Buenos ejemplos:

```txt
mi_producto
mi-producto
pos_warehouses
maximo_puntaje
```

Evita:

```txt
Mi Producto
mi.producto
mi/producto
```

### 11.2 Dominios

Usa `.localhost` para evitar tocar `/etc/hosts`.

Convención recomendada:

```txt
Frontend: http://producto.localhost
API:      http://api.producto.localhost
Docs:     http://api.producto.localhost/docs
```

### 11.3 Project names Docker Compose

Cada componente debe tener un `project_name` único para evitar colisiones:

```yaml
project_name: "mi_producto_backend"
```

El panel consulta contenedores por label:

```txt
com.docker.compose.project=<project_name>
```

---

## 12. Troubleshooting

### 12.1 `infra.localhost` no abre

Verifica:

```bash
docker context show
docker ps
cd ~/local-infra/traefik
docker compose -p local-infra ps
```

Reinicia base:

```bash
~/local-infra/bin/dev-start
```

### 12.2 Un dominio de proyecto no abre

Verifica que el servicio tenga:

- `traefik.enable=true`
- `traefik.docker.network=dev_proxy`
- router con `Host(...)`
- service port correcto
- red `dev_proxy` conectada
- `expose` del puerto interno

Consulta Traefik:

```txt
http://localhost:8080/dashboard/
```

### 12.3 El panel dice `Sin crear`

Significa que no encontró contenedores con el label Docker Compose correspondiente.

Soluciones:

1. Revisar `project_name` en `projects.yml`.
2. Ejecutar `Iniciar` desde UI.
3. O ejecutar manualmente:

```bash
cd /ruta/al/proyecto
docker compose -p nombre_project_name up -d --build --remove-orphans
```

### 12.4 Logs no aparecen

Verifica:

- El componente debe tener `runner: "docker-compose"`.
- El `project_name` debe coincidir con el proyecto Compose real.
- Deben existir contenedores creados para ese project name.

Comando manual equivalente:

```bash
cd /ruta/al/proyecto
docker compose -p nombre_project_name logs --tail 500
```

### 12.5 El proyecto está fuera de `~/Trabajo`

El contenedor del panel actualmente monta:

```txt
~/local-infra
~/Trabajo
```

Si quieres gestionar rutas fuera de esas carpetas, debes agregar el mount en:

```txt
~/local-infra/traefik/docker-compose.yml
```

En el servicio `control-panel`:

```yaml
volumes:
  - /ruta/host:/ruta/host
```

Luego reconstruir:

```bash
~/local-infra/bin/dev-start
```

---

## 13. Desarrollo del panel

Código UI/API:

```txt
~/local-infra/ui
```

Comandos:

```bash
cd ~/local-infra/ui
pnpm run test
pnpm run build
```

Estructura relevante:

```txt
ui/
├── server.mjs               # API local y static server
├── Dockerfile               # imagen del control panel
├── src/
│   ├── api/client.ts
│   ├── components/
│   │   ├── AddProjectWizard.tsx
│   │   ├── LogsModal.tsx
│   │   ├── ProductCard.tsx
│   │   └── ...
│   ├── hooks/useDashboard.ts
│   ├── lib/
│   └── styles.css
└── vitest.config.ts
```

---

## 14. Seguridad y buenas prácticas

- No ejecutes proyectos de terceros sin revisar `Dockerfile`, `docker-compose.yml`, scripts de instalación y `.env.example`.
- Prefiere registrar primero como `manual` si el proyecto no es confiable.
- Evita publicar puertos con `ports` salvo que sea estrictamente necesario; usa Traefik + `expose`.
- Mantén `project_name` único por componente.
- Haz backup antes de editar manualmente `projects.yml`.
- No uses `down -v` desde el panel; actualmente `Bajar` no borra volúmenes.

---

## 15. Checklist rápido de uso diario

```bash
~/local-infra/bin/dev-start
```

Abrir:

```txt
http://infra.localhost
```

Para trabajar:

1. Filtra producto si tienes muchos.
2. Click en `Iniciar` del producto.
3. Abre `Frontend` o `Backend /docs`.
4. Usa `Logs` para debug.
5. Activa `Live` si necesitas seguimiento continuo.
6. Al terminar, usa `Detener` o `Bajar` según necesites.
