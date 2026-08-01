# Alcance de experiencia NearProd 1.0.0: personas y journeys

- **Jira:** NPROD-80 / UX-01
- **Estado:** Aprobado como alcance objetivo 1.0.0 — capacidades condicionadas a sus gates
- **Fecha:** 2026-08-01
- **Responsable de aprobación:** Jehicoob López, Product Owner
- **Evidencia de aprobación:** autorización explícita para fusionar el PR #3 el 2026-08-01
- **Alcance:** instalación, primer uso, incorporación y operación local en macOS/Colima y Linux/Docker
- **Exclusiones:** implementación UI, investigación de mercado y generalización de estas personas a toda la población

## Propósito y fuentes

Este artefacto traduce las capacidades técnicas de NearProd en resultados que una persona desarrolladora debe poder completar y demostrar. Las personas son arquetipos operativos para revisar el producto; no son segmentos de mercado ni evidencia estadística.

El mapa se contrastó con el baseline `cab6fa33dd9a75b3b47b79a2167e29d00a968a19`:

- `README.md` y `bin/dev-start` describen el arranque actual centrado en Colima;
- `ui/server.mjs` expone detección, registro, acciones, logs y jobs;
- `ui/src/components/AddProjectWizard.tsx` ofrece fuentes local/GitHub/Compose/manual, pero la clonación Git real todavía no existe;
- `ui/src/components/ProductCard.tsx` y `LogsModal.tsx` cubren operación y diagnóstico básico;
- `docs/manual-uso-aplicativo.md` documenta el flujo actual;
- NPROD-15 define la frontera de seguridad objetivo, aún no implementada.

Cuando el comportamiento 1.0.0 todavía no existe o no ha sido probado con usuarios, se marca **Hipótesis**. Una hipótesis no se convierte en promesa de release sin la evidencia y aprobación indicadas.

## Capacidad actual frente al alcance objetivo

Este documento **define el alcance objetivo y sus gates; no certifica que NearProd 1.0.0 esté implementado, soportado o listo para release**. El Product Owner puede aprobar el mapa de experiencia sin aprobar como completadas las capacidades que describe. La aceptación de cada journey requiere evidencia posterior contra el producto ejecutable.

| Capacidad | Baseline actual | Estado en este alcance | Gate antes de afirmar soporte 1.0.0 |
| --- | --- | --- | --- |
| macOS + Colima | `bin/dev-start` inicia Colima y selecciona su contexto sin preflight formal | Objetivo principal, aún no validado en host limpio | Instalación/primer arranque en macOS real y criterios J1/J2. |
| Linux + Docker Engine | El mismo script intenta usar Colima; no existe adaptador Linux | Objetivo requerido, **no implementado y no soportado todavía** | Implementación multiplataforma, tests en Linux real y aprobación NPROD-33. |
| Preflight guiado | Dashboard consulta contexto/versión; no hay contrato `aprobado/advertencia/bloqueado/desconocido` ni recheck | Requisito objetivo, no implementado | Endpoint/CLI/UI de preflight y pruebas negativas J3. |
| Registro local/manual | Wizard, detección y registro existen | Capacidad parcial | Paths/trust/escritura atómica y evidencia J4. |
| GitHub | El wizard muestra “Clone from GitHub”, pero exige una ruta local y no clona | **Decisión H4 pendiente**; 1.0.0 solo puede prometer registro de checkout existente hasta resolverla | Decisión PO; si se incluye clone, implementación y pruebas J5. |
| Trust | Se persiste un booleano enviado por cliente; no se autoriza en acciones | Requisito de seguridad, no operativo | NPROD-17 con trust server-side/fingerprint y pruebas fail-closed. |
| Start/Stop/Restart/Down | Acciones directas existen; `Down` no tiene confirmación reforzada | Capacidad parcial | Preflight/trust, confirmación y reconciliación J6. |
| Logs | Lectura, búsqueda, Live y copia cruda existen | Capacidad parcial | Ownership/TTL y redacción antes de render/copiar J7. |
| Recuperación guiada | Backups YAML y mensajes parciales; no hay flujo de recuperación integral | Requisito objetivo, no implementado | Failure injection y evidencia J8/NPROD-24/49. |

