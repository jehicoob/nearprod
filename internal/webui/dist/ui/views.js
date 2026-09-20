import { api, humanBytes, message } from './api.js';
import { Modal, Icon, Alert, Skeleton, Field, LiveStatus } from './components.js';
const { useState, useEffect, useRef } = React;
export function DiscoverDialog({ roots, stacks, onClose, onSelect, onRoots }) {
    const [root, setRoot] = useState(roots[0] || ''), [depth, setDepth] = useState('8');
    const [candidates, setCandidates] = useState([]), [selected, setSelected] = useState([]), [error, setError] = useState(''), [busy, setBusy] = useState(false), [result, setResult] = useState('');
    const controller = useRef(null);
    useEffect(() => () => controller.current?.abort(), []);
    async function scan(e) {
        e.preventDefault();
        controller.current?.abort();
        controller.current = new AbortController();
        const signal = controller.current.signal;
        setBusy(true);
        setError('');
        setResult('');
        setCandidates([]);
        setSelected([]);
        try {
            const added = await api('/roots', { root }, signal);
            setRoot(added.root);
            onRoots();
            const response = await api('/discover', { root: added.root, depth: Number(depth) }, signal);
            setCandidates(response.candidates);
            setResult(`${response.candidates.length} aplicaciones posibles · ${response.entries} entradas revisadas${response.truncated ? ' · Límite alcanzado: usa una raíz más concreta.' : ''}${response.warnings.length ? ' · ' + response.warnings.join(' · ') : ''}`);
        }
        catch (e) {
            if (!signal.aborted)
                setError(message(e));
        }
        finally {
            if (controller.current?.signal === signal)
                setBusy(false);
        }
    }
    const selectable = candidates.filter(c => !stacks.some(s => s.path === c.path));
    return React.createElement(Modal, { title: "Descubrir aplicaciones", subtitle: "1. Busca en tu carpeta. 2. Selecciona las aplicaciones. 3. Revisa su configuraci\u00F3n y a\u00F1\u00E1delas a un grupo.", onClose: onClose, wide: true },
        React.createElement("form", { onSubmit: e => void scan(e) },
            React.createElement(Field, { label: "Carpeta ra\u00EDz", help: "Carpeta que contiene tus proyectos, por ejemplo ~/Projects. Se revisan subcarpetas; no se ejecuta ni se modifica ning\u00FAn repositorio." },
                React.createElement("input", { required: true, autoFocus: true, value: root, onChange: e => setRoot(e.target.value), list: "roots", placeholder: "~/Projects" })),
            React.createElement("datalist", { id: "roots" }, roots.map(r => React.createElement("option", { key: r, value: r }))),
            React.createElement("details", { className: "advanced compact-details" },
                React.createElement("summary", null, "Opciones de b\u00FAsqueda"),
                React.createElement(Field, { label: "Profundidad de b\u00FAsqueda", help: "Cu\u00E1ntos niveles de subcarpetas recorrer. 8 suele ser suficiente; 0 revisa solo la carpeta elegida. Se omiten dependencias, cach\u00E9s y enlaces a directorios." },
                    React.createElement("input", { type: "number", min: "0", max: "20", value: depth, onChange: e => setDepth(e.target.value) }))),
            React.createElement("div", { className: "inline-actions" },
                React.createElement("button", { className: "primary", disabled: busy },
                    React.createElement(Icon, { name: "search" }),
                    busy ? 'Buscando…' : 'Buscar aplicaciones'),
                busy && React.createElement("button", { type: "button", onClick: () => { controller.current?.abort(); setBusy(false); } }, "Cancelar b\u00FAsqueda"))),
        error && React.createElement(Alert, { error: true }, error),
        busy && React.createElement(Skeleton, { label: "Buscando aplicaciones en tus carpetas", rows: 5 }),
        result && React.createElement(React.Fragment, null,
            React.createElement("div", { className: "scan-result", role: "status" }, result),
            selectable.length > 0 && React.createElement("label", { className: "check-label" },
                React.createElement("input", { type: "checkbox", checked: selected.length === selectable.length && selectable.length > 0, onChange: e => setSelected(e.target.checked ? selectable.map(c => c.path).slice(0, 32) : []) }),
                "Seleccionar aplicaciones disponibles (m\u00E1ximo 32)")),
        React.createElement("div", { className: "candidate-list" }, candidates.map(c => {
            const known = stacks.find(s => s.path === c.path);
            return React.createElement("label", { className: `candidate selectable ${selected.includes(c.path) ? 'selected' : ''}`, key: c.path },
                React.createElement("input", { "aria-label": `Seleccionar ${c.relative}`, type: "checkbox", disabled: Boolean(known) || selected.length >= 32 && !selected.includes(c.path), checked: selected.includes(c.path), onChange: e => setSelected(e.target.checked ? [...selected, c.path] : selected.filter(v => v !== c.path)) }),
                React.createElement(Icon, { name: "folder", size: 22 }),
                React.createElement("span", { className: "candidate-description" },
                    React.createElement("strong", null, c.relative),
                    React.createElement("code", null, c.files.join(' · ')),
                    React.createElement("small", null, known ? `Ya registrada como ${known.id}. Configúrala desde Aplicaciones.` : c.ambiguous ? 'Hay varias bases o variantes: elige la combinación en el siguiente paso.' : 'Se propone el archivo base; puedes añadir variantes al revisar.')));
        })),
        result && !candidates.length && React.createElement(Alert, null, "No se encontraron archivos Compose. Comprueba la ra\u00EDz o aumenta la profundidad. Un Dockerfile sin Compose no define por s\u00ED solo c\u00F3mo levantar la aplicaci\u00F3n."),
        !result && !busy && React.createElement("div", { className: "empty small" },
            React.createElement(Icon, { name: "search", size: 30 }),
            React.createElement("p", null, "Selecciona, por ejemplo, el frontend y el backend de M\u00E1ximo Puntaje para registrarlos juntos. Cada aplicaci\u00F3n conserva su propio Compose.")),
        React.createElement("div", { className: "modal-actions" },
            React.createElement("span", { className: "selection-count" },
                selected.length,
                " seleccionadas"),
            React.createElement("button", { onClick: onClose }, "Cerrar"),
            React.createElement("button", { className: "primary", disabled: !selected.length || busy, onClick: () => onSelect(candidates.filter(c => selected.includes(c.path))) },
                "Configurar selecci\u00F3n ",
                React.createElement(Icon, { name: "chevron", size: 14 }))));
}
export function GroupDialog({ group, onClose, onSaved }) {
    const [name, setName] = useState(group?.name || ''), [busy, setBusy] = useState(false), [error, setError] = useState('');
    return React.createElement(Modal, { title: group ? 'Renombrar grupo' : 'Crear grupo', subtitle: "Un grupo organiza aplicaciones; no crea contenedores, redes ni dominios.", onClose: onClose },
        React.createElement("form", { onSubmit: e => { e.preventDefault(); setBusy(true); setError(''); void api(group ? '/groups/rename' : '/groups', { id: group?.id, name }).then(onSaved).catch(e => setError(message(e))).finally(() => setBusy(false)); } },
            React.createElement(Field, { label: "Nombre del grupo", help: group ? `Su identificador CLI (${group.id}) y las identidades Docker no cambian.` : 'Por ejemplo, Máximo Puntaje. Después puedes añadir aplicaciones desde Descubrir o mover las existentes desde Configurar.' },
                React.createElement("input", { required: true, maxLength: 120, autoFocus: true, value: name, onChange: e => setName(e.target.value), placeholder: "M\u00E1ximo Puntaje" })),
            error && React.createElement(Alert, { error: true }, error),
            React.createElement("div", { className: "modal-actions" },
                React.createElement("button", { type: "button", onClick: onClose }, "Cancelar"),
                React.createElement("button", { className: "primary", disabled: busy }, busy ? 'Guardando…' : 'Guardar grupo'))));
}
export function RuntimeView({ runtime, host, connected, operations, onSaved, notify, }) {
    const [doctor, setDoctor] = useState(null), [metrics, setMetrics] = useState(null);
    const [loadingDoctor, setLoadingDoctor] = useState(true), [loadingMetrics, setLoadingMetrics] = useState(true), [busy, setBusy] = useState(false), [error, setError] = useState("");
    const [settings, setSettings] = useState(runtime), [memory, setMemory] = useState("2"), [cpus, setCpus] = useState("2");
    const [preview, setPreview] = useState(null), [updates, setUpdates] = useState(null);
    const requests = useRef(null);
    const operationKey = operations
        .filter((o) => o.action.startsWith("runtime-"))
        .slice(0, 3)
        .map((o) => `${o.id}:${o.state}`)
        .join("|");
    const runtimeBusy = operations.some((o) => o.state === "running" && o.action.startsWith("runtime-"));
    const dirtySettings = JSON.stringify(settings) !== JSON.stringify(runtime);
    async function load() {
        requests.current?.abort();
        const ctrl = new AbortController();
        requests.current = ctrl;
        setLoadingDoctor(true);
        setLoadingMetrics(true);
        setError("");
        await Promise.allSettled([
            api("/doctor", undefined, ctrl.signal)
                .then((d) => {
                if (!ctrl.signal.aborted) {
                    setDoctor(d);
                    if (d.allocation) {
                        setMemory(String(d.allocation.memoryGiB));
                        setCpus(String(d.allocation.cpus));
                    }
                }
            })
                .catch((e) => {
                if (!ctrl.signal.aborted)
                    setError(message(e));
            })
                .finally(() => {
                if (!ctrl.signal.aborted)
                    setLoadingDoctor(false);
            }),
            api("/metrics", undefined, ctrl.signal)
                .then((m) => {
                if (!ctrl.signal.aborted)
                    setMetrics(m);
            })
                .catch((e) => {
                if (!ctrl.signal.aborted)
                    setError(message(e));
            })
                .finally(() => {
                if (!ctrl.signal.aborted)
                    setLoadingMetrics(false);
            }),
        ]);
    }
    useEffect(() => {
        const fallbackKind = host.runtime.supportedKinds[0] || runtime.kind;
        setSettings(host.runtime.supported
            ? runtime
            : {
                ...runtime,
                kind: fallbackKind,
                context: fallbackKind === "native" ? "default" : "colima",
                profile: "default",
            });
        void load();
        return () => requests.current?.abort();
    }, [runtime.kind, runtime.context, runtime.profile, connected, operationKey, host.runtime.supported]);
    async function task(work) {
        setBusy(true);
        setError("");
        try {
            await work();
        }
        catch (e) {
            setError(message(e));
        }
        finally {
            setBusy(false);
        }
    }
    const cannotChange = busy || runtimeBusy || loadingDoctor || dirtySettings;
    const managedVirtualMachine = host.runtime.managedVirtualMachine;
    const colimaState = doctor?.colima?.state;
    const colimaRunning = colimaState === "running";
    return (React.createElement("div", { className: "runtime-view" },
        React.createElement("div", { className: "section-heading" },
            React.createElement("div", null,
                React.createElement("h1", null, "Runtime y recursos"),
                React.createElement("p", null, managedVirtualMachine
                    ? "Colima mantiene una VM compartida. NearProd permanece fuera de ella."
                    : `NearProd usa Docker nativo en ${host.displayName}; no crea ni administra otra VM.`)),
            React.createElement("button", { onClick: () => void load(), disabled: busy || loadingDoctor || loadingMetrics, "aria-busy": loadingDoctor || loadingMetrics },
                React.createElement(Icon, { name: "refresh" }),
                "Actualizar diagn\u00F3stico")),
        React.createElement(LiveStatus, { message: loadingDoctor || loadingMetrics
                ? "Actualizando diagnóstico y consumo del runtime."
                : "" }),
        error && React.createElement(Alert, { error: true }, error),
        !host.runtime.supported && (React.createElement(Alert, { error: true },
            "El runtime guardado no est\u00E1 disponible en ",
            host.displayName,
            ". Guarda una opci\u00F3n soportada antes de operar Docker.")),
        dirtySettings && (React.createElement(Alert, null, "Hay cambios de contexto sin guardar. Gu\u00E1rdalos o desc\u00E1rtalos antes de administrar el runtime; los datos mostrados siguen correspondiendo al contexto guardado.")),
        runtimeBusy && (React.createElement(Alert, null, "Hay una operaci\u00F3n global del runtime en curso. Los controles quedan bloqueados hasta que termine; consulta Actividad.")),
        React.createElement("div", { className: "runtime-grid" },
            React.createElement("section", { className: "panel" },
                React.createElement("div", { className: "panel-heading" },
                    React.createElement(Icon, { name: "settings" }),
                    React.createElement("h2", null, "D\u00F3nde se ejecutan tus contenedores")),
                React.createElement("form", { onSubmit: (e) => {
                        e.preventDefault();
                        void task(async () => {
                            await api("/runtime/settings", settings);
                            onSaved();
                            notify("Contexto guardado. No se cambió el contexto global de Docker.");
                        });
                    } },
                    React.createElement(Field, { label: "Tipo de runtime", help: `Opciones soportadas por este binario en ${host.displayName}. No se infieren desde el navegador.` }, host.runtime.supportedKinds.length === 1 ? (React.createElement("input", { readOnly: true, value: settings.kind === "colima"
                            ? "Colima (macOS)"
                            : "Docker nativo" })) : (React.createElement("select", { disabled: runtimeBusy, value: settings.kind, onChange: (e) => setSettings({
                            ...settings,
                            kind: e.target.value,
                        }) }, host.runtime.supportedKinds.map((kind) => (React.createElement("option", { key: kind, value: kind }, kind === "colima" ? "Colima (macOS)" : "Docker nativo")))))),
                    React.createElement(Field, { label: "Contexto Docker", help: "Nombre de la conexi\u00F3n que utilizar\u00E1 NearProd. Puedes verlo con docker context ls; no cambies este valor para crear una VM nueva." },
                        React.createElement("input", { required: true, disabled: runtimeBusy, value: settings.context, onChange: (e) => setSettings({ ...settings, context: e.target.value }) })),
                    settings.kind === "colima" && (React.createElement(Field, { label: "Perfil Colima", help: "Identifica la VM existente; normalmente default. No es un perfil de servicios Compose." },
                        React.createElement("input", { required: true, disabled: runtimeBusy, value: settings.profile, onChange: (e) => setSettings({ ...settings, profile: e.target.value }) }))),
                    React.createElement("div", { className: "inline-actions" },
                        React.createElement("button", { disabled: busy || runtimeBusy || !dirtySettings }, "Guardar contexto"),
                        dirtySettings && (React.createElement("button", { type: "button", onClick: () => setSettings(runtime) }, "Descartar cambios"))),
                    React.createElement("p", { className: "hint" }, "Solo conexiones locales por socket Unix. Un cat\u00E1logo vinculado no se cambia a otro Engine silenciosamente."))),
            React.createElement("section", { className: "panel" },
                React.createElement("div", { className: "panel-heading" },
                    React.createElement(Icon, { name: "cube" }),
                    React.createElement("h2", null, managedVirtualMachine
                        ? "Máquina virtual de Colima"
                        : `Docker nativo en ${host.displayName}`)),
                !managedVirtualMachine ? (React.createElement(React.Fragment, null,
                    React.createElement("div", { className: "runtime-state", role: "status" },
                        React.createElement("span", { className: connected ? "live-dot" : "offline-dot" }),
                        React.createElement("strong", null, connected ? "Docker Engine conectado" : "Docker Engine no disponible")),
                    React.createElement("p", { className: "field-help" },
                        "Docker utiliza el kernel de ",
                        host.displayName,
                        ". La memoria y CPU globales se administran fuera de NearProd; no se ejecuta sudo ni se instala systemd autom\u00E1ticamente."))) : !doctor && loadingDoctor ? (React.createElement(Skeleton, { label: "Consultando estado y asignaci\u00F3n de Colima", rows: 6 })) : (React.createElement(React.Fragment, null,
                    React.createElement("div", { className: "runtime-state", role: "status" },
                        React.createElement("span", { className: colimaRunning ? "live-dot" : "offline-dot" }),
                        React.createElement("strong", null, runtimeBusy
                            ? "Operación en curso"
                            : colimaRunning
                                ? "Colima en ejecución"
                                : colimaState === "stopped"
                                    ? "Colima detenido"
                                    : colimaState === "missing"
                                        ? "Perfil no encontrado"
                                        : "Estado de Colima no comprobado")),
                    React.createElement("p", { className: "field-help" },
                        "El estado de la VM y la conexi\u00F3n con Docker Engine son comprobaciones distintas.",
                        " ",
                        colimaRunning && !connected
                            ? "La VM está activa, pero Docker no responde: revisa el diagnóstico, no vuelvas a iniciarla."
                            : ""),
                    colimaState === "stopped" && connected && (React.createElement(Alert, null, "Docker responde pero el diagn\u00F3stico del perfil indica que est\u00E1 detenido. Actualiza el diagn\u00F3stico antes de actuar.")),
                    React.createElement("div", { className: "metric-pair" },
                        React.createElement("div", null,
                            React.createElement("span", null, "RAM asignada a toda la VM"),
                            React.createElement("strong", null, doctor?.allocation
                                ? `${doctor.allocation.memoryGiB} GiB`
                                : "Sin datos")),
                        React.createElement("div", null,
                            React.createElement("span", null, "CPU virtuales"),
                            React.createElement("strong", null, doctor?.allocation?.cpus ?? "—"))),
                    React.createElement("p", { className: "hint" },
                        "VM: ",
                        doctor?.allocation?.vmType || "sin comprobar",
                        " \u00B7 montaje:",
                        " ",
                        doctor?.allocation?.mountType || "sin comprobar"),
                    React.createElement("div", { className: "form-grid" },
                        React.createElement(Field, { label: "RAM de la VM (GiB)", help: "L\u00EDmite compartido por Linux, Docker y todos los contenedores; no es una asignaci\u00F3n por aplicaci\u00F3n." },
                            React.createElement("input", { disabled: cannotChange, type: "number", step: "0.5", min: "0.5", value: memory, onChange: (e) => setMemory(e.target.value) })),
                        React.createElement(Field, { label: "CPU virtuales", help: "N\u00FAmero de CPU disponibles en la VM. Cambiar recursos puede requerir reiniciarla." },
                            React.createElement("input", { disabled: cannotChange, type: "number", min: "1", value: cpus, onChange: (e) => setCpus(e.target.value) }))),
                    React.createElement("div", { className: "inline-actions" },
                        React.createElement("button", { disabled: cannotChange ||
                                !doctor?.allocation ||
                                !["running", "stopped"].includes(colimaState || ""), onClick: () => void task(async () => setPreview(await api("/runtime/preview", {
                                memory: Number(memory),
                                cpus: Number(cpus),
                            }))) }, "Revisar cambio y alcance"),
                        colimaState === "stopped" && !connected && (React.createElement("button", { className: "primary", disabled: cannotChange, onClick: () => {
                                if (window.confirm("¿Iniciar el perfil Colima existente? No se creará otra VM ni se cambiará el contexto global."))
                                    void task(async () => {
                                        await api("/runtime/actions", {
                                            action: "start",
                                            confirm: true,
                                        });
                                        notify("Inicio de Colima solicitado. Consulta Actividad.");
                                    });
                            } },
                            React.createElement(Icon, { name: "play" }),
                            "Iniciar Colima")),
                        colimaRunning && !runtimeBusy && (React.createElement("span", { className: "runtime-ok" },
                            React.createElement(Icon, { name: "check", size: 16 }),
                            "Ya est\u00E1 iniciado"))),
                    React.createElement("p", { className: "hint" }, "La revisi\u00F3n muestra TODAS las cargas afectadas antes de aplicar un cambio; cerrar NearProd no detiene Colima."))))),
        React.createElement("section", { className: "panel" },
            React.createElement("div", { className: "panel-heading" },
                React.createElement(Icon, { name: "activity" }),
                React.createElement("h2", null, "Consumo observado")),
            !metrics && loadingMetrics ? (React.createElement(Skeleton, { label: "Cargando consumo de recursos", rows: 4 })) : metrics ? (React.createElement(React.Fragment, null,
                React.createElement("div", { className: "metrics" },
                    React.createElement("div", null,
                        React.createElement("span", null, "Memoria f\u00EDsica del host"),
                        React.createElement("strong", null, humanBytes(metrics.host.totalBytes))),
                    React.createElement("div", null,
                        React.createElement("span", null, "Proceso NearProd"),
                        React.createElement("strong", null, humanBytes(metrics.agent.rssBytes))),
                    managedVirtualMachine ? (React.createElement(React.Fragment, null,
                        React.createElement("div", null,
                            React.createElement("span", null, "Disponible en Linux invitado"),
                            React.createElement("strong", null, humanBytes(metrics.guest?.availableBytes))),
                        React.createElement("div", null,
                            React.createElement("span", null, "Swap usado en invitado"),
                            React.createElement("strong", null, metrics.guest
                                ? humanBytes(metrics.guest.swapTotalBytes -
                                    metrics.guest.swapFreeBytes)
                                : "Sin datos")))) : (React.createElement(React.Fragment, null,
                        React.createElement("div", null,
                            React.createElement("span", null, "Memoria visible para Docker"),
                            React.createElement("strong", null, humanBytes(metrics.engine?.memoryBytes))),
                        React.createElement("div", null,
                            React.createElement("span", null, "Contenedores activos observados"),
                            React.createElement("strong", null, metrics.containers.length))))),
                React.createElement("p", { className: "hint" },
                    metrics.note,
                    " ",
                    metrics.host.note),
                metrics.containers.length ? (React.createElement("div", { className: "table-wrap" },
                    React.createElement("table", null,
                        React.createElement("thead", null,
                            React.createElement("tr", null,
                                React.createElement("th", null, "Contenedor"),
                                React.createElement("th", null, "CPU"),
                                React.createElement("th", null, "RAM / l\u00EDmite"))),
                        React.createElement("tbody", null, metrics.containers.map((c) => (React.createElement("tr", { key: c.id },
                            React.createElement("td", null, c.name),
                            React.createElement("td", null, c.cpu),
                            React.createElement("td", null, c.memory)))))))) : (React.createElement("p", { className: "hint" }, connected
                    ? "No hay métricas de contenedores activos."
                    : "Docker no está conectado. Las métricas de contenedores no están disponibles.")))) : (React.createElement(Alert, null, "No fue posible cargar las m\u00E9tricas. Vuelve a consultar el diagn\u00F3stico."))),
        React.createElement("section", { className: "panel" },
            React.createElement("div", { className: "panel-heading" },
                React.createElement(Icon, { name: "terminal" }),
                React.createElement("h2", null, "Herramientas y versiones")),
            React.createElement("p", { className: "field-help" }, "Son componentes independientes del Engine. Docker puede responder aunque falte un plugin del cliente."),
            !doctor && loadingDoctor ? (React.createElement(Skeleton, { label: "Comprobando herramientas de Docker", rows: 4 })) : (doctor && (React.createElement(React.Fragment, null,
                React.createElement("p", { className: "hint" },
                    "NearProd ",
                    doctor.nearprod,
                    " \u00B7 Go ",
                    doctor.goVersion,
                    " \u00B7",
                    " ",
                    doctor.platform),
                React.createElement("div", { className: "tool-list" }, doctor.tools.filter((t) => t.supported).map((t) => (React.createElement("div", { key: t.name, className: "tool" },
                    React.createElement(Icon, { name: t.available ? "check" : "warning" }),
                    React.createElement("strong", null, t.name),
                    React.createElement("span", { className: "tool-status" }, t.available
                        ? "Disponible"
                        : t.status === "plugin-not-registered"
                            ? "Instalado, pero Docker no encuentra el plugin"
                            : "No disponible para NearProd"),
                    React.createElement("code", null, t.version),
                    React.createElement("p", { className: "field-help" },
                        t.purpose,
                        " ",
                        React.createElement("strong", null, t.required
                            ? "Requerido para su función."
                            : "No bloquea estado, logs ni detención.")),
                    !t.available && React.createElement("p", { className: "hint" }, t.hint))))),
                doctor.error && React.createElement(Alert, { error: true }, doctor.error.message),
                host.packageManagement.canInstall && doctor.tools.some((t) => !t.available && ["Compose", "Buildx"].includes(t.name)) && (React.createElement("details", { className: "advanced" },
                    React.createElement("summary", null, "C\u00F3mo reparar plugins instalados con Homebrew"),
                    React.createElement("p", null, "Primero comprueba si funcionan en tu terminal. Si all\u00ED funcionan pero NearProd no los detecta, reinicia el agente desde esa terminal para actualizar su entorno."),
                    React.createElement("pre", { className: "console" }, 'docker compose version\ndocker buildx version\n\n# Solo si faltan:\nbrew install docker-compose docker-buildx\n\n# Obtener la ruta de plugins:\necho "$(brew --prefix)/lib/docker/cli-plugins"'),
                    React.createElement("p", null,
                        "En ",
                        React.createElement("code", null, "~/.docker/config.json"),
                        " a\u00F1ade esa ruta absoluta a ",
                        React.createElement("code", null, "cliPluginsExtraDirs"),
                        " sin eliminar otras claves. En Apple Silicon suele ser",
                        " ",
                        React.createElement("code", null, "/opt/homebrew/lib/docker/cli-plugins"),
                        "."),
                    React.createElement("pre", { className: "console" }, '"cliPluginsExtraDirs": ["/opt/homebrew/lib/docker/cli-plugins"]'),
                    React.createElement("p", null,
                        "El fragmento es una propiedad JSON, no reemplaza el archivo entero. Despu\u00E9s ejecuta ",
                        React.createElement("code", null, "nearprod agent stop"),
                        " y",
                        " ",
                        React.createElement("code", null, "nearprod ui"),
                        "; no detienen tus contenedores.")))))),
            host.packageManagement.canCheckUpdates && (React.createElement(React.Fragment, null,
                React.createElement("button", { disabled: busy, onClick: () => void task(async () => setUpdates(await api("/updates", {}))) }, "Comprobar actualizaciones conocidas"),
                React.createElement("p", { className: "hint" },
                    "Consulta metadatos locales de ",
                    host.packageManagement.displayName,
                    ". No instala paquetes ni actualiza dependencias de proyectos."))),
            host.packageManagement.canCheckUpdates && updates && (React.createElement("div", { className: "update-results" },
                React.createElement("p", null, updates.message),
                updates.items.length > 0 ? (React.createElement("div", { className: "table-wrap" },
                    React.createElement("table", null,
                        React.createElement("thead", null,
                            React.createElement("tr", null,
                                React.createElement("th", null, "Herramienta"),
                                React.createElement("th", null, "Instalada"),
                                React.createElement("th", null, "Versi\u00F3n conocida"),
                                React.createElement("th", null, "Comando manual"))),
                        React.createElement("tbody", null, updates.items.map((i) => (React.createElement("tr", { key: i.name },
                            React.createElement("td", null, i.name),
                            React.createElement("td", null, i.installed.join(", ")),
                            React.createElement("td", null, i.latestKnown),
                            React.createElement("td", null,
                                React.createElement("code", null, i.command))))))))) : (React.createElement("p", { className: "hint" }, "No hay actualizaciones registradas en esta consulta; no confirma que los metadatos est\u00E9n al d\u00EDa."))))),
        preview && (React.createElement(Modal, { title: "Confirmar cambio global de Colima", subtitle: "La operaci\u00F3n afecta a toda la VM, no solo a NearProd.", onClose: () => setPreview(null) },
            React.createElement(Alert, { error: true }, preview.warning),
            React.createElement("p", null,
                "Asignaci\u00F3n solicitada:",
                " ",
                React.createElement("strong", null,
                    preview.memory,
                    " GiB \u00B7 ",
                    preview.cpus,
                    " CPU")),
            React.createElement("h3", null, "Contenedores activos afectados"),
            preview.affected.length ? (React.createElement("ul", { className: "hints" }, preview.affected.map((c) => (React.createElement("li", { key: c.id },
                c.name,
                " \u00B7 ",
                c.project || "fuera de Compose"))))) : (React.createElement("p", null, "No se observaron contenedores activos en esta revisi\u00F3n.")),
            React.createElement("div", { className: "modal-actions" },
                React.createElement("button", { onClick: () => setPreview(null) }, "Cancelar"),
                React.createElement("button", { className: "danger", disabled: busy || runtimeBusy, onClick: () => void task(async () => {
                        await api("/runtime/actions", {
                            ...preview,
                            action: "configure",
                            confirm: true,
                        });
                        setPreview(null);
                        notify("Cambio global solicitado. Consulta el resultado en Actividad.");
                    }) }, "Aplicar cambio global"))))));
}
export function LogsDrawer({ stack, observed, onClose }) {
    const [service, setService] = useState(''), [paused, setPaused] = useState(false), [lines, setLines] = useState([]), [state, setState] = useState('Conectando…'), [filter, setFilter] = useState('');
    const cursor = useRef(''), source = useRef(null), pre = useRef(null);
    useEffect(() => { cursor.current = ''; setLines([]); }, [stack.id, service]);
    useEffect(() => {
        if (paused) {
            setState('Pausado · seguimiento desconectado');
            return;
        }
        setState('Conectando…');
        const query = new URLSearchParams({ target: stack.id, follow: 'true', tail: '100' });
        if (service)
            query.set('service', service);
        if (cursor.current)
            query.set('since', cursor.current);
        const events = new EventSource(`/api/logs?${query}`);
        source.current = events;
        events.addEventListener('open', () => setState('En vivo'));
        events.addEventListener('line', (raw) => { const e = raw; const line = JSON.parse(e.data); if (e.lastEventId)
            cursor.current = e.lastEventId; setLines(prev => [...prev, { ...line, text: line.text.slice(-8000) }].slice(-400)); });
        events.addEventListener('log-error', (raw) => { setState(JSON.parse(raw.data).message); events.close(); });
        events.onerror = () => setState('Reconectando… Puede haber solapamientos o huecos.');
        return () => { events.close(); source.current = null; };
    }, [stack.id, service, paused]);
    useEffect(() => { if (!paused && pre.current)
        pre.current.scrollTop = pre.current.scrollHeight; }, [lines, paused]);
    const services = [...new Set(observed?.containers.map(c => c.service) || [])];
    return React.createElement("section", { className: "logs-drawer", "aria-label": "Logs en tiempo real" },
        React.createElement("div", { className: "logs-header" },
            React.createElement("div", null,
                React.createElement("span", { className: "eyebrow" }, "LOGS EN TIEMPO REAL"),
                React.createElement("h2", null, stack.id)),
            React.createElement("button", { className: "icon-button", "aria-label": "Cerrar logs", onClick: onClose },
                React.createElement(Icon, { name: "close" }))),
        React.createElement("div", { className: "logs-toolbar" },
            React.createElement("label", null,
                "Servicio",
                React.createElement("select", { "aria-describedby": "logs-service-help", value: service, onChange: e => setService(e.target.value) },
                    React.createElement("option", { value: "" }, "Todos (m\u00E1ximo 8 instancias)"),
                    services.map(s => React.createElement("option", { key: s, value: s }, s)))),
            React.createElement("label", null,
                "Filtro local",
                React.createElement("input", { "aria-describedby": "logs-filter-help", value: filter, onChange: e => setFilter(e.target.value), placeholder: "Filtrar salida\u2026" })),
            React.createElement("button", { onClick: () => setPaused(!paused) }, paused ? 'Reanudar' : 'Pausar'),
            React.createElement("button", { onClick: () => setLines([]) }, "Limpiar vista")),
        React.createElement("p", { className: "sr-only", id: "logs-service-help" }, "Selecciona de qu\u00E9 servicio leer logs. No cambia los contenedores."),
        React.createElement("p", { className: "sr-only", id: "logs-filter-help" }, "Filtra solo las l\u00EDneas visibles en este panel; no cambia la aplicaci\u00F3n ni el seguimiento."),
        React.createElement("div", { className: "log-status" },
            React.createElement("span", { className: !paused && state === 'En vivo' ? 'live-dot' : 'dot' }),
            state,
            React.createElement("span", null,
                "Buffer: ",
                lines.length,
                "/400 l\u00EDneas")),
        !lines.length && state === 'Conectando…' && React.createElement(Skeleton, { label: "Abriendo seguimiento de logs", rows: 4 }),
        React.createElement("pre", { ref: pre, className: "console logs-output" }, lines.filter(l => !filter || l.text.toLowerCase().includes(filter.toLowerCase())).map((l, i) => React.createElement("div", { className: l.stream === 'stderr' ? 'stderr' : '', key: `${i}-${l.time}` },
            React.createElement("span", { className: "log-service" }, l.service),
            " ",
            l.text))),
        React.createElement("p", { className: "hint" }, "Los logs pueden contener secretos. Se filtran algunos valores conocidos, no existe una garant\u00EDa de redacci\u00F3n completa. Cerrar esta vista libera el seguimiento, no detiene el contenedor."));
}
// Prevent accidental unused imports when this file is extended by the project.
