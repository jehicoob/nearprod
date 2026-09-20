# Ciclo de vida y cobertura CRUD

Esta matriz distingue metadata de NearProd, runtime Docker y datos persistentes. Una acción sobre el catálogo no implica borrar recursos físicos. El borrado de datos no se ofrece como atajo de limpieza.

| Recurso | Crear | Consultar | Actualizar / operar | Retirar / restaurar | Cobertura y límite |
| --- | --- | --- | --- | --- | --- |
| Aplicación | Registro individual o por descubrimiento | Lista, estado, Compose y rutas | Editar definición; iniciar, detener, reiniciar, reconstruir, watch | Archivar/restaurar aplicación detenida con sus bindings | Cubierto de forma reversible; el comando histórico `remove` se conserva por compatibilidad, pero la UI usa archivo |
| Grupo | Crear explícitamente o al registrar | Lista y aplicaciones | Renombrar | Eliminar solo si no contiene aplicaciones activas o archivadas | Cubierto con preview firmado y revalidación |
| Raíz de descubrimiento | Añadir | Lista y descubrimiento acotado | Volver a descubrir | Retirar solo si ninguna aplicación activa o archivada depende de ella | Cubierto; nunca borra checkouts o carpetas |
| Instancia de infraestructura | Crear | Lista, salud, logs, puertos y recursos | Iniciar, detener, comprobar | Archivar y restaurar metadata sin tocar runtime ni datos | Cubierto de forma reversible; configuración física permanece inmutable |
| Base o credencial lógica | Crear | Lista, conexión y prueba | Reintentar aprovisionamiento, backup y restore SQL | Archivar/restaurar; purgar SQL con backup o confirmación reforzada sin backup | PostgreSQL/MySQL cubiertos. Redis solo permite archivo porque no aísla datos por credencial |
| Vinculación | Crear con preview | Lista y comprobación | Reconfigurar mediante nueva revisión | Desvincular sin borrar la base | Cubierto; reiniciar/revisar la aplicación aplica el overlay resultante |
| Operación | Se crea por cada mutación coordinada | Historial, resultado y logs | Cancelar mientras está activa | Retención acotada automática | No necesita borrado manual básico |
| Runtime y proxy | Configurar/iniciar | Estado, doctor y métricas | Cambiar contexto solo sin recursos vinculados; iniciar/detener proxy | No aplica | Los recursos archivados también bloquean cambio de Engine/contexto |

## Operaciones disponibles

1. Aplicaciones: el archivo mueve la definición y sus bindings a un snapshot con digest. Exige Docker conectado y la aplicación detenida; restaurar no inicia runtime.
2. Grupos: solo se eliminan con cero aplicaciones activas y cero archivadas. Una carrera vuelve a comprobarse antes de escribir.
3. Raíces: solo se retira la referencia exacta si ningún path activo o archivado está dentro de ella. El filesystem no cambia.
4. Bases: el archivo reversible conserva base, cuenta, vault y datos. La purga PostgreSQL/MySQL exige ausencia de bindings y motor activo; puede crear/verificar un backup primero o exigir ID exacto y aceptación de pérdida de datos.
5. Configuración de infraestructura: puerto, imagen y persistencia permanecen inmutables sobre datos existentes. El flujo seguro es crear otra instancia, exportar/restaurar, validar consumidores y archivar el origen.

## Invariantes de instancia archivada

- Solo una instancia detenida y sin bindings puede archivarse.
- El preview no escribe catálogo ni ejecuta mutaciones Docker.
- La ejecución vuelve a comprobar fingerprint, metadata, Engine, endpoint, ownership, recursos y persistencia.
- NearProd serializa sus propias operaciones, pero no puede bloquear transaccionalmente a otro proceso con acceso directo al daemon Docker. No ejecutes comandos Docker externos sobre la instancia durante el archivo/restauración.
- El snapshot guarda la instancia y sus bases con digest; no contiene contraseñas, contenido SQL ni salida Docker arbitraria.
- IDs, proyectos, redes, hostnames, puertos, volúmenes y carpetas permanecen reservados.
- Restaurar conserva UID y credenciales, no arranca ni detiene contenedores y no cambia datos.
- No existe purga física implícita. La destrucción de una base es una operación diferente, explícita y auditable.

## Invariantes de aplicaciones, grupos y raíces

- Una aplicación archivada reserva ID, UID, proyecto Compose, rutas locales y grupo; sus bindings quedan dentro del snapshot y siguen bloqueando el ciclo de vida de la base relacionada.
- Archivar una aplicación exige que sus contenedores estén detenidos y sean propiedad del catálogo. No ejecuta Compose ni Docker.
- Restaurar una aplicación exige que sus bases vinculadas sigan activas. No inicia contenedores.
- Una aplicación archivada cuenta como dependencia del grupo y de la raíz para que ambos sigan disponibles al restaurarla.
- Eliminar un grupo o retirar una raíz solo modifica metadata y vuelve a comprobar dependencias bajo el lock de escritura.

## Invariantes de bases

- Archivar una base no ejecuta SQL: conserva la base física, usuario, credencial y almacenamiento, y reserva nombre/ID/cuenta.
- Bases con bindings activos o guardados dentro de aplicaciones archivadas no pueden archivarse ni purgarse.
- Una instancia no puede archivarse mientras contenga bases archivadas individualmente; primero deben restaurarse o purgarse.
- `backup-purge` termina y verifica dump/manifiesto/hash antes de ejecutar `DROP DATABASE`; si el backup falla, no se purga.
- `purge` sin backup exige confirmación, aceptación explícita de pérdida y el ID exacto de la base.
- PostgreSQL usa `DROP DATABASE ... WITH (FORCE)` fuera de la base destino y elimina después el rol limitado. MySQL elimina esquema y cuenta limitada.
- Al comenzar, la base pasa a `purging` y guarda modo, digest, backup y fase (`database`, `account` o `metadata`). Una falla parcial conserva metadata/vault y ese progreso para reintentar con el mismo modo.
- Cada reintento verifica el estado físico esperado antes de continuar: no elimina una base o cuenta reaparecida, ni una identidad que haya adquirido privilegios o recursos ajenos. Las fases ya completadas son idempotentes.
- Una base en `purging` no puede vincularse, archivarse ni aprovisionarse como si estuviera lista. Al completar SQL, se retira la credencial y por último la metadata; si esa escritura final falla, NearProd intenta restaurar y verificar el vault.
- Redis no ofrece purga por credencial: su instancia dedicada comparte el espacio de datos y debe archivarse o retirarse completa.
