# Implementación local con Colima + Traefik + dominios personalizados

Actualizado: 2026-06-09
Entorno objetivo: macOS Apple Silicon, MacBook Air M1, sin OrbStack, sin VPS, mínimo 2 proyectos Docker al mismo tiempo.

---

## 1. Objetivo

Montar un entorno local gratuito para ejecutar varios proyectos Docker simultáneamente sin conflictos de puertos.

La solución propuesta usa:

- **Colima** como motor Docker local.
- **Docker CLI + Docker Compose** como interfaz estándar para ejecutar los `docker-compose.yml` de cada proyecto sobre el motor Docker de Colima.
- **Traefik** como reverse proxy local único.
- **Dominios locales personalizados**, por ejemplo:
  - `pos.localhost`
  - `api.pos.localhost`
  - `barber.localhost`
  - `api.barber.localhost`
  - opcionalmente `pos.test`, `api.pos.test`, etc.

La regla central será:

> Solo Traefik publica puertos en tu Mac. Los proyectos no deben publicar `3000`, `5173`, `8000`, `5432`, etc. al host salvo excepciones puntuales.

---

## 2. Arquitectura final

```txt
MacBook Air M1
│
├── Colima VM
│   │
│   ├── Docker Engine
│   │   │
│   │   ├── dev_proxy network
│   │   │   │
│   │   │   └── Traefik
│   │   │       ├── 127.0.0.1:80
│   │   │       ├── 127.0.0.1:443 opcional
│   │   │       └── 127.0.0.1:8080 dashboard
│   │   │
│   │   ├── Proyecto POS
│   │   │   ├── frontend:5173 interno
│   │   │   ├── api:8000 interno
│   │   │   └── db:5432 interno
│   │   │
│   │   └── Proyecto Barber
│   │       ├── frontend:5173 interno
│   │       ├── api:8000 interno
│   │       └── db:5432 interno
│
└── Navegador macOS
    ├── http://pos.localhost
    ├── http://api.pos.localhost
    ├── http://barber.localhost
    └── http://api.barber.localhost
```

---

## 3. Decisiones técnicas

### 3.1. Por qué Colima

Colima es una alternativa local y gratuita para correr Docker en macOS/Linux. Permite definir recursos de la VM, como CPU, RAM y disco.

Punto importante:

> Colima no reemplaza los comandos `docker` ni `docker compose`. Colima reemplaza la parte pesada que normalmente aporta Docker Desktop: la VM local y el Docker Engine/daemon donde corren los contenedores.

En la práctica:

```txt
docker / docker compose = comandos que tú ejecutas
Colima                  = VM ligera + Docker Engine local
contenedores            = servicios de tus proyectos
```

Por eso, aunque uses Colima, seguirás ejecutando comandos como:

```bash
docker ps
docker compose up -d
docker compose down
docker compose logs -f
```

La diferencia es que esos comandos ya no hablarán con Docker Desktop, sino con el daemon Docker que corre dentro de Colima.

Documentación oficial:

- https://colima.run/docs/getting-started/
- https://colima.run/docs/configuration/

### 3.2. Por qué Traefik

Traefik puede leer labels de Docker y crear rutas automáticamente hacia contenedores.

Documentación oficial:

- https://doc.traefik.io/traefik/reference/routing-configuration/other-providers/docker/

### 3.3. Por qué no publicar puertos por proyecto

Si dos proyectos tienen esto:

```yaml
ports:
  - "5173:5173"
```

solo uno puede ocupar el puerto `5173` del Mac. Por eso el patrón correcto será:

```yaml
expose:
  - "5173"
```

y Traefik se encargará de enrutar por dominio.

Documentación oficial Docker:

- https://docs.docker.com/get-started/docker-concepts/running-containers/publishing-ports/
- https://docs.docker.com/reference/compose-file/networks/

---


## 4. Preparación y limpieza antes de instalar Colima

Esta fase asume que ya detuviste los contenedores actuales y que **no necesitas migrar volúmenes ni datos**, porque tus proyectos se pueden reconstruir con comandos como `make local`, seeds, fixtures o migraciones.

Aun así, la limpieza debe hacerse por niveles. La regla es:

> Primero inspeccionar, luego limpiar Docker, luego instalar Colima. No borres volúmenes si tienes bases de datos locales que no puedas regenerar.

---

### 4.1. Confirmar el estado actual de Docker

Antes de borrar nada, identifica qué motor Docker estás usando y qué queda creado:

```bash
docker context show
docker context ls

docker ps
docker ps -a
docker compose ls

docker system df
docker volume ls
docker network ls
```

Interpretación rápida:

- `docker ps`: contenedores corriendo.
- `docker ps -a`: contenedores detenidos que todavía existen.
- `docker compose ls`: proyectos Compose conocidos por el daemon actual.
- `docker system df`: espacio usado por imágenes, contenedores, volúmenes y build cache.
- `docker volume ls`: volúmenes, normalmente donde viven datos de DBs locales.

Si aparece información importante en bases de datos locales, haz backup antes de seguir. Si todo se puede recrear con `make local`, puedes limpiar con más tranquilidad.

---

### 4.2. Limpieza segura de proyectos detenidos

Si los proyectos ya están detenidos, puedes eliminar contenedores detenidos:

```bash
docker container prune
```

Eliminar redes no usadas:

```bash
docker network prune
```

Eliminar build cache:

```bash
docker builder prune
```

Eliminar imágenes no usadas:

```bash
docker image prune -a
```

Verificar nuevamente:

```bash
docker system df
docker ps -a
docker image ls
docker volume ls
```

