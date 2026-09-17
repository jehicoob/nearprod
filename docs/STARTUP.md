# Inicio automático en macOS

```bash
nearprod startup enable
nearprod startup status
nearprod startup disable
```

Se registra `~/Library/LaunchAgents/dev.nearprod.agent.<hash-catalogo>.plist` usando el ejecutable nativo estable `~/.local/bin/nearprod`. Es un LaunchAgent de tu usuario; no usa sudo ni arranca antes del login/FileVault. Solo inicia el controlador, no navegador, Colima, Traefik, bases o proyectos.

Se conserva el label del HOME anterior para no crear otra instalación paralela. Tras migrar desde 0.6 ejecuta enable otra vez: el nuevo plist elimina la dependencia de Node. Si estaba cargado, el registro de esa sesión puede mantener sus argumentos previos hasta el próximo login; NearProd no interrumpe ni duplica un agente. Detén explícitamente el anterior y abre `nearprod ui` para utilizar ahora el nativo.

RunAtLoad activado, KeepAlive desactivado. Detener NearProd es una intención que no se revierte automáticamente en bucle. `disable` retira el inicio futuro pero no mata la sesión actual. `nearprod agent stop` espera el cierre del socket antes de terminar para permitir una migración inmediata segura.

No evalúa .zshrc ni copia secretos del entorno al plist. Mantiene PATH estable, HOME y configuraciones Docker/Colima aprobadas; las apps deben tener su configuración explícita en los archivos o vínculos, no depender de un export temporal de otra terminal.

`status` muestra archivo/label/enabled/loaded; no es un chequeo de Docker. Ante problemas revisa `nearprod --identity`, el archivo/log mostrado y permisos de tareas en segundo plano de macOS. No modifiques plists ajenos ni agregues nearprod ui a .zshrc para simular autoarranque.

Linux/WSL no recibe un job de launchd ni un systemd inventado. Tests ejecutados con XML/archivos reales y launchctl simulado. Cerrar/iniciar sesión debe validarse en tu Mac.
