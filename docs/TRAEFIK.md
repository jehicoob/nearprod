# Accesos locales y compatibilidad

Un Traefik administrado por catálogo, en el Docker/Colima existente. NearProd Go conserva owner, project/network/UID, claves de router y alias generados por 0.6.1. No adopta un Traefik externo porque se llame dev-traefik. No instala Traefik nativo ni otra VM.

```bash
nearprod proxy status
nearprod proxy start --port 80
nearprod proxy start --port 80 --yes
nearprod url tienda/api --host api-tienda.localhost --service caddy --port 80
nearprod review tienda/api
nearprod review tienda/api --approve --yes --allow-unsafe
nearprod up tienda/api
nearprod url-check tienda/api --host api-tienda.localhost
```

Lee primero los riesgos antes de allow-unsafe. Las URLs son HTTP proyecto.localhost; un puerto distinto de80 se incluye en todas. Guardar una URL no crea/recrea contenedores. Activa el proxy, revisa e inicia. El overlay conserva redes originales y conecta solo servidores web elegidos; no monta socketDocker ni expone dashboard.

Se comprueban publicacionesDocker y listener TCP real. EACCES/EPERM del intento bind local puede permitir continuar solo cuando no hay listener y se puede delegar a Docker; estado desconocido o un puerto ocupado bloquea. Esto conserva el arreglo del caso Mac detectado en0.6.1. No se toma un puerto ajeno ni se ejecuta sudo.

No se borran puertos publicados del Compose: dos apps con un mismo hostport seguirán chocando. No se modifica CORS, hosts, APP_URL, API_URL, WebSocket/HMR, cookies o TLS forzado. Un backend HTTP debe escuchar en una interfaz accesible; PHP-FPM no es HTTP. Caddy80 y Traefik80 son puertos de contenedores diferentes, no un conflicto entre sí.

Comprobar acceso distingue el router esperado mediante respuesta y por separado el DNS. Un401/404 puede ser de la app, no un fallo del proxy; responderHTTP no certifica salud. .localhost debe comprobarse con el navegador/herramienta del equipo. No se modifica /etc/hosts ni DNS o certificados. No se publica al móvil/LAN, ni se enruta SQL como HTTP.

Detener proxy no detiene apps. Reaplicar lo puede recrear e interrumpir temporalmente URLs. Un up externo sin overlay puede quitar la conexión de NearProd; revisar/iniciar la restaura. El modo verify puede tener otro puerto interno configurado.

La aceptación real incluida usa Engine y HTTP real. No pudo ejecutarse en este entorno. Los tests unitarios/UI usan un proxy simulado explícito, no se anuncian como pruebas de Traefik real.
