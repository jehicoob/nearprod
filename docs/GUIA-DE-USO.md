# Guía del panel

Empieza por [INICIO-RAPIDO](INICIO-RAPIDO.md).

## Campos que suelen confundirse

**Grupo:** etiqueta organizativa; no fusiona Compose ni renombra recursos.

**Archivos Compose:** selección ordenada de la MISMA aplicación. Los siguientes pueden combinar/reemplazar valores; no son archivos de código que se deban pegar. Selecciona overrides explícitamente.

**Entorno automático:** delega las reglas de .env a Compose. Elegir archivos suministra --env-file; no inyecta todas las variables al contenedor automáticamente. Plantillas no son credenciales funcionales.

**Perfiles:** servicios opcionales declarados en Compose. No son los perfiles de Colima ni los grupos.

**Prueba de imagen:** configuración opcional para ejecutar código empaquetado; no garantía de producción ni de datos separados.

**Puerto URL:** puerto interno del servicio HTTP; el puerto de entrada de Traefik se elige una vez en Accesos locales.

**Carpeta de datos:** toda una instancia SQL, no cada base lógica; los proyectos importados conservan su propia persistencia.

Los skeletons diferencian carga y falta de datos. La vista conserva el último estado durante refresh. Desconexión no se representa como cero consumo. Detener no borra volúmenes.