---

### 4.3. Limpieza completa cuando no necesitas datos locales

Usa esta opción solo si aceptas perder:

- contenedores detenidos,
- imágenes descargadas/construidas,
- redes no usadas,
- build cache,
- volúmenes no usados, incluyendo datos de DB locales.

Comando completo:

```bash
docker system prune -a --volumes
```

Después:

```bash
docker system df
docker volume ls
docker ps -a
```

Si `docker volume ls` queda vacío o casi vacío, la limpieza fue profunda.

---

### 4.4. Si todavía tienes Docker Desktop instalado

No es obligatorio desinstalar Docker Desktop para usar Colima, pero sí debes evitar confusiones entre contextos.

Recomendación práctica:

1. Primero limpia el daemon actual.
2. Instala y prueba Colima.
3. Solo después decide si quieres desinstalar Docker Desktop.

Cuando Colima esté activo, usa:

```bash
docker context use colima
```

Y verifica:

```bash
docker context show
docker ps
```

Si decides mantener Docker Desktop instalado, la regla será:

```txt
Docker Desktop puede existir, pero tu entorno diario debe usar context = colima.
```

---

### 4.5. Desinstalación opcional de Docker Desktop

Si quieres una instalación realmente limpia y ya confirmaste que no necesitas datos del daemon de Docker Desktop, puedes desinstalarlo.

Si fue instalado con Homebrew Cask:

```bash
brew uninstall --cask docker
```

Si fue instalado manualmente:

1. Cierra Docker Desktop.
2. Mueve Docker.app a la papelera.
3. Reinicia terminal.
4. Verifica:

```bash
docker context ls
which docker
```

Importante: normalmente conviene conservar la CLI `docker` instalada por Homebrew aunque elimines Docker Desktop, porque Colima usa esa CLI para hablar con su daemon.

---

### 4.6. Limpieza profunda manual de Docker Desktop

Esta fase es opcional y destructiva. Úsala solo si quieres borrar restos de Docker Desktop y ya hiciste backup de cualquier dato importante.

Primero revisa si existen estas rutas:

```bash
ls -la ~/Library/Containers | grep -i docker || true
ls -la ~/Library/Application\ Support | grep -i docker || true
ls -la ~/Library/Group\ Containers | grep -i docker || true
ls -la ~/.docker || true
```

Rutas típicas relacionadas con Docker Desktop:

```txt
~/Library/Containers/com.docker.docker
~/Library/Application Support/Docker Desktop
~/Library/Group Containers/group.com.docker
~/.docker
```

No borres `~/.docker` si quieres conservar configuraciones como login a registries, contexts o credenciales. Si tienes dudas, renómbralo como backup en vez de eliminarlo:

```bash
mv ~/.docker ~/.docker.backup
```

Recomendación: no hagas limpieza profunda manual salvo que Docker Desktop esté causando conflictos claros.

---

### 4.7. Symlinks root que pueden quedar después de borrar Docker Desktop

En algunas instalaciones manuales de Docker Desktop pueden quedar enlaces en `/usr/local/bin` apuntando a `/Applications/Docker.app`, por ejemplo:

```txt
/usr/local/bin/docker
/usr/local/bin/docker-compose
/usr/local/bin/docker-credential-desktop
/usr/local/bin/docker-credential-osxkeychain
/usr/local/bin/kubectl.docker
```

Si esos archivos pertenecen a `root`, puede que no se puedan borrar desde una sesión automatizada sin contraseña de administrador. En ese caso, bórralos manualmente con:

```bash
sudo rm -f \
  /usr/local/bin/docker \
  /usr/local/bin/docker-compose \
  /usr/local/bin/docker-credential-desktop \
  /usr/local/bin/docker-credential-osxkeychain \
  /usr/local/bin/kubectl.docker
```

Después instala la CLI de Docker con Homebrew:

```bash
brew install docker docker-compose
```

Verifica que el comando usado sea el de Homebrew:

```bash
which docker
# esperado: /opt/homebrew/bin/docker
```

Si `/opt/homebrew/bin` está antes que `/usr/local/bin` en tu `PATH`, la CLI de Homebrew tendrá prioridad aunque queden symlinks viejos. Aun así, lo más limpio es eliminarlos.

También puede quedar esta carpeta protegida por macOS:

```txt
~/Library/Containers/com.docker.docker
```

Si no se deja borrar por terminal, elimínala desde Finder o concede permisos de privacidad a Terminal/iTerm en:

```txt
System Settings → Privacy & Security → Full Disk Access
```

---

### 4.8. Limpiar una instalación previa de Colima

Si ya habías probado Colima antes y quieres empezar desde cero:

```bash
colima stop
colima delete
```

`colima delete` elimina la VM de Colima y sus datos. Si ya tenías contenedores o volúmenes dentro de Colima, se perderán.

Después puedes crear una nueva VM limpia con los comandos de instalación de esta guía.

---

### 4.9. Estado esperado antes de instalar Colima

Antes de continuar, idealmente deberías tener:

```bash
docker ps
# sin contenedores corriendo

docker ps -a
# sin contenedores viejos importantes

docker system df
# uso bajo o razonable
```

Y a nivel conceptual:

```txt
- Proyectos detenidos.
- Sin necesidad de migrar volúmenes.
- Datos locales regenerables con make local / seeders / migraciones.
- Docker Desktop desinstalado o al menos no usado como contexto diario.
- Listo para instalar Colima limpio.
```

---

## 5. Instalación base

### 5.1. Instalar herramientas

Con Homebrew:

```bash
brew install colima docker docker-compose
```