Mientras un gate esté abierto, el lenguaje público debe decir “objetivo”, “en evaluación” o “no soportado”; nunca presentar la capacidad como disponible por la sola aprobación de este artefacto.

## Principios del alcance 1.0.0

1. **Local-first y explicable:** cada paso debe indicar qué comprueba, qué cambia y dónde deja evidencia.
2. **Fail closed:** una comprobación desconocida no se presenta como éxito ni habilita una acción peligrosa.
3. **Recuperable:** todo fallo esperado ofrece causa probable, acción siguiente y forma de volver a verificar.
4. **Paridad de resultado, no de comandos:** macOS/Colima y Linux/Docker pueden tener pasos distintos, pero deben terminar en el mismo estado observable.
5. **Progresivo:** un proyecto manual o incompleto puede registrarse sin fingir que es ejecutable.
6. **Seguridad visible en el momento de riesgo:** clonar, confiar, ejecutar y bajar requieren una decisión comprensible en contexto.
7. **Sin conocimiento oculto:** la UI y CLI deben exponer el comando/diagnóstico necesario sin depender de memoria tribal.
8. **Accesible por teclado y tecnología asistiva:** el flujo crítico no depende solo de color, puntero preciso o animación.

## Matriz de personas

| ID | Arquetipo y contexto | Objetivo principal | Conocimiento asumido | Restricciones y riesgos | Señal de éxito |
| --- | --- | --- | --- | --- | --- |
| **P1** | Daniela, desarrolladora macOS con Apple Silicon y Colima | Levantar un checkout limpio sin administrar manualmente contextos/redes | Terminal, Git y conceptos básicos de contenedores | Colima detenido, contexto Docker incorrecto, arquitectura arm64, rutas bajo `/Users`, recursos limitados | `infra.localhost` abre, Docker/Colima aparece saludable y un proyecto inicia con URLs útiles. |
| **P2** | Luis, desarrollador Linux con Docker Engine/Compose | Usar el runtime existente sin instalar ni arrancar Colima | Docker CLI, permisos Linux y systemd básicos | Socket no accesible, grupo `docker`, rootless vs rootful, SELinux/AppArmor, UID/GID y rutas bajo `/home` | Preflight reconoce Docker Linux, no intenta Colima y opera sin permisos más amplios de los necesarios. |
| **P3** | Mei, desarrolladora que incorpora un repositorio GitHub existente | Clonar/registrar un repo, revisar lo detectado y habilitarlo con confianza explícita | Git y lectura de README; no conoce `projects.yml` | URL/rama incorrecta, repos privado, credenciales, código de terceros, compose hostil o ausente | El repo queda clonado en una raíz permitida, detectado, revisado y no se ejecuta hasta aprobar confianza. |
| **P4** | Andrés, desarrollador con proyecto manual o parcialmente configurado | Registrar contexto, URLs y faltantes sin forzar Docker Compose | Conoce la aplicación, no necesariamente la infraestructura completa | No hay compose, faltan variables/servicios, componente externo o ejecución manual | El proyecto aparece como `manual`/`external`/no listo, con acciones peligrosas deshabilitadas y pasos claros para progresar. |

### Necesidades compartidas

- Saber si el problema está en NearProd, Docker/runtime, configuración o el proyecto.
- Ver estado actual, última comprobación y acción siguiente sin inspeccionar logs crudos primero.
- Entender el alcance de `Start`, `Stop`, `Restart` y especialmente `Down` antes de confirmar.
- Copiar un diagnóstico redactado para soporte sin filtrar secretos.
- Cancelar o reintentar sin duplicar trabajos ni dejar estado ambiguo.

### No-personas para 1.0.0

Quedan fuera del diseño principal, sin impedir compatibilidad futura:

- operador de producción remota o multiusuario;
- administrador de un clúster Kubernetes;
- usuario Windows/WSL como plataforma aprobada;
- persona sin permisos para instalar/operar un runtime local;
- automatización CI/CD que requiera API remota estable.

## Definición de éxito de experiencia 1.0.0

Estas métricas son criterios de aceptación para pruebas moderadas o dogfood, no resultados ya demostrados:

