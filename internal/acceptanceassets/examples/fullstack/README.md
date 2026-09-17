# Demo frontend + API

Dos servicios reales HTTP en un único Compose, sin dependencias npm y sin publicar puertos del host. No es un framework de producción ni una sustitución de tu React/FastAPI.

Copia esta carpeta a una raíz autorizada. Registra una aplicación con `compose.yaml` y dos URLs:

| Dominio | Servicio | Puerto interno |
|---|---|---|
| `demo.localhost` | `frontend` | `3000` |
| `api-demo.localhost` | `api` | `3000` |

Activa Traefik en 80, revisa/aprueba e inicia. Abre `http://demo.localhost` y pulsa **Consultar API**: el navegador solicita `http://api-demo.localhost/api/message`.

Para cambiar dominios o usar proxy en 8080, copia `.env.example` a `.env`, ajusta ambos valores y vuelve a revisar e iniciar para recrear con el nuevo entorno. No basta `restart` para aplicar variables distintas. CORS permite solo el origen exacto configurado, no `*`.

El HTML/JSON/CORS se probó por HTTP nativo sobre Linux. Las imágenes y el acceso mediante Traefik NO se ejecutaron en el entorno de auditoría. La aceptación real los comprueba con copias temporales y dominios aleatorios; sus recursos se limpian al finalizar.