Si también quieres tener `docker compose` como plugin moderno, normalmente el paquete `docker-compose` de Homebrew lo deja disponible. Verifica:

```bash
docker --version
docker compose version
colima version
```

---

## 6. Configurar Colima

### 6.1. Configuración recomendada para MacBook Air M1 de 8 GB RAM

```bash
colima start \
  --vm-type vz \
  --mount-type virtiofs \
  --cpu 4 \
  --memory 4 \
  --disk 80
```

Notas:

- `--vm-type vz`: usa Apple Virtualization Framework.
- `--mount-type virtiofs`: mejor rendimiento de archivos en macOS moderno.
- `--memory 4`: razonable para un Mac de 8 GB.
- `--cpu 4`: suficiente para dos proyectos sin ahogar macOS.

### 6.2. Configuración recomendada para MacBook Air M1 de 16 GB RAM

```bash
colima start \
  --vm-type vz \
  --mount-type virtiofs \
  --cpu 4 \
  --memory 8 \
  --disk 100
```

### 6.3. Verificar que Colima esté activo

```bash
colima status
docker context ls
docker ps
```

Si Docker no está apuntando a Colima:

```bash
docker context use colima
```

### 6.4. Parar Colima

```bash
colima stop
```

### 6.5. Reiniciar Colima

```bash
colima restart
```

### 6.6. Editar configuración permanente

```bash
colima start --edit
```

Ejemplo de configuración permanente:

```yaml
cpu: 4
memory: 4
disk: 80
vmType: vz
mountType: virtiofs
runtime: docker
kubernetes:
  enabled: false
autoActivate: true
```

Recomendación: **no actives Kubernetes** para este caso. Aumenta consumo y no lo necesitas para correr 2 proyectos con Compose.

---

## 7. Estructura local recomendada

Crear una carpeta central para infraestructura local fuera de los proyectos, por ejemplo `~/local-infra`:

```bash
mkdir -p ~/local-infra/traefik/dynamic
mkdir -p ~/local-infra/certs
```

Estructura esperada:

```txt
~/local-infra
├── README.md
├── projects.yml                 # opcional: registro de proyectos para devctl/UI propia
├── traefik
│   ├── docker-compose.yml
│   └── dynamic
│       └── tls.yml              # opcional para HTTPS local
├── certs                        # opcional para mkcert
├── bin
│   └── devctl                   # opcional: CLI local para start/stop/status
├── dockge                       # opcional: UI externa para stacks Compose
│   └── docker-compose.yml
└── stacks                       # opcional: stacks administrados por Dockge
```

Esta carpeta puede vivir directamente en tu home, por ejemplo `~/local-infra`. No tiene que estar dentro de `~/Trabajo` ni dentro de ningún proyecto.

---

## 8. Crear red compartida para el proxy

```bash
docker network create dev_proxy
```

Si ya existe, Docker mostrará error. Puedes comprobar:

```bash
docker network ls | grep dev_proxy
```

---

## 9. Traefik local HTTP

Archivo:

```txt
~/local-infra/traefik/docker-compose.yml
```

Contenido inicial HTTP:

```yaml
services:
  traefik:
    image: traefik:v3
    container_name: dev-traefik
    command:
      - --api.insecure=true
      - --providers.docker=true
      - --providers.docker.exposedbydefault=false
      - --providers.docker.network=dev_proxy
      - --entrypoints.web.address=:80
    ports:
      - "127.0.0.1:80:80"
      - "127.0.0.1:8080:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    networks:
      - dev_proxy
    restart: unless-stopped

networks:
  dev_proxy:
    external: true
```

Levantar Traefik:

```bash
cd ~/local-infra/traefik
docker compose up -d
```

Ver dashboard:

```txt
http://localhost:8080
```

Ver logs:

```bash
docker logs -f dev-traefik
```

---

## 10. Dominios locales

Tienes dos caminos. Recomiendo empezar con `.localhost` porque no requiere instalar DNS local.

---

### Opción A recomendada: dominios `.localhost`

Ejemplos:

```txt
pos.localhost
api.pos.localhost
barber.localhost
api.barber.localhost
```

Ventajas:

- No necesitas tocar `/etc/hosts`.
- No necesitas `dnsmasq`.
- Funciona muy bien en navegadores modernos.

Desventaja:

- Algunas herramientas CLI antiguas pueden no resolver todos los subdominios de `.localhost`. Si pasa, usa Opción B o C.

---

### Opción B: dominios reales de desarrollo con `dnsmasq`

Ejemplos:

```txt
pos.test
api.pos.test
barber.test
api.barber.test
```

Instalar `dnsmasq`:

```bash
brew install dnsmasq
```

Crear configuración:

```bash
mkdir -p $(brew --prefix)/etc
printf '\naddress=/.test/127.0.0.1\n' >> $(brew --prefix)/etc/dnsmasq.conf
```

Iniciar servicio:

```bash
sudo brew services start dnsmasq
```

Crear resolver de macOS:

```bash
sudo mkdir -p /etc/resolver
echo "nameserver 127.0.0.1" | sudo tee /etc/resolver/test
```

Probar:

```bash
ping pos.test
ping api.pos.test
```

Si no resuelve, limpiar caché DNS de macOS:

```bash
sudo dscacheutil -flushcache
sudo killall -HUP mDNSResponder
```

Notas:

- Evita usar `.local` porque macOS lo usa para Bonjour/mDNS.
- `.test` es adecuado para entornos de prueba locales.

---

### Opción C: `/etc/hosts` manual para pocos proyectos

