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

## Guía para el agente y paridad con la web

El saludo MCP indica leer `nearprod_guide` y consultar `nearprod_capabilities`. La guía también está publicada como recurso `nearprod://guide`; el catálogo completo de esquemas está en `nearprod://operations`. Son parte del binario, por lo que un cliente no necesita acceso al repositorio de NearProd para entenderlo.

La guía cubre requisitos y restricciones de Compose, modos `dev`/`verify`, ejemplos, diagnóstico de memoria, salud, confirmaciones y el recorrido **inspeccionar → explicar → editar → validar → aplicar → comprobar**. Un Compose válido para Docker no implica compatibilidad automática: NearProd exige archivos explícitos y rechaza `include`/`extends` para revisar completamente su fingerprint. La edición de Compose/Dockerfile se hace con las herramientas de archivos del propio agente y dentro de lo autorizado; el MCP modifica metadata, no ofrece un editor general de archivos.

`nearprod_capabilities` devuelve un índice compacto. Selecciona `area` y `operation` para obtener el esquema completo, ruta, herramienta y resultado esperado; `schemas=true` incluye los esquemas de todas las operaciones seleccionadas. Los esquemas del catálogo describen la API HTTP; `tools/list` describe las entradas de cada herramienta MCP.

| Herramientas | Cobertura |
| --- | --- |
| Herramientas existentes de aplicaciones/grupos | Registro, edición completa, revisión/aprobación, operación, archive/restore/remove/purge, grupos y seguimiento de operaciones |
| `nearprod_applications` | Registro por lote, adopción de existentes y Watch |
| `nearprod_roots` | Agregar, revisar y retirar raíces |
| `nearprod_diagnostics` | Estado, salud del agente, doctor, métricas, imágenes y rutas de configuración |
| `nearprod_runtime` | Selección de runtime, inicio y revisión/aplicación de recursos de Colima |
| `nearprod_proxy` | Estado, revisión, inicio/parada y comprobación de acceso |
| `nearprod_infrastructure` | Instancias, bases/usuarios, vinculaciones, conexiones, pruebas, backups/restauración y ciclo de vida |
| `nearprod_maintenance` | Inventario/versiones de herramientas, revisión/aplicación, actualizaciones, estado de inicio y backup privado de configuración |
| `nearprod_application_logs`, `nearprod_infrastructure_logs` | Logs sin seguimiento, hasta 500 líneas y 512 KiB, con plazo de 30 segundos y señal de truncamiento |

Las herramientas por área reciben `operation` y `parameters` con un esquema específico para cada operación. Las herramientas originales conservan sus nombres. Para confirmar eliminación de grupo o archive/restore/purge de aplicaciones ahora debe enviarse el `fingerprint` revisado; el MCP no lo sustituye automáticamente por uno nuevo. Purge además exige `typedId` y `acknowledgeDataLoss=true`. `remove` sigue siendo retiro de metadata y **no** significa limpieza completa.

Las operaciones asíncronas devuelven `id`; consúltalo con `nearprod_operation` hasta un estado terminal y comprueba luego la salud real. Los errores de dominio conservan `code`, `message` y `details` en `result.error`, con `isError=true`.

La paridad es operativa: los eventos SSE de la web se sustituyen por consultas de estado/operaciones, y los logs usan sus herramientas acotadas. Login/logout/emparejamiento y el estado visual de los acordeones pertenecen al navegador. Configurar inicio automático, ejecutar migración y detener el agente siguen siendo recorridos administrativos del CLI; el puente avanzado mantiene acceso a endpoints JSON locales. No envíes `/events`, `/logs` ni `/infra/logs` a `nearprod_api`.

Las pruebas comparan las rutas HTTP y acciones de dominio con el catálogo MCP, ejercitan esquemas y recorridos con agentes temporales y verifican que los detalles de errores y las confirmaciones lleguen íntegros. Esto no sustituye pruebas reales del framework de cada aplicación.
