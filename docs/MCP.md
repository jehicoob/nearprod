# Acceso MCP local

NearProd incluye un servidor MCP local para consultar, configurar y operar el mismo catálogo del panel y del CLI. Está pensado para una sola persona en su equipo local: **no implementa usuarios, perfiles de permisos, sesiones ni credenciales MCP**.

## Configuración

Abre **Acceso MCP** en el panel y copia el bloque generado. Usa la ruta absoluta del binario y el `NEARPROD_HOME` que el panel tiene abierto:

```json
{
  "mcpServers": {
    "nearprod": {
      "command": "/ruta/absoluta/a/nearprod",
      "args": ["mcp", "serve"],
      "env": {
        "NEARPROD_HOME": "/ruta/al/home/de/nearprod"
      }
    }
  }
}
```

El proceso habla MCP solo por stdin/stdout. Inicia o reutiliza el agente NearProd en `127.0.0.1`, pero no abre un puerto adicional ni incluye tokens en la configuración del cliente.

## Alcance

El cliente recibe acceso total al catálogo seleccionado:

- Consultar catálogo, estado, runtime, infraestructura, operaciones y herramientas.
- Descubrir, registrar, editar, revisar y aprobar aplicaciones.
- Iniciar, detener, reiniciar y reconstruir aplicaciones o grupos.
- Consultar logs acotados y cancelar operaciones.
- Crear, modificar o retirar metadata y llamar a la API local avanzada, incluida infraestructura.

Las herramientas específicas ofrecen esquemas claros para los recorridos habituales. `nearprod_api` permite usar cualquier endpoint JSON local no cubierto todavía por una herramienta específica.

## Límites de seguridad

- Cualquier proceso capaz de iniciar esta configuración controla todos los recursos del `NEARPROD_HOME` indicado. Configura solo clientes y agentes de confianza.
- MCP no lee ni escribe directamente el catálogo, Docker o SQL. Todas las llamadas pasan por el agente loopback, que sigue siendo el único escritor.
- Las acciones destructivas mantienen las mismas comprobaciones de ownership, previews, fingerprints, confirmaciones y aceptación explícita de pérdida de datos de la API y el CLI.
- Que una herramienta esté marcada como destructiva es una señal para el cliente MCP; no reemplaza las validaciones de NearProd.
- No compartas `agent.json`, backups de configuración, vaults, salida con credenciales ni el directorio completo de NearProd con un servicio remoto.

Para retirar acceso, elimina esta entrada de la configuración del cliente y detén sus procesos MCP. El agente NearProd puede continuar para el panel y el CLI.