Si solo necesitas 2 proyectos y no quieres `dnsmasq`:

```bash
sudo nano /etc/hosts
```

Agregar:

```txt
127.0.0.1 pos.test
127.0.0.1 api.pos.test
127.0.0.1 barber.test
127.0.0.1 api.barber.test
```

Desventaja: no soporta wildcard. Cada dominio nuevo debe agregarse manualmente.

---

## 11. Adaptar un proyecto a Traefik

### 11.1. Antes

Ejemplo problemático:

```yaml
services:
  frontend:
    ports:
      - "5173:5173"

  api:
    ports:
      - "8000:8000"

  db:
    ports:
      - "5432:5432"
```

Esto genera conflictos cuando otro proyecto usa los mismos puertos.

### 11.2. Después

Ejemplo recomendado:

```yaml
services:
  frontend:
    build: .
    command: npm run dev -- --host 0.0.0.0
    expose:
      - "5173"
    labels:
      - traefik.enable=true
      - traefik.docker.network=dev_proxy
      - traefik.http.routers.pos-frontend.rule=Host(`pos.localhost`)
      - traefik.http.routers.pos-frontend.entrypoints=web
      - traefik.http.services.pos-frontend.loadbalancer.server.port=5173
    networks:
      - default
      - dev_proxy

  api:
    build: ./api
    expose:
      - "8000"
    labels:
      - traefik.enable=true
      - traefik.docker.network=dev_proxy
      - traefik.http.routers.pos-api.rule=Host(`api.pos.localhost`)
      - traefik.http.routers.pos-api.entrypoints=web
      - traefik.http.services.pos-api.loadbalancer.server.port=8000
    networks:
      - default
      - dev_proxy

  db:
    image: postgres:16
    environment:
      POSTGRES_DB: app
      POSTGRES_USER: app
      POSTGRES_PASSWORD: secret
    volumes:
      - db_data:/var/lib/postgresql/data
    networks:
      - default

volumes:
  db_data:

networks:
  dev_proxy:
    external: true
```

Puntos importantes:

- `frontend` y `api` están en `dev_proxy` porque Traefik debe alcanzarlos.
- `db` no está en `dev_proxy`; solo la app la necesita.
- `db` no publica `5432` al Mac.
- `expose` documenta el puerto interno para otros contenedores.

---

## 12. Ejemplo completo: dos proyectos al mismo tiempo

### 12.1. Proyecto POS

`docker-compose.yml` o `docker-compose.override.yml`:

```yaml
services:
  frontend:
    command: npm run dev -- --host 0.0.0.0
    expose:
      - "5173"
    labels:
      - traefik.enable=true
      - traefik.docker.network=dev_proxy
      - traefik.http.routers.pos-frontend.rule=Host(`pos.localhost`)
      - traefik.http.routers.pos-frontend.entrypoints=web
      - traefik.http.services.pos-frontend.loadbalancer.server.port=5173
    networks:
      - default
      - dev_proxy
    mem_limit: 768m
    cpus: 1.0

  api:
    expose:
      - "8000"
    labels:
      - traefik.enable=true
      - traefik.docker.network=dev_proxy
      - traefik.http.routers.pos-api.rule=Host(`api.pos.localhost`)
      - traefik.http.routers.pos-api.entrypoints=web
      - traefik.http.services.pos-api.loadbalancer.server.port=8000
    networks:
      - default
      - dev_proxy
    mem_limit: 768m
    cpus: 1.0

networks:
  dev_proxy:
    external: true
```

Levantar:

```bash
cd ~/ruta-a-tus-proyectos/pos
docker compose -p pos up -d
```

Acceder:

```txt
http://pos.localhost
http://api.pos.localhost
```

---

### 12.2. Proyecto Barber

```yaml
services:
  frontend:
    command: npm run dev -- --host 0.0.0.0
    expose:
      - "5173"
    labels:
      - traefik.enable=true
      - traefik.docker.network=dev_proxy
      - traefik.http.routers.barber-frontend.rule=Host(`barber.localhost`)
      - traefik.http.routers.barber-frontend.entrypoints=web
      - traefik.http.services.barber-frontend.loadbalancer.server.port=5173
    networks:
      - default
      - dev_proxy
    mem_limit: 768m
    cpus: 1.0

  api:
    expose:
      - "8000"
    labels:
      - traefik.enable=true
      - traefik.docker.network=dev_proxy
      - traefik.http.routers.barber-api.rule=Host(`api.barber.localhost`)
      - traefik.http.routers.barber-api.entrypoints=web
      - traefik.http.services.barber-api.loadbalancer.server.port=8000
    networks:
      - default
      - dev_proxy
    mem_limit: 768m
    cpus: 1.0

networks:
  dev_proxy:
    external: true
```

Levantar:

```bash
cd ~/ruta-a-tus-proyectos/barber
docker compose -p barber up -d
```

Acceder:

```txt
http://barber.localhost
http://api.barber.localhost
```

---

## 13. HTTPS local opcional

Para la mayoría de desarrollo local, HTTP es suficiente. Pero HTTPS local es útil si necesitas:

- Cookies `Secure`.
- OAuth/callbacks.
- PWA/service workers con restricciones.
- Simular producción.
- Laravel Sanctum con cookies cross-subdomain más realistas.

### 13.1. Instalar mkcert

```bash
brew install mkcert nss
mkcert -install
```

### 13.2. Crear certificado wildcard local

Para `.localhost`:

```bash
cd ~/local-infra/certs
mkcert "*.localhost" localhost 127.0.0.1 ::1
```

Esto generará archivos similares a:

