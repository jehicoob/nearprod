import { api, message } from './api.js';
import { Alert, Field, Modal, Skeleton, Icon } from './components.js';
const { useState, useEffect } = React;
export function ToolsView({ notify, operations, }) {
    const [data, setData] = useState(null), [error, setError] = useState(""), [loading, setLoading] = useState(false), [selected, setSelected] = useState(null), [startup, setStartup] = useState(null);
    async function load() {
        setLoading(true);
        try {
            setData(await api("/tools"));
            setStartup(await api("/startup"));
            setError("");
        }
        catch (e) {
            setError(message(e));
        }
        finally {
            setLoading(false);
        }
    }
    useEffect(() => {
        void load();
    }, []);
    const active = operations.some((o) => o.state === "running");
    return (React.createElement(React.Fragment, null,
        React.createElement("div", { className: "section-heading" },
            React.createElement("div", null,
                React.createElement("span", { className: "eyebrow" }, "DEPENDENCIAS Y ARRANQUE"),
                React.createElement("h1", null, "Herramientas del equipo"),
                React.createElement("p", null, "Detectar primero. Conservar lo que ya funciona. Instalar solo con una revisi\u00F3n expl\u00EDcita.")),
            React.createElement("button", { disabled: loading, onClick: () => void load() },
                React.createElement(Icon, { name: "refresh" }),
                "Actualizar inventario")),
        error && React.createElement(Alert, { error: true }, error),
        !data && loading && (React.createElement(Skeleton, { label: "Detectando gestores y herramientas", rows: 7 })),
        " ",
        data && (React.createElement(React.Fragment, null,
            React.createElement(Alert, null, data.recommendation),
            React.createElement("section", { className: "panel" },
                React.createElement("h2", null, "Tu entorno detectado"),
                React.createElement("p", null,
                    "Agente nativo: ",
                    React.createElement("strong", null, data.binary.language),
                    " \u00B7 compilador: ",
                    React.createElement("strong", null, data.binary.version)),
                React.createElement("code", { className: "break-all" }, data.binary.path),
                React.createElement("p", { className: "hint" }, "NearProd no necesita Node, npm ni un gestor de versiones para ejecutarse. Los gestores detectados pertenecen a tus proyectos y se conservan sin cambios."),
                React.createElement("div", { className: "table-wrap" },
                    React.createElement("table", null,
                        React.createElement("thead", null,
                            React.createElement("tr", null,
                                React.createElement("th", null, "Gestor"),
                                React.createElement("th", null, "\u00C1mbito / evidencia"),
                                React.createElement("th", null, "Ruta"),
                                React.createElement("th", null, "Administraci\u00F3n"))),
                        React.createElement("tbody", null, data.managers.map((m, i) => (React.createElement("tr", { key: `${m.id}-${i}` },
                            React.createElement("td", null, m.id),
                            React.createElement("td", null,
                                {
                                    system: "Herramientas del sistema",
                                    node: "Node.js",
                                    runtimes: "Runtimes",
                                    "node-packages": "Paquetes Node",
                                }[m.scope] || m.scope,
                                " ",
                                "\u00B7",
                                " ",
                                {
                                    executable: "ejecutable encontrado",
                                    "shell-file": "archivo de shell",
                                    "pinned-runtime-path": "ruta del Node fijado",
                                }[m.evidence] || m.evidence),
                            React.createElement("td", { className: "mono break-all" }, m.path),
                            React.createElement("td", null, m.managed ? "Homebrew disponible" : "Solo detección")))))))),
            React.createElement("div", { className: "tool-cards" }, data.tools.filter((t) => t.supported).map((t) => (React.createElement("section", { className: "panel", key: t.id },
                React.createElement("div", { className: "panel-heading" },
                    React.createElement(Icon, { name: "terminal" }),
                    React.createElement("h2", null, t.name),
                    React.createElement("span", { className: `badge ${t.available ? "badge-healthy" : "badge-unknown"}` }, t.available
                        ? "Disponible"
                        : t.status === "plugin-not-registered"
                            ? "Plugin sin registrar"
                            : "No disponible")),
                React.createElement("p", null, t.purpose),
                React.createElement("p", { className: "mono break-all" },
                    t.version || "Versión sin comprobar",
                    React.createElement("br", null),
                    t.path || "No localizado"),
                React.createElement("small", null,
                    "Origen:",
                    " ",
                    {
                        existing: "Instalación existente",
                        homebrew: "Homebrew",
                        none: "No detectado",
                    }[t.provider] || t.provider,
                    " ",
                    "\u00B7",
                    " ",
                    t.required
                        ? "Necesario para operar"
                        : "Necesario al construir, no para leer logs"),
                data.packageManagement.canInstall && (React.createElement("div", { className: "modal-actions" },
                    React.createElement("button", { disabled: active, onClick: () => setSelected(t) }, "Versiones / instalar / reparar"))))))),
            React.createElement("section", { className: "panel" },
                React.createElement("h2", null, "Traefik \u00B7 componente central"),
                React.createElement("p", null,
                    "Traefik proporciona las URLs ",
                    React.createElement("code", null, "proyecto.localhost"),
                    ". Se aprovisiona dentro de Docker desde",
                    " ",
                    React.createElement("strong", null, "Accesos locales"),
                    ". Se administra como componente central, no como paquete del host ni como motor de datos opcional.")))),
        React.createElement(StoragePanel, null),
        React.createElement("section", { className: "panel" },
            React.createElement("h2", null, startup?.supported
                ? "Iniciar NearProd al entrar en macOS"
                : "Inicio automático"),
            startup ? (React.createElement("p", null, startup.message)) : (React.createElement(Skeleton, { label: "Comprobando inicio autom\u00E1tico", rows: 2 })),
            startup?.supported && (React.createElement(React.Fragment, null,
                React.createElement("pre", { className: "console" },
                    "nearprod startup enable",
                    React.createElement("br", null),
                    "nearprod startup status",
                    React.createElement("br", null),
                    "nearprod startup disable"),
                React.createElement("p", null, "Ejecuta estos comandos como tu usuario, sin sudo. Se inicia el agente despu\u00E9s del inicio de sesi\u00F3n, no antes de desbloquear el equipo. No abre el navegador ni enciende Colima, Traefik o proyectos autom\u00E1ticamente."),
                React.createElement("p", { className: "hint" }, "Deshabilitar impide pr\u00F3ximos arranques y conserva el agente actual. Para cerrarlo usa nearprod agent stop. El instalador nativo no depende de Node ni de fnm.")))),
        selected && (React.createElement(ToolDialog, { tool: selected, onClose: () => setSelected(null), onSubmitted: (id) => {
                setSelected(null);
                notify(`Operación ${id} iniciada. Consulta Actividad y vuelve a comprobar el inventario al terminar.`);
            } }))));
}
function ToolDialog({ tool, onClose, onSubmitted }) {
    const [versions, setVersions] = useState(null), [formula, setFormula] = useState(''), [action, setAction] = useState(tool.status === 'plugin-not-registered' ? 'repair' : 'install'), [preview, setPreview] = useState(null), [error, setError] = useState(''), [busy, setBusy] = useState(false);
    useEffect(() => {
        const controller = new AbortController();
        void api('/tools/versions', { tool: tool.id }, controller.signal).then(v => { setVersions(v); setFormula(v.options[0]?.formula || ''); }).catch(e => {
            if (!controller.signal.aborted)
                setError(message(e));
        });
        return () => controller.abort();
    }, [tool.id]);
    async function submit() {
        setBusy(true);
        try {
            if (!preview)
                setPreview(await api('/tools/preview', { tool: tool.id, formula, action }));
            else {
                const op = await api('/tools/actions', { tool: tool.id, formula, action, fingerprint: preview.fingerprint, confirm: true });
                onSubmitted(op.id);
            }
        }
        catch (e) {
            setError(message(e));
        }
        finally {
            setBusy(false);
        }
    }
    return React.createElement(Modal, { title: `Gestionar ${tool.name}`, subtitle: "Proveedor inicial: Homebrew macOS; no sudo ni actualizaci\u00F3n global", onClose: onClose, wide: true },
        error && React.createElement(Alert, { error: true }, error),
        !versions && !error && React.createElement(Skeleton, { label: "Consultando versiones realmente disponibles", rows: 5 }),
        " ",
        versions && React.createElement("form", { onSubmit: e => { e.preventDefault(); void submit(); } },
            React.createElement(Alert, null,
                versions.note,
                " Cancelar conserva la instalada. No se ofrece cualquier versi\u00F3n hist\u00F3rica."),
            React.createElement(Field, { label: "F\u00F3rmula / versi\u00F3n", help: "Se muestran \u00FAnicamente f\u00F3rmulas disponibles consultadas en Homebrew. Una versi\u00F3n fijada no se desfija autom\u00E1ticamente." },
                React.createElement("select", { disabled: !!preview, required: true, value: formula, onChange: e => setFormula(e.target.value) }, versions.options.map(o => React.createElement("option", { key: o.formula, value: o.formula },
                    o.formula,
                    " \u00B7 ",
                    o.version,
                    o.installed.length ? ' · instalada ' + o.installed.join(', ') : '',
                    o.pinned ? ' · FIJADA' : '')))),
            React.createElement(Field, { label: "Acci\u00F3n", help: "Reparar registra plugins sin reinstalar. Instalar/actualizar puede descargar dependencias y requerir red." },
                React.createElement("select", { disabled: !!preview, value: action, onChange: e => setAction(e.target.value) },
                    React.createElement("option", { value: "install" }, "Instalar la opci\u00F3n elegida"),
                    React.createElement("option", { value: "upgrade" }, "Actualizar la f\u00F3rmula elegida"),
                    ['compose', 'buildx'].includes(tool.id) && React.createElement("option", { value: "repair" }, "Reparar registro del plugin"))),
            preview && React.createElement(React.Fragment, null,
                preview.warnings.map((w, i) => React.createElement(Alert, { key: i }, w)),
                React.createElement("p", null, preview.command ? "Comando aprobado: " + preview.command.join(" ") : "Se añadirá el directorio de plugins a la configuración Docker, preservando el resto y guardando un backup cuando exista."),
                React.createElement("details", null,
                    React.createElement("summary", null, "Ruta, dependencias y detalles t\u00E9cnicos"),
                    React.createElement("pre", { className: "console review-json" }, JSON.stringify(preview, null, 2)))),
            React.createElement("div", { className: "modal-actions" },
                React.createElement("button", { type: "button", onClick: onClose }, "Conservar instalada / cerrar"),
                preview && React.createElement("button", { type: "button", onClick: () => setPreview(null) }, "Cambiar selecci\u00F3n"),
                React.createElement("button", { className: "primary", disabled: busy || !formula }, busy ? 'Consultando…' : preview ? 'Confirmar instalación / reparación' : 'Revisar cambios'))));
}
function StoragePanel() {
    const [paths, setPaths] = useState(null), [error, setError] = useState(''), [busy, setBusy] = useState(false);
    useEffect(() => { void api('/config/paths').then(setPaths).catch(e => setError(message(e))); }, []);
    async function backup() { setBusy(true); try {
        const v = await api('/config/backup', { confirm: true });
        setError('Backup privado guardado en ' + v.file);
    }
    catch (e) {
        setError(message(e));
    }
    finally {
        setBusy(false);
    } }
    return React.createElement("section", { className: "panel" },
        React.createElement("h2", null, "Configuraci\u00F3n y datos persistentes"),
        React.createElement("p", null, "Actualizar el ejecutable no borra el cat\u00E1logo. Tus proyectos, URLs y v\u00EDnculos se conservan. Los vol\u00FAmenes y carpetas existentes no se mueven."),
        !paths ? React.createElement(Skeleton, { label: "Consultando almacenamiento", rows: 3 }) : React.createElement("dl", null,
            React.createElement("dt", null, "Cat\u00E1logo de proyectos"),
            React.createElement("dd", { className: "mono break-all" }, paths.catalog),
            React.createElement("dt", null, "Carpeta para nuevas instancias de datos"),
            React.createElement("dd", { className: "mono break-all" }, paths.defaultDatabaseRoot),
            React.createElement("dt", null, "Backups de configuraci\u00F3n"),
            React.createElement("dd", { className: "mono break-all" }, paths.configurationBackups)),
        React.createElement("p", { className: "hint" }, "Una carpeta de datos corresponde a una instancia, no a cada base l\u00F3gica. El backup de configuraci\u00F3n contiene credenciales privadas, no los datos de los motores. No lo compartas."),
        React.createElement("button", { disabled: busy, onClick: () => void backup() }, busy ? 'Guardando…' : 'Guardar backup de configuración'),
        error && React.createElement("p", { role: "status", className: "break-all" }, error));
}