| Resultado | Criterio de éxito propuesto | Evidencia mínima |
| --- | --- | --- |
| Instalación | 4/5 participantes por plataforma completan desde checkout limpio sin intervención del moderador fuera de prerrequisitos documentados | Grabación/notas, tiempos, comandos y estado final. |
| Primer arranque | Mediana ≤ 10 min después de prerrequisitos; 100% identifica runtime, URL del panel y resultado del preflight | Eventos o checklist con timestamps y captura del estado. |
| Incorporación local | 4/5 registran un proyecto representativo sin editar YAML manualmente | Payload/config diff, estado del proyecto y observaciones. |
| GitHub existente | 4/5 distinguen clonado, detección, revisión y confianza antes de ejecutar | Auditoría del flujo y respuesta a pregunta de comprensión. |
| Operación | 5/5 predicen correctamente el efecto de Start/Stop/Restart/Down antes de confirmar | Test de comprensión y estado Docker antes/después. |
| Diagnóstico | 4/5 encuentran la primera causa accionable y ejecutan/recomiendan el siguiente paso en ≤ 5 min | Diagnóstico exportado y verificación posterior. |
| Recuperación | Ningún caso esperado exige borrar estado a ciegas; 4/5 vuelven a un estado verificable sin soporte | Secuencia de recuperación y resultado de recheck. |
| Accesibilidad | Journeys críticos completables por teclado; nombres/estado/error anunciables; contraste y foco conformes al gate definido en NPROD-84 | Auditoría automatizada + manual de teclado/lector. |

**Hipótesis H1:** los umbrales anteriores son alcanzables y suficientemente exigentes para 1.0.0. **Propietario:** Product Owner. **Siguiente acción:** ejecutar cinco sesiones por plataforma o justificar una muestra menor antes de cerrar NPROD-84.

## Contrato común de los journeys

Cada journey usa esta evidencia:

- **Inicio:** plataforma, arquitectura, runtime/contexto, commit/versión, raíz de proyectos y estado relevante redactado.
- **Progreso:** preflight, acciones explícitas, job ID, timestamps y mensajes accionables.
- **Final:** estado NearProd + estado Docker/configuración + URL/archivo verificable según el resultado.
- **Privacidad:** tokens, cookies, `.env`, credenciales Git y secretos de logs nunca entran en capturas o reportes.

Un **error tolerable** conserva estado íntegro, explica la causa probable, ofrece una acción reversible y permite reintentar. Un **criterio de abandono** detiene el journey sin improvisar cuando continuar consumiría datos, permisos, seguridad o una decisión humana.

---

## J1 — Instalación desde checkout limpio

**Personas:** P1 y P2; P3/P4 como consumidoras posteriores.

**Precondiciones**

- macOS soportado con Colima instalable, o Linux soportado con Docker Engine y Compose plugin.
- Git, shell y permisos documentados; checkout limpio en una ruta permitida.
- Sin reutilizar `.env`, `projects.yml`, red o contenedores de una instalación anterior.

**Pasos críticos**

1. Leer requisitos detectados para la plataforma/arquitectura actual.
2. Ejecutar un preflight sin mutaciones que muestre versión/estado de Git, Docker, Compose, puertos, filesystem y permisos.
3. Revisar el plan exacto: archivos a crear, red, imágenes/builds, puertos y comandos.
4. Confirmar la instalación/bootstrap local.
5. Generar configuración desde ejemplos sin sobrescribir estado existente.
6. Ejecutar verificación final y mostrar cómo deshacer la instalación incompleta.

**Resultado esperado**

- NearProd queda instalado/configurado de forma reproducible.
- El runtime correcto está seleccionado; Linux no instala/inicia Colima.
- La salida contiene versión, rutas no secretas, verificaciones y siguiente comando.

**Errores tolerables**

- Dependencia ausente, puerto ocupado, Docker detenido, versión no soportada o permisos insuficientes: preflight falla antes de mutar y ofrece comandos específicos de diagnóstico.
- Descarga/build interrumpido: reintento idempotente sin reemplazar configuración humana.

**Criterios de abandono**