```txt
_wildcard.localhost+3.pem
_wildcard.localhost+3-key.pem
```

Renómbralos para simplificar:

```bash
mv _wildcard.localhost+3.pem local-cert.pem
mv _wildcard.localhost+3-key.pem local-key.pem
```

Para `.test`:

```bash
cd ~/local-infra/certs
mkcert "*.test" "*.pos.test" "*.barber.test" localhost 127.0.0.1 ::1
```

---

### 13.3. Configurar Traefik con HTTPS

`~/local-infra/traefik/docker-compose.yml`:

```yaml
services:
  traefik:
    image: traefik:v3
    container_name: dev-traefik
    command:
      - --api.insecure=true
      - --providers.docker=true
      - --providers.docker.exposedbydefault=false
      - --providers.docker.network=dev_proxy
      - --providers.file.directory=/etc/traefik/dynamic
      - --providers.file.watch=true
      - --entrypoints.web.address=:80
      - --entrypoints.websecure.address=:443
    ports:
      - "127.0.0.1:80:80"
      - "127.0.0.1:443:443"
      - "127.0.0.1:8080:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./dynamic:/etc/traefik/dynamic:ro
      - ../certs:/certs:ro
    networks:
      - dev_proxy
    restart: unless-stopped

networks:
  dev_proxy:
    external: true
```

`~/local-infra/traefik/dynamic/tls.yml`:

```yaml
tls:
  certificates:
    - certFile: /certs/local-cert.pem
      keyFile: /certs/local-key.pem
```

Reiniciar Traefik:

```bash
cd ~/local-infra/traefik
docker compose up -d --force-recreate
```

---

### 13.4. Labels HTTPS por proyecto

```yaml
labels:
  - traefik.enable=true
  - traefik.docker.network=dev_proxy
  - traefik.http.routers.pos-frontend.rule=Host(`pos.localhost`)
  - traefik.http.routers.pos-frontend.entrypoints=websecure
  - traefik.http.routers.pos-frontend.tls=true
  - traefik.http.services.pos-frontend.loadbalancer.server.port=5173
```

Acceso:

```txt
https://pos.localhost
```

---

## 14. Vite, Next.js y HMR detrás de Traefik

### 14.1. Vite

El contenedor debe escuchar en `0.0.0.0`:

```bash
npm run dev -- --host 0.0.0.0
```

Si HMR falla, configurar `vite.config.ts`:

```ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    host: '0.0.0.0',
    port: 5173,
    strictPort: true,
    allowedHosts: [
      'pos.localhost',
      'barber.localhost',
    ],
    hmr: {
      host: 'pos.localhost',
      clientPort: 80,
    },
  },
})
```

Para HTTPS:

```ts
hmr: {
  host: 'pos.localhost',
  clientPort: 443,
  protocol: 'wss',
}
```

Si el mismo repo se usa para varios dominios, lee el host desde variable de entorno:

```ts
const devHost = process.env.VITE_DEV_HOST || 'pos.localhost'

export default defineConfig({
  server: {
    host: '0.0.0.0',
    port: 5173,
    allowedHosts: [devHost],
    hmr: {
      host: devHost,
      clientPort: process.env.VITE_DEV_HTTPS === 'true' ? 443 : 80,
      protocol: process.env.VITE_DEV_HTTPS === 'true' ? 'wss' : 'ws',
    },
  },
})
```

### 14.2. Next.js

Usa:

```yaml
command: npm run dev -- -H 0.0.0.0
```

O en script:

```json
{
  "scripts": {
    "dev": "next dev -H 0.0.0.0"
  }
}
```

### 14.3. Laravel

Si usas Laravel dentro del contenedor:

```bash
php artisan serve --host=0.0.0.0 --port=8000
```

Si usas Nginx + PHP-FPM, Traefik debe apuntar al puerto de Nginx, normalmente `80`.

---

## 15. Variables de entorno por proyecto

Ejemplo frontend:

```env
VITE_APP_URL=http://pos.localhost
VITE_API_URL=http://api.pos.localhost
```

Con HTTPS:

```env
VITE_APP_URL=https://pos.localhost
VITE_API_URL=https://api.pos.localhost
VITE_DEV_HTTPS=true
VITE_DEV_HOST=pos.localhost
```

Ejemplo Laravel/API:

```env
APP_URL=http://api.pos.localhost
FRONTEND_URL=http://pos.localhost
SANCTUM_STATEFUL_DOMAINS=pos.localhost,api.pos.localhost,localhost,127.0.0.1
SESSION_DOMAIN=.pos.localhost
CORS_ALLOWED_ORIGINS=http://pos.localhost
```

Con HTTPS:

```env
APP_URL=https://api.pos.localhost
FRONTEND_URL=https://pos.localhost
SANCTUM_STATEFUL_DOMAINS=pos.localhost,api.pos.localhost
SESSION_DOMAIN=.pos.localhost
SESSION_SECURE_COOKIE=true
CORS_ALLOWED_ORIGINS=https://pos.localhost
```

Notas:

- Si usas dominios por proyecto, no mezcles cookies entre proyectos.
- Para `pos.localhost`, usa `SESSION_DOMAIN=.pos.localhost` solo si necesitas compartir entre subdominios como `api.pos.localhost` y `admin.pos.localhost`.
- Si da problemas con cookies en `.localhost`, prueba con `.test` + dnsmasq.

---

## 16. Bases de datos sin conflictos de puertos

### 16.1. Regla recomendada

No publicar DBs al Mac:

```yaml
db:
  image: postgres:16
  expose:
    - "5432"
```

La API se conecta así:

```env
DB_HOST=db
DB_PORT=5432
```

### 16.2. Acceder a la DB desde el Mac solo cuando sea necesario

Opción simple: publicar un puerto único por proyecto solo en override local.

Proyecto POS:

```yaml
db:
  ports:
    - "127.0.0.1:15432:5432"
```

Proyecto Barber:

```yaml
db:
  ports:
    - "127.0.0.1:25432:5432"
```

Tabla sugerida:

```txt
POS PostgreSQL       15432
Barber PostgreSQL    25432
Máximo PostgreSQL    35432
POS MySQL            13306
Barber MySQL         23306
Máximo MySQL         33306
Redis POS            16379
Redis Barber         26379
```

### 16.3. Alternativa: Adminer detrás de Traefik

```yaml
adminer:
  image: adminer
  expose:
    - "8080"
  labels:
    - traefik.enable=true
    - traefik.docker.network=dev_proxy
    - traefik.http.routers.pos-adminer.rule=Host(`adminer.pos.localhost`)
    - traefik.http.routers.pos-adminer.entrypoints=web
    - traefik.http.services.pos-adminer.loadbalancer.server.port=8080
  networks:
    - default
    - dev_proxy
  profiles:
    - tools
```

Levantar solo cuando lo necesites:

```bash
docker compose --profile tools up -d adminer
```

---

## 17. Control de recursos

### 17.1. Límites por servicio

```yaml
services:
  frontend:
    mem_limit: 768m
    cpus: 1.0

  api:
    mem_limit: 768m
    cpus: 1.0

  db:
    mem_limit: 512m
    cpus: 0.5

  redis:
    mem_limit: 128m
    cpus: 0.25
```

### 17.2. Servicios opcionales con profiles

```yaml
services:
  queue:
    build: .
    command: php artisan queue:work
    profiles:
      - workers

  selenium:
    image: selenium/standalone-chrome
    profiles:
      - e2e

  meilisearch:
    image: getmeili/meilisearch:v1.8
    profiles:
      - search
```

Uso normal:

```bash
docker compose up -d
```

Con workers:

```bash
docker compose --profile workers up -d
```

Con búsqueda:

```bash
docker compose --profile search up -d
```

### 17.3. Qué mantener siempre arriba

Para 2 proyectos simultáneos en MacBook Air M1:

```txt
Siempre:
- Traefik
- frontend proyecto A
- api proyecto A
- db proyecto A
- frontend proyecto B
- api proyecto B
- db proyecto B

Bajo demanda:
- workers
- queues
- selenium
- mailpit
- minio
- meilisearch
- elasticsearch
- storybook
```

---

## 18. Comandos diarios

### 18.1. Arrancar entorno

```bash
colima start
cd ~/local-infra/traefik && docker compose up -d
```

### 18.2. Levantar proyecto POS

```bash
cd ~/ruta-a-tus-proyectos/pos
docker compose -p pos up -d
```

### 18.3. Levantar proyecto Barber

```bash
cd ~/ruta-a-tus-proyectos/barber
docker compose -p barber up -d
```

### 18.4. Ver todo lo que está corriendo

```bash
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
```

### 18.5. Ver recursos

```bash
docker stats
```

### 18.6. Apagar un proyecto

```bash
cd ~/ruta-a-tus-proyectos/pos
docker compose -p pos down
```

### 18.7. Apagar todo excepto volúmenes

```bash
docker compose -p pos down
docker compose -p barber down
cd ~/local-infra/traefik && docker compose down
colima stop
```

---

## 19. Scripts opcionales

Puedes crear scripts en `~/local-infra/bin`.

```bash
mkdir -p ~/local-infra/bin
```

### 19.1. `dev-start`

```bash
cat > ~/local-infra/bin/dev-start <<'SCRIPT'
#!/usr/bin/env bash
set -euo pipefail

colima status >/dev/null 2>&1 || colima start

docker network inspect dev_proxy >/dev/null 2>&1 || docker network create dev_proxy

cd "$HOME/local-infra/traefik"
docker compose up -d

echo "Entorno local listo. Traefik: http://localhost:8080"
SCRIPT

chmod +x ~/local-infra/bin/dev-start
```

Agregar al PATH en `~/.zshrc`:

```bash
export PATH="$HOME/local-infra/bin:$PATH"
```

Uso:

```bash
dev-start
```

---


## 20. Panel local para encender/apagar proyectos

Después de tener Colima + Traefik funcionando, tiene sentido agregar una forma cómoda de prender y apagar proyectos. Esto ayuda a ahorrar RAM/CPU porque no tienes que mantener todo arriba todo el tiempo.

La recomendación es hacerlo por fases:

```txt
Fase 1: devctl CLI simple
Fase 2: Dockge como UI ya hecha, si encaja con tu flujo
Fase 3: frontend propio personalizado, solo si Dockge/devctl no son suficientes
```

---

### 20.1. Fase 1 recomendada: `devctl`

Primero crea un registro simple de proyectos en:

```txt
~/local-infra/projects.yml
```

Ejemplo:

```yaml
projects:
  barber:
    path: "~/ruta-a-tus-proyectos/barber"
    project_name: "barber"
    urls:
      app: "http://barber.localhost"
      api: "http://api.barber.localhost"

  maximo:
    path: "~/ruta-a-tus-proyectos/maximo"
    project_name: "maximo"
    urls:
      app: "http://maximo.localhost"
      api: "http://api.maximo.localhost"
```

Comandos objetivo:

```bash
devctl start barber
devctl stop barber
devctl restart barber
devctl logs barber
devctl status
devctl open barber
```

