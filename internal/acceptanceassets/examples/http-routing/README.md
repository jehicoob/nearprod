# Dos URLs, un Compose

Demostración HTTP con `traefik/whoami:v1.11.0`. No sustituye un frontend/API real ni se ha ejecutado la imagen en el entorno de generación; forma parte de la aceptación preparada. No publica puertos en el host ni utiliza datos del usuario.

Autoriza la carpeta `examples` como raíz y registra esta aplicación. Selecciona únicamente `compose.yaml`. Configura estas dos URLs en **URL de acceso — Traefik**:

| Dominio sugerido | Servicio | Puerto interno |
|---|---|---:|
| `demo.localhost` | `frontend` | 8080 |
| `api-demo.localhost` | `api` | 8080 |

Activa el proxy en **Accesos locales**, revisa/aprueba e inicia la aplicación. Abre ambos enlaces; el nombre devuelto permite distinguir los servicios. No hay healthcheck declarado: running no prueba salud; comprueba HTTP. Si ya usas esos nombres, elige otros únicos.

Detener conserva los contenedores y el proxy. Quitar del catálogo no los elimina; no hay volúmenes de datos en este ejemplo. La limpieza automatizada solo pertenece al script aislado `scripts/smoke-proxy.mjs`, no a este registro manual.