- Plataforma/arquitectura fuera de la matriz aprobada.
- Docker requiere privilegios no aceptados o el socket/contexto no puede verificarse.
- La ruta objetivo contiene estado real que sería sobrescrito.
- Se requiere egress y no está disponible; registrar artefacto faltante, no fingir instalación.

**Evidencia requerida**

- Preflight antes/después, versión instalada, archivos creados, runtime/contexto y prueba de rollback/no-overwrite.

**Hipótesis H2:** un único entry point puede ofrecer paridad de resultado con adaptadores macOS/Linux. **Propietario:** responsable de plataforma. **Siguiente acción:** spike/implementación de bootstrap y prueba en hosts limpios de ambas plataformas.

## J2 — Primer arranque

**Personas:** P1 y P2.

**Precondiciones**

- J1 completado; configuración válida y no contiene secretos expuestos.
- Runtime instalado; ningún estado previo de NearProd está en ejecución.

**Pasos críticos**

1. Ejecutar `dev-start` o entry point equivalente.
2. Detectar plataforma: iniciar/seleccionar Colima solo en macOS; verificar Docker existente en Linux.
3. Verificar/crear la red compartida sin reemplazar redes incompatibles silenciosamente.
4. Levantar Traefik y control panel; esperar readiness real, no solo proceso iniciado.
5. Mostrar URL canónica y estado de salud de cada dependencia.
6. Abrir/consultar el panel y confirmar que el estado coincide con Docker.

**Resultado esperado**

- `infra.localhost` responde desde loopback, UI y API están listas y el estado Docker es comprensible.
- Repetir el arranque es idempotente y no duplica recursos.

**Errores tolerables**

- Runtime todavía arrancando, build lento o health check transitorio: progreso visible, timeout finito y reintento seguro.
- Red existente compatible: reutilizar y reportar.

**Criterios de abandono**

- Puerto 80/8080 ocupado sin alternativa aprobada.
- Red existente incompatible, contexto Docker desconocido o health check no converge.
- La exposición deja de ser loopback o la frontera de seguridad no puede verificarse.

**Evidencia requerida**

- Contexto/runtime, recursos Compose, bind de puertos, health/readiness y HTTP de `infra.localhost`.

## J3 — Preflight fallido

**Personas:** P1, P2, P3 y P4.

**Precondiciones**

- Al menos una comprobación falla por fixture: Docker caído, versión, permisos, puerto, path, compose o configuración.

**Pasos críticos**

1. Mostrar resumen `aprobado / advertencia / bloqueado / desconocido` por comprobación.
2. Explicar la comprobación fallida y su impacto, ocultando secretos.
3. Ofrecer una acción siguiente específica de plataforma y una forma de copiar el diagnóstico.
4. No ejecutar la acción protegida mientras el bloqueo exista.
5. Repetir solo las comprobaciones afectadas o el preflight completo bajo solicitud.

**Resultado esperado**

- La persona identifica causa, dueño y siguiente acción; después del arreglo, el recheck demuestra el cambio.

**Errores tolerables**

- Comprobación no disponible: estado `desconocido`, diagnóstico y bloqueo conservador.
- Varias fallas: ordenar por dependencia/impacto en vez de mostrar un error genérico.

**Criterios de abandono**

- La remediación exige root, borrar datos, cambiar seguridad o adivinar una configuración.
- Dos reintentos reproducen la falla sin nueva evidencia; exportar diagnóstico y escalar.

**Evidencia requerida**

- Código/ID de comprobación, salida redactada, acción recomendada, estado antes/después y ruta de soporte.

**Hipótesis H3:** los usuarios prefieren remediación guiada en la interfaz frente a solo logs/comandos. **Propietario:** Product/UX. **Siguiente acción:** prueba comparativa de comprensión con tres fallas frecuentes por plataforma.

## J4 — Agregar proyecto local

**Personas:** P1, P2 y P4.

**Precondiciones**

- Sesión propietaria y frontera NPROD-15 implementadas.
- Ruta local dentro de una raíz permitida; repo/config permanece no confiable inicialmente.

**Pasos críticos**