Internamente `devctl` solo ejecutaría comandos conocidos:

```bash
docker compose -p barber up -d
docker compose -p barber stop
docker compose -p barber logs -f --tail 200
```

Ventajas:

- Muy liviano.
- No agrega otro servicio web todavía.
- Fácil de auditar.
- Sirve como base si después construyes un frontend propio.

---

### 20.2. Fase 2 opcional: Dockge como UI lista

Dockge es una UI para manejar stacks de Docker Compose. Puede iniciar, detener, reiniciar, editar compose files y ver terminal/logs.

Importante: Dockge funciona mejor cuando los stacks están organizados dentro de una carpeta de stacks, por ejemplo:

```txt
~/local-infra/stacks
├── barber
│   └── compose.yaml
├── maximo
│   └── compose.yaml
└── pos
    └── compose.yaml
```

Por eso no lo usaría como primer paso obligatorio si tus proyectos ya tienen sus propios `docker-compose.yml` dentro de cada repo. Primero dejaría funcionando Colima + Traefik + `devctl`; después evaluaría si conviene adaptar los stacks para Dockge.

Ejemplo de Dockge en:

```txt
~/local-infra/dockge/docker-compose.yml
```

```yaml
services:
  dockge:
    image: louislam/dockge:1
    container_name: local-dockge
    expose:
      - "5001"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./data:/app/data
      - ../stacks:/opt/stacks
    environment:
      - DOCKGE_STACKS_DIR=/opt/stacks
    labels:
      - traefik.enable=true
      - traefik.docker.network=dev_proxy
      - traefik.http.routers.dockge.rule=Host(`infra.localhost`)
      - traefik.http.routers.dockge.entrypoints=web
      - traefik.http.services.dockge.loadbalancer.server.port=5001
    networks:
      - dev_proxy
    restart: unless-stopped

networks:
  dev_proxy:
    external: true
```

Levantar:

```bash
cd ~/local-infra/dockge
docker compose -p dockge up -d
```

Acceso:

```txt
http://infra.localhost
```

Recomendación: usar Dockge solo después de que el flujo base esté estable.

---

### 20.3. Fase 3 opcional: frontend propio

Si quieres algo totalmente adaptado a tu flujo, se puede construir un panel propio encima de `projects.yml`.

Vista esperada:

```txt
Barber
Estado: running
RAM: 780 MB
CPU: 4%
[Start] [Stop] [Restart] [Logs] [Abrir app] [Abrir API]

Máximo Puntaje
Estado: stopped
[Start] [Stop] [Logs]
```

Arquitectura sugerida:

```txt
~/local-infra/ui
├── frontend    # React/Vite o Next
└── backend     # Node/Go que ejecuta comandos controlados
```

El backend no debería aceptar comandos arbitrarios. Solo acciones permitidas contra proyectos registrados en `projects.yml`:

```txt
start | stop | restart | logs | status | open
```

Esto evita convertir el panel en una terminal remota insegura.

---

## 21. Checklist para adaptar cada proyecto existente

Para cada repo:

- [ ] Revisar `docker-compose.yml` y `docker-compose.override.yml`.
- [ ] Quitar `ports:` de frontend, backend y DB.
- [ ] Cambiar `ports:` por `expose:` en servicios web.
- [ ] Agregar red externa `dev_proxy`.
- [ ] Conectar solo servicios web a `dev_proxy`.
- [ ] Agregar labels Traefik con dominio único.
- [ ] Asegurar que dev server escuche en `0.0.0.0`.
- [ ] Configurar variables `APP_URL`, `FRONTEND_URL`, `API_URL`.
- [ ] Ajustar CORS/cookies si hay frontend separado de API.
- [ ] Agregar `mem_limit` y `cpus`.
- [ ] Mover workers/search/e2e/tools a `profiles`.
- [ ] Levantar con `docker compose -p nombre up -d`.
- [ ] Verificar en `http://localhost:8080` que Traefik detectó rutas.
- [ ] Abrir dominio local en navegador.

---

## 22. Problemas comunes

### 22.1. `address already in use :80`

Otro proceso usa el puerto 80.

Verificar:

```bash
sudo lsof -nP -iTCP:80 -sTCP:LISTEN
```

Opciones:

- Detener el proceso.
- Cambiar Traefik a otro puerto, por ejemplo `8088:80`, aunque perderías dominios sin puerto.

### 22.2. Traefik no detecta el servicio

Revisar:

```bash
docker logs dev-traefik
```

Checklist:

- El servicio tiene `traefik.enable=true`.
- El servicio está conectado a `dev_proxy`.
- El label `traefik.docker.network=dev_proxy` existe.
- El puerto interno coincide con `loadbalancer.server.port`.
- El contenedor está corriendo y healthy.

### 22.3. Dominio no resuelve

Para `.localhost`, prueba en navegador primero.

Para `.test`:

```bash
cat /etc/resolver/test
brew services list | grep dnsmasq
ping pos.test
```

Limpiar caché:

```bash
sudo dscacheutil -flushcache
sudo killall -HUP mDNSResponder
```

### 22.4. Vite carga pero HMR no funciona

Configurar `server.hmr.host` y `clientPort`.

HTTP:

```ts
hmr: {
  host: 'pos.localhost',
  clientPort: 80,
}
```

HTTPS:

```ts
hmr: {
  host: 'pos.localhost',
  clientPort: 443,
  protocol: 'wss',
}
```

### 22.5. La app dentro del contenedor solo escucha en localhost

El proceso debe escuchar en `0.0.0.0`, no en `127.0.0.1`.

