# Instancias compartidas y persistencia

Traefik pertenece a Accesos locales, no a las bases opcionales. Cerca de tu Colima hay un solo runtime; encender otra instancia no crea otra VM. Mantén apagados motores que no uses.

## Crear y vincular PostgreSQL/MySQL

Panel: Infraestructura → Crear instancia → motor/imagen → nombre/ID → persistencia → recursos → puerto opcional → revisar/confirmar. Imágenes admitidas: PostgreSQL 17/18, MySQL 8.4. No seleccionar latest o major incompatible sobre datos existentes.

Ejemplo de CLI (cada mutación usa confirmación):

```bash
nearprod infra ports --from 15432 --to 15442
nearprod infra create --engine postgres --id pg-main --persistence volume --port 15432
nearprod infra create --engine postgres --id pg-main --persistence volume --port 15432 --yes
nearprod infra database --instance pg-main --name tienda_dev --yes
nearprod infra list
nearprod infra connection --database ID_DEVUELTO
```

`--persistence volume` conserva los datos en un volumen administrado por Docker aunque se recree el contenedor; no uses `docker volume prune` ni `docker compose down -v` sobre recursos que quieras conservar. `--persistence folder` sin ruta propone `~/.nearprod/databases/postgres/pg-main`; los archivos del motor quedan en su subcarpeta data. `--data-dir` permite otra carpeta dedicada nueva/vacía. PostgreSQL 18 con Docker nativo en Linux/WSL2 requiere `volume` en esta versión porque el UID del motor no puede atravesar de forma segura una carpeta privada del usuario host; NearProd lo rechaza antes de crear recursos. El puerto es opcional; para acceso solo entre contenedores no hace falta publicarlo.

Memoria se expresa en MiB para instancia y GiB para Colima. Es un límite, no una reserva. No hay garantía de que todas las bases y aplicaciones quepan en 2 GiB. MySQL requiere más margen en sus defaults. La cuenta de app no es root/postgres/npadmin.

Por cada proyecto SQL crea una base lógica y un usuario propios. Una carpeta física representa toda la instancia; no hay selector de carpeta por base/schema. PostgreSQL dev/verify pueden ser bases distintas compartiendo motor; su CPU/RAM y fallos siguen compartidos.

## Conexión de un proyecto

Panel: base → Vincular proyecto → aplicación → servicios consumidores → modo → nombres de variables → revisar y confirmar. No selecciones un frontend estático que compila variables en el navegador para entregarle contraseñas. La API/worker recibe env del contenedor, no un `.env` reescrito en el checkout.

```bash
nearprod infra bind --target tienda/api --database ID_DEVUELTO --services api,worker --url-var DATABASE_URL
nearprod infra bind --target tienda/api --database ID_DEVUELTO --services api,worker --url-var DATABASE_URL --yes
nearprod review tienda/api
nearprod review tienda/api --approve --yes --allow-unsafe
nearprod up tienda/api
nearprod infra check --database ID_DEVUELTO
nearprod infra check-binding --binding ID_VINCULO
```

`--allow-unsafe` solo después de leer las capacidades/redes que vas a autorizar. No es una bandera que se aplique indiscriminadamente. Usar Iniciar aplica el overlay; Reiniciar mantiene la configuración existente.

Dentro de Docker utiliza el hostname interno mostrado y 5432/3306/6379. En el Mac usa 127.0.0.1 y el puerto publicado elegido. No uses proyecto.localhost como destino SQL ni localhost para encontrar otro contenedor.

Los puertos propuestos tienen disponibilidad momentánea, se comprueban otra vez antes de iniciar. La publicación se limita a loopback. No se detienen procesos ajenos para liberar un puerto.

## Tu Compose existente

Su base, depends_on, volumes y migraciones no se eliminan. Puedes no usar infraestructura compartida. Si decides cambiar, prepara la configuración de desarrollo y los datos deliberadamente; no se copia el contenido de la DB propia ni se desactiva su servicio automáticamente. Un Compose existente tampoco garantiza persistencia si nunca la configuraste.

## Protección de datos

Las carpetas se validan y tienen marcador de propiedad. No se adopta un directorio existente por adivinar su motor. En Colima, comprobar visibilidad/escritura puede fallar por montajes/permisos: utiliza carpeta compartida o volumen, no chmod777. En Linux/WSL2 no cambies propietario o permisos recursivamente para forzar PostgreSQL 18: crea otra instancia con volumen y restaura un backup lógico si ya había datos. No muevas ni sincronices con nube una carpeta de DB activa.