1. Elegir “carpeta local” y seleccionar/ingresar una ruta.
2. Resolver la ruta de forma segura y detectar artefactos sin ejecutar código del proyecto.
3. Revisar tipo, runner, roles, URLs, identificadores y advertencias detectadas.
4. Editar datos explícitos sin perder la diferencia entre detectado y declarado.
5. Revisar un diff/plan de `projects.yml` y guardar atómicamente.
6. Mostrar el proyecto como no confiable/manual/no listo hasta completar el gate aplicable.

**Resultado esperado**

- Producto y componentes aparecen una vez, con estado honesto y sin ejecución automática.

**Errores tolerables**

- Stack ambiguo o compose ausente: permitir registro manual con advertencia y acciones deshabilitadas.
- Slug duplicado: proponer alternativa sin sobrescribir el existente.

**Criterios de abandono**

- Ruta inexistente, fuera de raíces, escape por symlink o ilegible.
- Configuración cambió durante la revisión o la escritura atómica no puede garantizarse.
- El guardado requiere reemplazar un producto sin confirmación específica.

**Evidencia requerida**

- Identidad de path/fingerprint, detección, diff aprobado, backup/commit atómico y dashboard resultante.

## J5 — Registrar un checkout GitHub existente; evaluar clonación integrada

**Personas:** P3; P1/P2 como variantes de plataforma.

**Estado de alcance**

El baseline no implementa clonación: la opción visual `github-clone` registra una ruta local y el backend no posee endpoint/operación Git. Por tanto, el compromiso mínimo de 1.0.0 es **registrar un checkout existente**. Los pasos de clonación descritos abajo solo entran al compromiso si el Product Owner aprueba H4 y existe implementación verificada; hasta entonces constituyen el diseño de la alternativa en evaluación.

**Precondiciones**

- Git disponible; raíz de clones permitida y escribible.
- Acceso público o credencial Git configurada fuera de NearProd; nunca se pega/persiste un token en la UI.
- NPROD-15/17/18 implementados para confianza y paths.

**Pasos críticos**

1. Para el compromiso mínimo, clonar externamente con Git configurado y seleccionar el checkout local sin entregar credenciales a NearProd.
2. Validar nombre destino/path, remote sanitizado, commit y colisiones.
3. Detectar y registrar el checkout sin ejecutar código.
4. Fijar identidad revisable: remote sanitizado, commit, path real y archivos detectados.
5. Registrar como no confiable; mostrar preflight y cambios ejecutables relevantes.
6. Obtener aprobación explícita de confianza antes de cualquier build/start/log privilegiado.
7. Solo si H4 se aprueba: permitir URL/referencia, ejecutar clone mediante transporte Git configurado y reportar progreso/limpieza sin capturar credenciales.

**Resultado esperado**

- Checkout existente íntegro y trazable, registro separado de ejecución y confianza ligada al fingerprint aprobado. La clonación integrada no forma parte del resultado esperado mientras H4 siga abierta.

**Errores tolerables**

- Repo privado sin credencial, referencia inexistente o red interrumpida: explicar carril fallido y permitir reintento/limpieza segura del clone incompleto.
- Proyecto sin compose: registrar manual si la persona lo decide.

**Criterios de abandono**

- URL contiene credenciales, host/protocolo no permitido o destino escapa la raíz.
- Destino no vacío, firma/política requerida falla o no puede fijarse el commit.
- La UI propone ejecutar automáticamente un repo recién clonado.

**Evidencia requerida**

- Remote sanitizado, commit, path real, resultado de clone/detección y registro de trust separado.

**Hipótesis H4:** NearProd 1.0.0 debe incluir clonación Git dentro del producto, no solo registro de un checkout clonado externamente. **Propietario:** Product Owner. **Siguiente acción:** aprobar inclusión y riesgo de credenciales o conservar el compromiso mínimo “registrar checkout existente”. Hasta decidir e implementar, la clonación se marca **en evaluación** y no como feature comprometida.

## J6 — Iniciar, detener, reiniciar y bajar un producto

**Personas:** P1, P2 y P3; P4 solo para componentes ejecutables.

**Precondiciones**

- Producto registrado, configuración válida y trust vigente en todos los componentes afectados.
- Preflight de Docker/path/compose aprobado; no existe otro job incompatible.
- **Gap actual:** el backend no aplica `trusted` ni un preflight formal y `ActionButtons` dispara `Down` sin confirmación. Por ello, el baseline no satisface estas precondiciones y no constituye evidencia de aceptación de J6.