Ejemplos:

```bash
npm run dev -- --host 0.0.0.0
php artisan serve --host=0.0.0.0 --port=8000
next dev -H 0.0.0.0
```

### 22.6. Colima se siente lento

Verificar configuración:

```bash
colima status
```

Recomendaciones:

- Usar `--vm-type vz`.
- Usar `--mount-type virtiofs`.
- Evitar montar carpetas gigantes innecesarias.
- Evitar `node_modules` montado desde macOS si el proyecto permite instalarlo dentro del contenedor/volumen.
- Apagar servicios opcionales.
- Revisar `docker stats`.

### 22.7. Memoria insuficiente

Opciones:

- Bajar Colima a 3 GB si macOS sufre, pero tendrás menos margen.
- Mover workers/search/e2e a profiles.
- No correr dos Elasticsearch/Meilisearch al tiempo.
- Compartir Redis/Mailpit entre proyectos si no necesitas aislamiento perfecto.
- Usar `mem_limit` por servicio.

---

## 23. Patrón recomendado de nombres

### 23.1. Project names Docker Compose

Usa nombres explícitos:

```bash
docker compose -p pos up -d
docker compose -p barber up -d
docker compose -p maximo up -d
```

Esto evita nombres raros derivados de carpetas con espacios.

### 23.2. Routers Traefik

Usa nombres únicos:

```txt
pos-frontend
pos-api
barber-frontend
barber-api
maximo-frontend
maximo-api
```

### 23.3. Dominios

Con `.localhost`:

```txt
pos.localhost
api.pos.localhost
admin.pos.localhost
barber.localhost
api.barber.localhost
admin.barber.localhost
```

Con `.test`:

```txt
pos.test
api.pos.test
admin.pos.test
barber.test
api.barber.test
admin.barber.test
```

---

## 24. Seguridad local

Traefik montará:

```yaml
- /var/run/docker.sock:/var/run/docker.sock:ro
```

Esto permite a Traefik leer metadatos de Docker. Es normal para el proveedor Docker, pero sigue siendo acceso sensible. Recomendaciones:

- Mantener Traefik solo en local.
- Publicar puertos como `127.0.0.1:80:80`, no `80:80`.
- No exponer dashboard en red local.
- No usar este compose tal cual en producción.

---

## 25. Plantilla base reutilizable para un proyecto

Copia esto como punto de partida en `docker-compose.override.yml` del proyecto:

```yaml
services:
  frontend:
    expose:
      - "5173"
    labels:
      - traefik.enable=true
      - traefik.docker.network=dev_proxy
      - traefik.http.routers.PROJECT-frontend.rule=Host(`PROJECT.localhost`)
      - traefik.http.routers.PROJECT-frontend.entrypoints=web
      - traefik.http.services.PROJECT-frontend.loadbalancer.server.port=5173
    networks:
      - default
      - dev_proxy
    mem_limit: 768m
    cpus: 1.0

  api:
    expose:
      - "8000"
    labels:
      - traefik.enable=true
      - traefik.docker.network=dev_proxy
      - traefik.http.routers.PROJECT-api.rule=Host(`api.PROJECT.localhost`)
      - traefik.http.routers.PROJECT-api.entrypoints=web
      - traefik.http.services.PROJECT-api.loadbalancer.server.port=8000
    networks:
      - default
      - dev_proxy
    mem_limit: 768m
    cpus: 1.0

networks:
  dev_proxy:
    external: true
```

Reemplazar:

```txt
PROJECT -> pos, barber, maximo, etc.
5173    -> puerto interno real del frontend
8000    -> puerto interno real de la API
```

---

## 26. Fuentes oficiales

- Colima Getting Started: https://colima.run/docs/getting-started/
- Colima Configuration: https://colima.run/docs/configuration/
- Docker port publishing: https://docs.docker.com/get-started/docker-concepts/running-containers/publishing-ports/
- Docker Compose networks: https://docs.docker.com/reference/compose-file/networks/
- Docker Compose project name: https://docs.docker.com/compose/how-tos/project-name/
- Docker Compose profiles: https://docs.docker.com/compose/how-tos/profiles/
- Traefik Docker provider: https://doc.traefik.io/traefik/reference/routing-configuration/other-providers/docker/
- GitLab dnsmasq macOS pattern: https://docs.gitlab.com/development/pages/dnsmasq/

---

## 27. Resumen operativo

La implementación mínima para tu caso es:

```bash
# 1. Inspeccionar y limpiar el Docker actual si no necesitas datos locales.
docker context show
docker ps -a
docker system df
# Opcional y destructivo para volúmenes no usados:
# docker system prune -a --volumes

# 2. Instalar herramientas.
brew install colima docker docker-compose

# 3. Crear Colima limpio.
colima start --vm-type vz --mount-type virtiofs --cpu 4 --memory 4 --disk 80
docker context use colima

# 4. Crear infraestructura central.
mkdir -p ~/local-infra/traefik/dynamic ~/local-infra/certs ~/local-infra/bin ~/local-infra/stacks

# 5. Crear red compartida y levantar Traefik.
docker network create dev_proxy
cd ~/local-infra/traefik
docker compose up -d
```

Luego en cada proyecto:

- quitar `ports`,
- usar `expose`,
- agregar labels Traefik,
- conectar servicios web a `dev_proxy`,
- levantar con project name explícito.

Ejemplo:

```bash
docker compose -p pos up -d
docker compose -p barber up -d
```

Acceso final:

```txt
http://pos.localhost
http://api.pos.localhost
http://barber.localhost
http://api.barber.localhost
```
