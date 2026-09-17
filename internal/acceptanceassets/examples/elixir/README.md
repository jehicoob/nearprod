# Elixir: fixture de acceso HTTP

Servicio `web`, puerto interno **4000**; sugiere dominio `elixir-demo.localhost`.
Registra `compose.yaml`, configura esa URL, activa el proxy, revisa/aprueba e inicia.
`/` y `/health` responden JSON con `nearprod-elixir`.

Este servidor diagnóstico usa Elixir/Erlang sin Hex para comprobar imagen, puerto, rutas y operaciones. No sustituye Phoenix/Bandit en producción. Consulta `docs/ELIXIR.md` para integrar tu aplicación real.

La aceptación `nearprod self-test --yes --context colima` usa una copia aislada. **No se ejecutó una imagen Elixir ni un intérprete Elixir en el entorno de auditoría.**