**Pasos críticos**

1. Seleccionar producto o componente y una acción permitida.
2. Mostrar alcance: componentes, orden, comando semántico, efectos, recursos y riesgo.
3. Para `Down`, usar confirmación reforzada y distinguir claramente volúmenes/datos (no eliminarlos por defecto).
4. Ejecutar un job con timeout/cancelación; para grupos, completar preflight antes del primer side effect.
5. Mostrar progreso y estado final reconciliado con Docker.
6. Proponer siguiente acción ante fallo parcial sin reportar éxito global.

**Resultado esperado**

- `Start`: componentes esperados listos; `Stop`: detenidos sin eliminación; `Restart`: vuelven saludables; `Down`: recursos declarados se retiran dentro del alcance confirmado.

**Errores tolerables**

- Imagen/build tardío, servicio unhealthy o componente no runnable: job falla de forma explícita y conserva evidencia.
- Acción idempotente sobre estado ya alcanzado: informar “sin cambios” como éxito verificable.

**Criterios de abandono**

- Trust/preflight obsoleto, alcance cambiante o componentes ambiguos.
- Acción destructiva incluye volúmenes/datos no presentados.
- Timeout/cancelación no puede detener o reconciliar el job; marcar estado desconocido y bloquear acciones conflictivas.

**Evidencia requerida**

- Actor/sesión, fingerprint confiable, action plan, job ID, tiempos, exit code y estado Docker antes/después.

## J7 — Consultar logs

**Personas:** P1, P2, P3 y P4 cuando el runner lo permita.

**Precondiciones**

- Identidad autorizada, trust vigente, componente existente y runtime consultable.
- **Gap actual:** la UI copia `safeOutput` sin redacción y los jobs/logs no tienen ownership/TTL server-side. El flujo actual es una capacidad parcial, no evidencia de privacidad o autorización para J7.

**Pasos críticos**

1. Abrir logs desde el componente correcto y ver identidad/última actualización.
2. Cargar una ventana limitada con estados de carga, vacío y error diferenciados.
3. Buscar, navegar coincidencias, filtrar y activar/desactivar Live.
4. Pausar Live cuando el modal se cierra o el documento deja de estar activo.
5. Copiar/exportar salida redactada con contexto diagnóstico, sin secretos.
6. Enlazar la causa accionable con una comprobación o siguiente paso cuando sea posible.

**Resultado esperado**

- La persona encuentra una señal útil, sabe su antigüedad/componente y puede compartir evidencia segura.

**Errores tolerables**

- Sin contenedores/logs: estado vacío explicativo, no error 500 genérico.
- Docker temporalmente no disponible: conservar última lectura marcada como obsoleta y permitir recheck.

**Criterios de abandono**

- La salida excede límites, contiene material no redactable o pertenece a otro actor/proyecto.
- Live genera carga no acotada o no puede detenerse.

**Evidencia requerida**

- Componente, rango/tail, timestamp, consulta/filtro, redacción y diagnóstico seleccionado.

**Hipótesis H5:** actualización cada 2 segundos es aceptable para equipos y runtimes soportados. **Propietario:** ingeniería/UX. **Siguiente acción:** medir carga/legibilidad y definir backoff/pausa antes del gate de performance.

## J8 — Recuperar fallo de configuración o runtime

**Personas:** P1, P2, P3 y P4.

**Precondiciones**

- Fixture reproducible: YAML inválido, backup/escritura interrumpida, Docker caído, contexto equivocado, permisos, red/puerto o job timeout.

**Pasos críticos**

1. Detectar y clasificar la falla sin mutar más estado.
2. Mostrar último estado conocido bueno, estado actual y recursos afectados.
3. Ofrecer plan reversible: recheck, restaurar backup validado, corregir campo, reiniciar runtime o reconciliar job.
4. Mostrar diff/impacto antes de restaurar o cambiar configuración.
5. Ejecutar una acción a la vez y verificar desde la fuente autoritativa.
6. Cerrar con estado saludable, degradado explícito o escalamiento con diagnóstico redactado.

**Resultado esperado**

