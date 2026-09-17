import { api, ApiError, humanBytes, message } from './api.js';
import { Alert, Badge, Busy, Icon, ImageDialog, Modal, OperationsView, ReviewDialog, Skeleton } from './components.js';
import { DiscoverDialog, RuntimeView, LogsDrawer, GroupDialog } from './views.js';
import { StackEditor, BatchEditor } from './editor.js';
import { ProxyView, RouteLinks } from './proxy.js';
import { InfrastructureView } from './infrastructure.js';
import { ToolsView } from './tools.js';
const { useState, useEffect } = React;
const EMPTY = { connected: false, checkedAt: null, stacks: [] };
function App() {
    const [catalog, setCatalog] = useState(null), [status, setStatus] = useState(EMPTY), [auth, setAuth] = useState(null), [code, setCode] = useState(''), [loginError, setLoginError] = useState('');
    const [page, setPage] = useState('projects'), [search, setSearch] = useState(''), [discover, setDiscover] = useState(false), [editor, setEditor] = useState(null);
    const [review, setReview] = useState(null), [logs, setLogs] = useState(null), [image, setImage] = useState(null), [mode, setMode] = useState({});
    const [notice, setNotice] = useState(null), [working, setWorking] = useState(false), [adoption, setAdoption] = useState(null);
    const [batch, setBatch] = useState(null), [groupEditor, setGroupEditor] = useState(null);
    const notify = (text, error = false) => setNotice({ text, error });
    async function load() {
        try {
            const c = await api('/catalog');
            setCatalog(c);
            setAuth(true);
            setStatus(await api('/status'));
        }
        catch (e) {
            if (e instanceof ApiError && e.code === 'AUTH_REQUIRED')
                setAuth(false);
            else {
                setAuth(false);
                setLoginError(message(e));
            }
        }
    }
    useEffect(() => { void load(); }, []);
    useEffect(() => {
        if (!auth)
            return;
        const events = new EventSource('/api/events');
        events.onmessage = (e) => {
            const event = JSON.parse(e.data);
            if (event.type === 'status' && event.status)
                setStatus(event.status);
            if (event.type === 'catalog')
                void api('/catalog').then(setCatalog).catch(e => notify(message(e), true));
            if (event.type === 'operation' && event.operation) {
                const op = event.operation;
                setCatalog(c => c ? { ...c, operations: [op, ...c.operations.filter(v => v.id !== op.id)].slice(0, 80) } : c);
                if (op.state === 'failed')
                    notify(`${op.action}: ${(Array.isArray(op.results) ? op.results : []).find(r => r.error)?.error?.message || op.error?.message || 'Operación fallida. Consulta Actividad.'}`, true);
                if (op.state === 'succeeded') {
                    notify(`${op.action}: operación completada. La salud de la aplicación se muestra por separado.`);
                    void api('/catalog').then(setCatalog).catch(() => { });
                }
            }
            if (event.type === 'operation-line' && event.line) {
                const line = event.line;
                setCatalog(c => c ? { ...c, operations: c.operations.map(v => v.id === event.id ? { ...v, lines: [...v.lines, line].slice(-100) } : v) } : c);
            }
            if (event.type === 'watch' && event.watch) {
                const watch = event.watch;
                setStatus(s => ({ ...s, stacks: s.stacks.map(v => v.id === event.id ? { ...v, watch } : v) }));
            }
        };
        events.onerror = () => { void api('/catalog').catch(e => { if (e instanceof ApiError && e.code === 'AUTH_REQUIRED')
            setAuth(false); }); };
        return () => events.close();
    }, [auth]);
    async function work(task) { setWorking(true); try {
        await task();
    }
    catch (e) {
        notify(message(e), true);
    }
    finally {
        setWorking(false);
    } }
    async function action(stack, action) {
        const selectedMode = mode[stack.id] || stack.activeMode || 'dev';
        if (['up', 'rebuild'].includes(action) && stack.routes?.length && !catalog?.proxy?.enabled) {
            setPage('proxy');
            notify('Activa Traefik una vez para este catálogo; después revisa e inicia la aplicación.');
            return;
        }
        if (['up', 'rebuild'].includes(action) && !stack.trust[selectedMode]) {
            setReview({ stack, mode: selectedMode });
            return;
        }
        const confirmMode = !stack.activeMode || stack.activeMode === selectedMode || window.confirm('Cambiar de modo recrea los servicios y conserva los mismos datos/volúmenes. ¿Continuar con el stack completo?');
        if (!confirmMode)
            return;
        if (action === 'rebuild' && !window.confirm('¿Construir y recrear este stack? Puede consumir CPU/RAM y tardar varios minutos. Los volúmenes se conservan.'))
            return;
        await work(async () => { await api('/actions', { target: stack.id, action, mode: selectedMode, confirmMode: true }); notify(`${action}: solicitud enviada. Consulta Actividad.`); });
    }
    async function groupAction(product, action) {
        if (action === 'up' && !catalog?.proxy?.enabled && catalog?.stacks.some(s => s.product === product && s.routes?.length)) {
            setPage('proxy');
            notify('Activa Traefik antes de iniciar este grupo con URLs.');
            return;
        }
        if (!window.confirm(`${action === 'up' ? 'Iniciar' : 'Detener'} las aplicaciones del grupo ${product}. Los stacks son independientes; no es una transacción y no se borran datos. ¿Continuar?`))
            return;
        await work(async () => { await api('/actions', { target: product, action }); notify('Operación de grupo enviada. Cada aplicación informa su resultado.'); });
    }
    const cancel = (id) => { if (window.confirm('Cancelar el cliente no garantiza que el Engine detenga el trabajo ya recibido. ¿Solicitar cancelación?'))
        void work(async () => { await api('/cancel', { id }); notify('Cancelación solicitada; revisa el estado real.'); }); };
    if (auth === null)
        return React.createElement("div", { className: "login-page" },
            React.createElement("div", { className: "login-card" },
                React.createElement("h2", null, "NearProd"),
                React.createElement(Skeleton, { label: "Abriendo tu entorno local", rows: 5 })));
    if (!auth)
        return React.createElement("div", { className: "login-page" },
            React.createElement("div", { className: "login-brand" },
                React.createElement("div", { className: "logo" },
                    React.createElement(Icon, { name: "cube", size: 30 })),
                React.createElement("span", null,
                    "NearProd",
                    React.createElement("span", { className: "brand-dot" }, "."))),
            React.createElement("section", { className: "login-card" },
                React.createElement("span", { className: "eyebrow" }, "TU ENTORNO. BAJO CONTROL."),
                React.createElement("h1", null, "Conecta tu consola local"),
                React.createElement("p", null,
                    "Abre una terminal y ejecuta ",
                    React.createElement("code", null, "nearprod ui"),
                    ". Pega el c\u00F3digo de acceso de un solo uso."),
                React.createElement("form", { onSubmit: e => { e.preventDefault(); setWorking(true); void api('/session', { code }).then(() => { setCode(''); return load(); }).catch(e => setLoginError(message(e))).finally(() => setWorking(false)); } },
                    React.createElement("label", null,
                        "C\u00F3digo de acceso",
                        React.createElement("input", { autoFocus: true, autoComplete: "off", required: true, value: code, onChange: e => setCode(e.target.value), placeholder: "C\u00F3digo de la terminal" })),
                    loginError && React.createElement(Alert, { error: true }, loginError),
                    React.createElement("button", { className: "primary full", disabled: working },
                        working ? 'Conectando…' : 'Abrir NearProd',
                        React.createElement(Icon, { name: "chevron" }))),
                React.createElement("div", { className: "login-footer" },
                    React.createElement("span", { className: "live-dot" }),
                    "Local \u00B7 Solo loopback \u00B7 Sin cuenta en la nube")),
            React.createElement("p", { className: "hint" }, "El panel funciona aunque Colima est\u00E9 apagado. No inicia tus contenedores autom\u00E1ticamente."));
    if (!catalog)
        return React.createElement(Busy, null);
    const running = status.stacks.filter(s => s.execution === 'running').length, totalContainers = status.stacks.reduce((n, s) => n + s.containers.length, 0), active = catalog.operations.filter(o => o.state === 'running');
    const filtered = catalog.stacks.filter(s => `${s.name} ${s.id} ${s.path} ${s.projectName} ${catalog.groups?.find(g => g.id === s.product)?.name || ""}`.toLowerCase().includes(search.toLowerCase()));
    const groups = [...new Set([...(catalog.groups || []).filter(g => !search || g.name.toLowerCase().includes(search.toLowerCase()) || filtered.some(s => s.product === g.id)).map(g => g.id), ...filtered.map(s => s.product)])];
    const groupName = (id) => catalog.groups?.find(g => g.id === id)?.name || id;
    const checked = Boolean(status.checkedAt);
    return React.createElement("div", { className: "shell" },
        React.createElement("aside", { className: "sidebar" },
            React.createElement("a", { className: "brand", href: "/", "aria-label": "NearProd inicio" },
                React.createElement("span", { className: "logo" },
                    React.createElement(Icon, { name: "cube", size: 24 })),
                "NearProd",
                React.createElement("span", { className: "brand-dot" }, ".")),
            React.createElement("span", { className: "version-label" },
                "LOCAL CONTROL / v",
                catalog.version),
            React.createElement("div", { className: "nav-caption" }, "WORKSPACE"),
            React.createElement("nav", { "aria-label": "Navegaci\u00F3n principal" },
                React.createElement("button", { className: page === 'projects' ? 'active' : '', onClick: () => setPage('projects') },
                    React.createElement(Icon, { name: "grid" }),
                    "Aplicaciones",
                    React.createElement("span", null, catalog.stacks.length)),
                React.createElement("button", { className: page === 'proxy' ? 'active' : '', onClick: () => setPage('proxy') },
                    React.createElement(Icon, { name: "link" }),
                    "Accesos locales"),
                React.createElement("button", { className: page === 'infra' ? 'active' : '', onClick: () => setPage('infra') },
                    React.createElement(Icon, { name: "cube" }),
                    "Infraestructura"),
                React.createElement("button", { className: page === 'tools' ? 'active' : '', onClick: () => setPage('tools') },
                    React.createElement(Icon, { name: "terminal" }),
                    "Herramientas"),
                React.createElement("button", { className: page === 'runtime' ? 'active' : '', onClick: () => setPage('runtime') },
                    React.createElement(Icon, { name: "settings" }),
                    "Runtime y recursos"),
                React.createElement("button", { className: page === 'activity' ? 'active' : '', onClick: () => setPage('activity') },
                    React.createElement(Icon, { name: "activity" }),
                    "Actividad",
                    active.length > 0 && React.createElement("span", null, active.length))),
            React.createElement("div", { className: "sidebar-bottom" },
                React.createElement("div", { className: "host-card" },
                    React.createElement(Icon, { name: "terminal" }),
                    React.createElement("div", null,
                        React.createElement("strong", null, "CONTROLADOR LOCAL"),
                        React.createElement("p", null, "Fuera de la VM"))),
                React.createElement("p", null,
                    "Compose es la fuente de verdad.",
                    React.createElement("br", null),
                    "NearProd organiza y opera."),
                React.createElement("button", { className: "text-button", onClick: () => void api('/logout', {}).then(() => setAuth(false)) }, "Cerrar sesi\u00F3n del panel"))),
        React.createElement("div", { className: "main" },
            React.createElement("header", { className: "topbar" },
                React.createElement("div", { className: "breadcrumb" },
                    "Workspace ",
                    React.createElement(Icon, { name: "chevron", size: 13 }),
                    React.createElement("strong", null, page === 'projects' ? 'Aplicaciones' : page === 'runtime' ? 'Runtime' : page === 'proxy' ? 'Accesos locales' : page === 'infra' ? 'Infraestructura' : page === 'tools' ? 'Herramientas' : 'Actividad')),
                React.createElement("div", { className: "topbar-status" },
                    React.createElement("span", { className: status.connected ? 'live-dot' : 'offline-dot' }),
                    !checked ? 'Comprobando Engine…' : status.connected ? 'Engine conectado' : 'Engine no disponible',
                    React.createElement("span", { className: "context-tag" }, catalog.runtime.context))),
            React.createElement("main", null,
                notice && React.createElement("div", { className: `toast ${notice.error ? 'error' : ''}`, role: notice.error ? 'alert' : 'status' },
                    React.createElement(Icon, { name: notice.error ? 'warning' : 'check' }),
                    React.createElement("span", null, notice.text),
                    React.createElement("button", { className: "icon-button", "aria-label": "Cerrar aviso", onClick: () => setNotice(null) },
                        React.createElement(Icon, { name: "close", size: 16 }))),
                page === 'projects' && React.createElement(React.Fragment, null,
                    React.createElement("div", { className: "section-heading hero" },
                        React.createElement("div", null,
                            React.createElement("span", { className: "eyebrow" }, "DE TU C\u00D3DIGO AL CONTENEDOR"),
                            React.createElement("h1", null,
                                "Tu entorno, en un solo lugar",
                                React.createElement("span", { className: "brand-dot" }, ".")),
                            React.createElement("p", null, "Descubre, ejecuta y comprueba tus aplicaciones sin perder de vista lo que est\u00E1 pasando.")),
                        React.createElement("button", { className: "primary", onClick: () => setDiscover(true) },
                            React.createElement(Icon, { name: "plus" }),
                            "Descubrir aplicaciones")),
                    React.createElement("div", { className: "overview" },
                        React.createElement("div", null,
                            React.createElement("span", null, "GRUPOS"),
                            React.createElement("strong", null,
                                catalog.groups.length,
                                React.createElement("small", null, "agrupaciones locales"))),
                        React.createElement("div", null,
                            React.createElement("span", null, "APLICACIONES EN EJECUCI\u00D3N"),
                            React.createElement("strong", null,
                                !checked ? React.createElement(Skeleton, { compact: true, label: "Comprobando aplicaciones", rows: 1 }) : status.connected ? running : '—',
                                React.createElement("small", null,
                                    "de ",
                                    catalog.stacks.length,
                                    " registrados"))),
                        React.createElement("div", null,
                            React.createElement("span", null, "CONTENEDORES OBSERVADOS"),
                            React.createElement("strong", null,
                                !checked ? React.createElement(Skeleton, { compact: true, label: "Contando contenedores", rows: 1 }) : status.connected ? totalContainers : '—',
                                React.createElement("small", null, status.connected ? 'incluye detenidos' : 'runtime sin conexión'))),
                        React.createElement("div", null,
                            React.createElement("span", null, "MEMORIA DEL ENGINE"),
                            React.createElement("strong", { className: "compact" },
                                !checked ? React.createElement(Skeleton, { compact: true, label: "Consultando memoria del Engine", rows: 1 }) : humanBytes(status.info?.memoryBytes),
                                React.createElement("small", null, "compartida por todas las cargas")))),
                    checked && !status.connected && React.createElement("div", { className: "runtime-banner" },
                        React.createElement(Icon, { name: "warning", size: 23 }),
                        React.createElement("div", null,
                            React.createElement("strong", null, "El controlador est\u00E1 disponible; el runtime, no."),
                            React.createElement("p", null,
                                status.error?.message || 'Consulta el diagnóstico y comprueba Colima.',
                                " Puedes descubrir y registrar proyectos sin encender la VM.")),
                        React.createElement("button", { onClick: () => setPage('runtime') },
                            "Revisar runtime ",
                            React.createElement(Icon, { name: "chevron", size: 14 }))),
                    React.createElement("div", { className: "project-toolbar" },
                        React.createElement("h2", null,
                            "Aplicaciones ",
                            React.createElement("span", { className: "count" }, catalog.stacks.length)),
                        React.createElement("div", null,
                            React.createElement("label", { className: "search" },
                                React.createElement(Icon, { name: "search" }),
                                React.createElement("input", { "aria-label": "Filtrar aplicaciones", value: search, onChange: e => setSearch(e.target.value), placeholder: "Grupo, aplicaci\u00F3n o ruta\u2026" })),
                            React.createElement("button", { className: "icon-button", "aria-label": "Actualizar estado", disabled: working, onClick: () => void work(async () => setStatus(await api('/status?refresh=1'))) },
                                React.createElement(Icon, { name: "refresh" })),
                            React.createElement("button", { onClick: () => setGroupEditor({}) },
                                React.createElement(Icon, { name: "folder", size: 14 }),
                                "Crear grupo"),
                            React.createElement("button", { onClick: () => setEditor({}) }, "Registrar manualmente"))),
                    !catalog.stacks.length && React.createElement("div", { className: "empty" },
                        React.createElement("div", { className: "empty-icon" },
                            React.createElement(Icon, { name: "folder", size: 34 })),
                        React.createElement("h2", null, "Empieza por tu carpeta de proyectos"),
                        React.createElement("p", null,
                            "NearProd detecta archivos Compose y te propone una agrupaci\u00F3n.",
                            React.createElement("br", null),
                            "Nada se construye ni se ejecuta sin tu intervenci\u00F3n."),
                        React.createElement("button", { className: "primary", onClick: () => setDiscover(true) },
                            React.createElement(Icon, { name: "search" }),
                            "Explorar mi carpeta"),
                        React.createElement("code", null, "nearprod init ~/Projects")),
                    catalog.stacks.length > 0 && !groups.length && React.createElement("div", { className: "empty small" },
                        React.createElement("p", null,
                            "No hay aplicaciones que coincidan con \u00AB",
                            search,
                            "\u00BB.")),
                    groups.map(product => React.createElement("section", { className: "product", key: product },
                        React.createElement("div", { className: "product-heading" },
                            React.createElement("span", { className: "product-icon" },
                                React.createElement(Icon, { name: "folder" })),
                            React.createElement("div", null,
                                React.createElement("h2", null, groupName(product)),
                                React.createElement("p", null,
                                    catalog.stacks.filter(s => s.product === product).length,
                                    " aplicaciones \u00B7 CLI: ",
                                    React.createElement("code", null, product))),
                            React.createElement("div", { className: "product-actions" },
                                React.createElement("button", { onClick: () => setGroupEditor({ group: { id: product, name: groupName(product) } }) }, "Renombrar grupo"),
                                React.createElement("button", { disabled: working || !status.connected || active.length > 0 || !catalog.stacks.some(s => s.product === product), onClick: () => void groupAction(product, 'up') }, "Iniciar grupo"),
                                React.createElement("button", { disabled: working || !status.connected || active.length > 0 || !catalog.stacks.some(s => s.product === product), onClick: () => void groupAction(product, 'stop') }, "Detener grupo"))),
                        React.createElement("div", { className: "stacks" },
                            !catalog.stacks.some(s => s.product === product) && React.createElement("div", { className: "empty small" },
                                React.createElement("p", null, "Grupo vac\u00EDo. Selecciona aplicaciones desde Descubrir y elige este grupo, o mueve una existente desde Configurar."),
                                React.createElement("button", { onClick: () => setDiscover(true) }, "A\u00F1adir aplicaciones")),
                            filtered.filter(s => s.product === product).map(s => {
                                const observed = status.stacks.find(v => v.id === s.id);
                                const selectedMode = mode[s.id] || s.activeMode || 'dev';
                                const isBusy = working || active.some(o => !o.targets.length || o.targets.includes(s.id));
                                return React.createElement("article", { className: "stack", key: s.id },
                                    React.createElement("div", { className: "stack-main" },
                                        React.createElement("div", { className: "stack-symbol" },
                                            React.createElement(Icon, { name: "cube" })),
                                        React.createElement("div", { className: "stack-description" },
                                            React.createElement("h3", null,
                                                s.name,
                                                React.createElement("span", { className: "mono" }, s.slug)),
                                            React.createElement("p", { className: "mono path", title: s.path }, s.path),
                                            React.createElement("div", { className: "stack-meta" },
                                                React.createElement("span", { className: "mono" }, s.projectName),
                                                React.createElement(Badge, { value: observed?.execution || 'unknown' }),
                                                observed?.execution === 'running' && React.createElement(Badge, { value: observed.health }))),
                                        React.createElement("div", { className: "stack-mode" },
                                            React.createElement("label", null,
                                                "Modo",
                                                React.createElement("select", { "aria-label": `Modo ${s.id}`, value: selectedMode, onChange: e => setMode({ ...mode, [s.id]: e.target.value }) },
                                                    React.createElement("option", { value: "dev" }, "Desarrollo"),
                                                    s.modes.verify && React.createElement("option", { value: "verify" }, "Prueba de imagen"))),
                                            React.createElement("span", { className: "hint" }, s.activeMode ? `Último solicitado: ${s.activeMode}` : 'Aún no iniciado aquí'))),
                                    React.createElement("div", { className: "stack-bottom" },
                                        React.createElement("div", { className: "stack-secondary" },
                                            React.createElement("button", { onClick: () => setReview({ stack: s, mode: selectedMode }) },
                                                React.createElement(Icon, { name: s.trust[selectedMode] ? 'check' : 'search', size: 14 }),
                                                s.trust[selectedMode] ? 'Revisar' : 'Revisar y aprobar'),
                                            React.createElement("button", { title: "Editar archivos, grupo y enlaces locales; no ejecuta Docker", onClick: () => setEditor({ stack: s }) }, "Configurar"),
                                            React.createElement("button", { disabled: !status.connected || isBusy, onClick: () => void work(async () => setAdoption(await api('/adoption', { target: s.id }))), title: "Vincula contenedores existentes de esta aplicaci\u00F3n tras comprobar su identidad" }, "Vincular existentes"),
                                            React.createElement("button", { title: "Solo elimina el registro; los contenedores siguen existiendo", disabled: isBusy, onClick: () => { if (window.confirm(`¿Quitar ${s.id} del catálogo? NO se detienen contenedores ni se borran datos.`))
                                                    void work(async () => { await api('/stacks/remove', { target: s.id, confirm: true }); await load(); notify('Se quitó únicamente el registro.'); }); } }, "Quitar")),
                                        React.createElement("div", { className: "stack-actions" },
                                            React.createElement("button", { disabled: !status.connected, onClick: () => setLogs(s) },
                                                React.createElement(Icon, { name: "terminal", size: 14 }),
                                                "Logs"),
                                            React.createElement("button", { disabled: !status.connected || isBusy, onClick: () => void action(s, 'restart'), title: "No aplica cambios del Compose ni variables" },
                                                React.createElement(Icon, { name: "refresh", size: 14 }),
                                                "Reiniciar"),
                                            React.createElement("button", { disabled: !status.connected || isBusy, onClick: () => void action(s, 'rebuild') }, "Construir"),
                                            React.createElement("button", { className: "start", disabled: !status.connected || isBusy, onClick: () => void action(s, 'up') },
                                                React.createElement(Icon, { name: "play", size: 13 }),
                                                "Iniciar"),
                                            React.createElement("button", { className: "stop", disabled: !status.connected || isBusy, onClick: () => void action(s, 'stop') },
                                                React.createElement(Icon, { name: "stop", size: 13 }),
                                                "Detener"))),
                                    React.createElement(RouteLinks, { stack: s, observed: observed, proxyPort: catalog.proxy?.port }),
                                    !!s.links?.length && React.createElement("div", { className: "app-links" },
                                        React.createElement("span", null, "Accesos definidos por ti \u00B7 sin comprobar conectividad"),
                                        s.links.map(l => React.createElement("a", { key: l.url, href: l.url, target: "_blank", rel: "noopener noreferrer" },
                                            l.label,
                                            React.createElement(Icon, { name: "link", size: 13 }),
                                            React.createElement("small", null, l.url)))),
                                    (observed?.containers.length || 0) > 0 && React.createElement("details", { className: "container-details" },
                                        React.createElement("summary", null,
                                            observed?.containers.length,
                                            " contenedores \u00B7 servicios, puertos y artefactos"),
                                        React.createElement("div", { className: "container-list" }, observed?.containers.map(c => React.createElement("div", { className: "container", key: c.id },
                                            React.createElement("span", { className: "mono" }, c.service),
                                            React.createElement(Badge, { value: c.state }),
                                            React.createElement(Badge, { value: c.health }),
                                            !c.owned && React.createElement("span", { className: "warning-text" }, "Sin adoptar"),
                                            c.oom && React.createElement("span", { className: "warning-text" }, "OOM"),
                                            React.createElement("span", { className: "mono muted" }, c.image),
                                            React.createElement("div", { className: "container-links" },
                                                c.ports.filter((p, i, a) => a.findIndex(v => v.port === p.port && v.container === p.container && v.host === p.host) === i).map(p => p.url ? React.createElement("a", { title: `Puerto publicado ${p.container}; protocolo web sugerido, sin comprobar conectividad`, key: `${p.container}-${p.host}-${p.port}`, href: p.url, target: "_blank", rel: "noreferrer noopener" },
                                                    ":",
                                                    p.port,
                                                    React.createElement(Icon, { name: "link", size: 12 })) : React.createElement("span", { className: "hint mono", title: "Puerto publicado; no se asume que sea una p\u00E1gina web", key: `${p.container}-${p.host}-${p.port}` },
                                                    p.host || "0.0.0.0",
                                                    ":",
                                                    p.port,
                                                    " \u2192 ",
                                                    p.container)),
                                                React.createElement("button", { onClick: () => setImage(c) }, "Imagen / plataforma")))))),
                                    React.createElement("div", { className: "watch-bar" },
                                        React.createElement("span", null, "Recarga: usa los montajes y el servidor dev del proyecto. Watch solo es necesario si el Compose declara develop.watch."),
                                        React.createElement("button", { disabled: !status.connected || isBusy || s.activeMode !== 'dev', onClick: () => void work(async () => { await api('/watch', { target: s.id, action: 'start' }); notify('Watch solicitado; requiere develop.watch y un Compose compatible.'); }) }, "Activar Watch"),
                                        observed?.watch.state !== 'off' && observed?.watch.state && React.createElement(React.Fragment, null,
                                            React.createElement("span", null, observed.watch.state),
                                            React.createElement("button", { onClick: () => void work(async () => { await api('/watch', { target: s.id, action: 'stop' }); }) }, "Detener Watch")),
                                        observed?.watch.error && React.createElement("span", { className: "warning-text" }, observed.watch.error.message)));
                            })))),
                    React.createElement("section", { className: "panel recent" },
                        React.createElement("div", { className: "panel-heading" },
                            React.createElement(Icon, { name: "activity" }),
                            React.createElement("h2", null, "Actividad reciente"),
                            React.createElement("button", { className: "text-button", onClick: () => setPage('activity') }, "Ver toda")),
                        React.createElement(OperationsView, { operations: catalog.operations.slice(0, 3), cancel: cancel }))),
                page === 'infra' && React.createElement(InfrastructureView, { catalog: catalog, status: status, notify: notify }),
                page === 'tools' && React.createElement(ToolsView, { operations: catalog.operations, notify: notify }),
                page === 'proxy' && React.createElement(ProxyView, { catalog: catalog, status: status, notify: notify, onSaved: () => void load(), onConfigure: s => setEditor({ stack: s }) }),
                page === 'runtime' && React.createElement(RuntimeView, { runtime: catalog.runtime, connected: status.connected, operations: catalog.operations, onSaved: () => void load(), notify: notify }),
                page === 'activity' && React.createElement(React.Fragment, null,
                    React.createElement("div", { className: "section-heading" },
                        React.createElement("div", null,
                            React.createElement("h1", null, "Actividad local"),
                            React.createElement("p", null, "Interfaz y CLI comparten operaciones, exclusi\u00F3n mutua e historial acotado.")),
                        React.createElement("span", { className: "count" },
                            catalog.operations.length,
                            " / 80")),
                    React.createElement(Alert, null, "Cancelar solicita detener el proceso cliente. No garantiza que Docker abandone una construcci\u00F3n ya enviada, ni revierte lo que alcanz\u00F3 a ejecutar."),
                    React.createElement(OperationsView, { operations: catalog.operations, cancel: cancel }))),
            React.createElement("footer", { className: "footer" },
                React.createElement("span", null,
                    "NearProd ",
                    catalog.version,
                    " \u00B7 controlador local"),
                React.createElement("span", null,
                    status.checkedAt ? `Comprobado: ${new Date(status.checkedAt).toLocaleTimeString()}` : 'Estado pendiente',
                    " \u00B7 Traefik ",
                    status.proxy?.state === 'running' ? 'activo' : 'sin conexión confirmada'))),
        discover && React.createElement(DiscoverDialog, { roots: catalog.roots, stacks: catalog.stacks, onClose: () => setDiscover(false), onSelect: cs => { setDiscover(false); setBatch(cs); }, onRoots: () => void load() }),
        editor && React.createElement(StackEditor, { ...editor, groups: catalog.groups, onClose: () => setEditor(null), onSaved: () => { setEditor(null); void load(); notify('Configuración guardada. No se ejecutó Docker.'); } }),
        batch && React.createElement(BatchEditor, { candidates: batch, groups: catalog.groups, onClose: () => setBatch(null), onSaved: () => { setBatch(null); void load(); notify('Aplicaciones registradas en el grupo. No se ejecutó Docker.'); } }),
        groupEditor && React.createElement(GroupDialog, { ...groupEditor, onClose: () => setGroupEditor(null), onSaved: () => { setGroupEditor(null); void load(); notify('Grupo guardado. No se modificó Docker.'); } }),
        review && React.createElement(ReviewDialog, { ...review, onClose: () => setReview(null), onApproved: () => { setReview(null); void load(); notify('Configuración aprobada. Ahora puedes iniciarla explícitamente.'); } }),
        logs && React.createElement(LogsDrawer, { key: logs.id, stack: logs, observed: status.stacks.find(s => s.id === logs.id), onClose: () => setLogs(null) }),
        image && React.createElement(ImageDialog, { container: image, onClose: () => setImage(null) }),
        adoption && React.createElement(Modal, { title: "Revisar adopci\u00F3n", subtitle: adoption.id, onClose: () => setAdoption(null) },
            React.createElement(Alert, { error: !adoption.allowed }, adoption.warning),
            !adoption.allowed && React.createElement(Alert, { error: true }, "Las rutas/propietarios no coinciden. No se adoptar\u00E1n estos recursos."),
            React.createElement("ul", null, adoption.containers.map(c => React.createElement("li", { key: c.id },
                c.name,
                " ",
                React.createElement("code", null, c.id.slice(0, 12))))),
            !adoption.containers.length && React.createElement("p", null, "No hay contenedores existentes; se vincular\u00E1 la identidad del Engine."),
            React.createElement("div", { className: "modal-actions" },
                React.createElement("button", { onClick: () => setAdoption(null) }, "Cancelar"),
                React.createElement("button", { className: "primary", disabled: !adoption.allowed || working, onClick: () => void work(async () => { await api('/adopt', { target: adoption.id, fingerprint: adoption.fingerprint, confirm: true }); setAdoption(null); await load(); notify('Identidad adoptada explícitamente.'); }) }, "Confirmar adopci\u00F3n"))));
}
ReactDOM.createRoot(document.getElementById('root')).render(React.createElement(App, null));
