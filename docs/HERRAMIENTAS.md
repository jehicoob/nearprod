# Herramientas del equipo

NearProd Go es el controlador y ya no necesita un runtime Node. Herramientas detecta binarios/rutas/versión y posibles gestores (Homebrew, fnm, nvm, asdf, mise, Volta, npm/pnpm). Detectar un gestor no le atribuye control de todas las herramientas; fnm no instala Docker y NearProd no cambia su default.

Docker CLI habla con Engine. Compose resuelve/ejecuta las apps. Buildx es necesario para construcciones que lo requieran, no para consultar logs. Colima proporciona el perfil VM existente en macOS. Traefik es una imagen core administrada desde Accesos locales, no un paquete nativo ni una DB opcional.

```bash
nearprod tools list
nearprod tools versions --tool compose
nearprod tools repair --tool compose
nearprod tools repair --tool compose --yes
```

La escritura inicial soportada es Homebrew macOS: consulta las fórmulas realmente disponibles y hace preview de versión/dependencias. No hay selector de todas las versiones históricas ni instalación de Homebrew, sudo, taps, upgrades globales o eliminación de proveedores ajenos.

Compose y Buildx instalados pero no descubiertos por docker se reparan mediante cliPluginsExtraDirs. Merge/backup privado de Docker config, sin reemplazar auths ni otras claves. Un JSON inválido, symlink o cambio concurrente impide sobrescribir.

El inventario separa Node de desarrollo de Go activo. Las actualizaciones de Docker CLI no demuestran una actualización del Engine dentro de Colima. Mantenimiento global se bloquea durante builds/Watch; actualizar Colima requiere su VM detenida. Estas operaciones se prueban con Homebrew simulado, no se ejecutaron en un Mac en esta revisión.