- El sistema vuelve a un estado verificable sin pérdida silenciosa; si no puede, queda bloqueado de forma segura con siguiente acción/propietario.

**Errores tolerables**

- Backup más reciente inválido: probar candidatos solo después de validarlos, sin sobrescribir el actual.
- Docker sigue caído tras restart sugerido: conservar configuración y escalar por plataforma.
- Job expirado/desconocido: reconciliar con Docker antes de permitir otra mutación.

**Criterios de abandono**

- La recuperación borraría volúmenes/repos/configuración sin plan y confirmación fuerte.
- No puede determinarse cuál estado es autoritativo.
- Requiere relajar permisos/seguridad, usar root indiscriminadamente o exponer secretos.

**Evidencia requerida**

- Código de falla, diagnóstico, diff/backup elegido, acción, verificación posterior y rollback/escalamiento.

**Hipótesis H6:** backups rotados de `projects.yml` bastan para recuperación 1.0.0. **Propietario:** ingeniería de datos/configuración. **Siguiente acción:** failure-injection de escritura interrumpida/concurrencia y decisión en NPROD-24/49.

---

## Diferencias legítimas por plataforma

| Área | macOS + Colima | Linux + Docker Engine | Invariante de experiencia |
| --- | --- | --- | --- |
| Runtime | NearProd puede iniciar/consultar Colima y seleccionar contexto `colima` | Usa Docker existente; no instala ni invoca Colima | Preflight identifica runtime/contexto exactos y no cambia uno desconocido. |
| Socket/conexión | Socket/contexto administrado por Colima, VM subyacente | `/var/run/docker.sock`, rootless socket u otro `DOCKER_HOST` | La conexión se descubre/configura; no se asume una ruta única ni se elevan permisos silenciosamente. |
| Filesystem | APFS, rutas `/Users`, montajes hacia VM, case sensitivity variable | ext4/NFS/otros, `/home`, UID/GID, SELinux/AppArmor | Paths permitidos, symlinks y permisos se validan en el punto de uso. |
| Arquitectura | arm64 común; imágenes x86 pueden emularse | x86_64/arm64 según host | Se reporta arquitectura/plataforma de imagen y fallo accionable antes de loops de build. |
| Servicios | Colima puede tardar/iniciar recursos configurables | Docker puede depender de systemd/rootless session | Progreso, timeout y comandos de diagnóstico son específicos; resultado final es Docker listo. |
| DNS/host | `.localhost` estándar; acceso desde contenedor/VM puede variar | `.localhost` estándar; `host.docker.internal` no siempre equivalente | URLs del usuario funcionan desde navegador host; conectividad interna se prueba por separado. |
| Permisos | No usar prompts de admin como flujo normal | Membresía `docker`, rootless, políticas MAC | Menor privilegio; si requiere cambio de seguridad, detener y explicar al propietario. |

No son diferencias legítimas: omitir seguridad en una plataforma, usar mensajes genéricos, esconder comandos, tener resultados distintos para las mismas acciones o degradar accesibilidad.

**Hipótesis H7:** Colima y Docker Engine son las únicas variantes obligatorias de 1.0.0; Docker Desktop/OrbStack quedan sujetos a NPROD-33. **Propietario:** Product Owner + plataforma. **Siguiente acción:** aprobar matriz de soporte antes de convertir nombres de runtime en promesas públicas.

## Alcance de experiencia 1.0.0

### Objetivo candidato 1.0.0, condicionado a implementación y gates

- Preflight y bootstrap diferenciados para macOS/Colima y Linux/Docker; Linux permanece no soportado hasta implementar y verificar la ruta no-Colima.
- Primer arranque idempotente con readiness y URL canónica.
- Registro seguro de carpetas locales y proyectos manuales/parciales.
- Dashboard honesto por producto/componente.
- Start/Stop/Restart/Down con alcance, progreso, resultado y confirmación proporcional.
- Logs limitados, buscables, pausables y exportables con redacción.
- Recuperación guiada de fallos soportados de configuración/runtime.
- Auth/Origin/CSRF/trust/path controls definidos por NPROD-15 y issues dependientes.
- Accesibilidad del flujo crítico conforme al gate NPROD-84.

### En evaluación — requiere decisión