El secreto administrativo se almacena en vault0600; el archivo de secreto que lee el entrypoint tiene permisos de lectura para el UID del contenedor dentro de una carpeta privada del host. Esto evita que el entrypoint cambie a postgres/mysql y deje de poder leer un bind 0600 propiedad del UID macOS. No promete secreto frente al propio usuario o un administrador Docker.

No borres vaults: sin las credenciales de una instancia inicializada NearProd bloquea el arranque, no las reinventa. Tampoco recrea un volumen desaparecido si la metadata dice que contiene datos.

Stop de un proyecto no detiene la base. Stop de la instancia muestra consumidores y puede requerir `--allow-active`. No hay borrado de bases/datos en la app ni down -v/prune global.

## Archivar y restaurar una instancia

Archivar retira una instancia del catálogo activo sin borrar ni modificar contenedores, redes, volúmenes, carpetas, imágenes, vaults, credenciales o datos. Antes exige que el motor esté detenido, que no tenga vinculaciones y que Engine, endpoint, ownership y persistencia sigan coincidiendo. El preview es de solo lectura y la confirmación vuelve a comprobar su fingerprint.

```bash
nearprod infra stop --instance pg-main --yes
nearprod infra archive-instance --instance pg-main
nearprod infra archive-instance --instance pg-main --yes
nearprod infra list
nearprod infra restore-instance --instance pg-main
nearprod infra restore-instance --instance pg-main --yes
```

Mientras esté archivada se reservan ID, UID, proyecto Compose, hostname, red, volumen, carpeta y puerto. No se puede cambiar de Engine/contexto ni crear otra instancia o aplicación con esas identidades. Restaurar devuelve la misma metadata activa y tampoco inicia el runtime. `nearprod infra restore` sigue siendo exclusivamente la importación SQL de un backup; no restaura una instancia archivada.

Una base individual también puede archivarse sin ejecutar SQL. Debe estar sin vinculaciones, incluidas las conservadas por aplicaciones archivadas. Su nombre, ID, usuario, credencial y datos quedan reservados hasta restaurarla o purgarla:

```bash
nearprod infra archive-database --database ID
nearprod infra archive-database --database ID --yes
nearprod infra restore-database --database ID --yes
```

PostgreSQL y MySQL permiten una purga física explícita. La opción recomendada crea primero el backup privado y su manifiesto/hash; un fallo de backup aborta antes de borrar. La variante sin backup exige una bandera propia además de `--yes`:

```bash
nearprod infra purge-database --database ID --backup --yes
nearprod infra purge-database --database ID --backup --directory /ruta/privada --yes
nearprod infra purge-database --database ID --without-backup --acknowledge-data-loss ID --yes
```

La purga exige motor activo y cero bindings; elimina solo la base/esquema y su cuenta limitada. No elimina la instancia, bases hermanas, volúmenes o carpetas. Redis no permite esta operación porque una credencial no delimita físicamente sus claves; usa archivo reversible o retira la instancia dedicada completa.

## Backup y restauración

```bash
nearprod infra backup --database ID_DEVUELTO --yes
nearprod infra restore --database ID_BASE_NUEVA --file /ruta/archivo --trusted-backup --yes
```

Backups por defecto: `~/.nearprod/backups/databases`; permiten carpeta alternativa. Stream binario a disco, archivos privados con manifiesto/hash. Restore exige motor/familia compatible, destino vacío y no vinculado y un backup explícitamente confiado, sin elevar privilegios para importar SQL arbitrario. MySQL no promete importar DEFINER ajenos, rutinas/eventos de cualquier origen. Un fallo puede dejar objetos parciales; no hay rollback ficticio.

`config backup` guarda metadata/credenciales, no datos SQL; un volumen o carpeta no sustituye ninguna copia externa. No hay upgrade mayor ni cambio in situ de puerto/carpeta: prepara otra instancia y exporta/restaura de manera controlada.

## Redis

Instancia dedicada a una aplicación; no aislamiento por prefijos o números de base. Soporta conexiones URL REDIS_URL, persistencia AOF o modo efímero explícito, permisos limitados sin comandos administrativos globales. Una instancia compartida SQL no implica que toda caché y cola deban mezclarse en Redis. No hay asistente gráfico de backup/restore físico Redis.

## Comprobación real

`nearprod self-test --yes --context colima --infra-only --folder` comprueba carpetas en Colima. En Linux/WSL2 usa `nearprod self-test --yes --context default --infra-only`, que valida volúmenes, SQL, credenciales, persistencia tras recreación, restore y Redis. La aceptación de tus migraciones y framework requiere tu proyecto real.