- Clonación GitHub integrada (**H4**); el compromiso mínimo es registrar un checkout clonado externamente.
- TLS local y cookie `Secure`; NPROD-15 conserva el riesgo explícito de HTTP loopback.
- Docker Desktop y OrbStack como runtimes soportados (**H7**).

### Fuera de 1.0.0

- Operación remota/multiusuario, colaboración y RBAC múltiple.
- Windows/WSL aprobado, Kubernetes y runtimes no incluidos en NPROD-33.
- Deploy de producción, gestión de secretos de aplicaciones o credenciales Git.
- Eliminación de volúmenes/datos por defecto.
- Telemetría cloud/egress obligatorio y soporte autónomo basado en IA.
- Edición arbitraria de YAML sin schema/diff/rollback.

## Matriz criterio → artefacto → validación

| Criterio NPROD-80 | Cobertura | Validación propuesta |
| --- | --- | --- |
| Cuatro personas mínimas | P1–P4 | Revisión de matriz y trazabilidad de cada persona a journeys. |
| Ocho journeys requeridos | J1–J8 | Checklist automático de secciones + revisión de campos obligatorios. |
| Resultado, pasos, errores y abandono | Cada J1–J8 | Revisión estructural y escenarios de dogfood/fixtures. |
| Diferencias macOS/Linux | Tabla de plataforma | Pruebas en hosts reales; no aceptar solo mocks de plataforma. |
| Precondiciones y evidencia | Contrato común + cada journey | Registro de sesión sin secretos y estado antes/después. |
| Hipótesis explícitas | H1–H7 | Cada una tiene propietario y siguiente acción. |
| Aprobación PO | Gate siguiente | Checklist marcado y decisión en PR/Jira. |

## Hipótesis abiertas y siguiente acción

| ID | Hipótesis/decisión | Propietario | Evidencia o siguiente acción | Bloquea |
| --- | --- | --- | --- | --- |
| H1 | Umbrales de éxito son adecuados | Product Owner | Sesiones por plataforma y revisión de NPROD-84 | Cierre UX-01/UX-02/UX-03 |
| H2 | Un entry point da paridad de resultado | Plataforma | Hosts limpios macOS/Linux | Instalación 1.0.0 |
| H3 | Remediación guiada mejora comprensión | Product/UX | Prueba de tres fallas por plataforma | Diseño de preflight |
| H4 | Clonación Git integrada pertenece a 1.0.0 | Product Owner | Aprobar feature o limitar a checkout existente | Journey GitHub |
| H5 | Live logs cada 2 s es aceptable | Ingeniería/UX | Medición y backoff/pausa | Logs/performance |
| H6 | Backups YAML bastan para recuperación | Configuración/datos | Failure injection NPROD-24/49 | Recuperación |
| H7 | Colima + Docker Engine agotan soporte base | Product Owner/plataforma | NPROD-33 | Promesa de plataforma |

## Gate de aprobación del Product Owner resuelto

Decisiones confirmadas mediante autorización explícita de merge:

- [x] Apruebo P1–P4 como arquetipos de diseño 1.0.0, no como representación universal.
- [x] Apruebo J1–J8 y sus criterios de abandono como alcance crítico.
- [x] Apruebo los criterios de éxito propuestos o registro cambios medibles.
- [x] Decido H4: para 1.0.0 se compromete el registro de checkout existente; la clonación Git integrada permanece en evaluación hasta implementación aprobada.
- [x] Confirmo que H7 queda delegado a NPROD-33 antes de prometer soporte público.
- [x] Acepto que NPROD-84 y la implementación de seguridad son gates, no trabajo implícitamente completado por este documento.
- [x] Confirmo que Linux, preflight, trust, confirmación de `Down`, redacción de logs y recuperación no se comunicarán como soportados hasta aportar evidencia de implementación.
- [x] Las hipótesis restantes conservan propietario y siguiente acción aprobados.

**Condición de parada cumplida:** el Product Owner aprobó el mapa de personas/journeys y sus gates. Esta aprobación no declara implementadas ni soportadas las capacidades condicionadas; los issues dueños deben aportar evidencia antes de comunicarlas como parte operativa de 1.0.0.
