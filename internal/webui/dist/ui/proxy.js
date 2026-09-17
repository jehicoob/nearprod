import { suggestedHost } from './route-suggestions.js';
import { api, message } from './api.js';
import { Alert, Field, Icon, LiveStatus, Modal, Skeleton } from './components.js';
const { useState, useEffect } = React;
export function RouteEditor({ value, onChange, initialOptions }) {
    const [options, setOptions] = useState(initialOptions), [loading, setLoading] = useState(false), [error, setError] = useState('');
    const selected = JSON.stringify(value.modes.dev.files), routes = value.routes || [];
    useEffect(() => {
        if (!value.path)
            return;
        const controller = new AbortController();
        setLoading(true);
        void api('/project-options', { path: value.path, files: value.modes.dev.files }, controller.signal).then(v => { if (!controller.signal.aborted) {
            setOptions(v);
            setError('');
        } }).catch(e => { if (!controller.signal.aborted)
            setError(message(e)); }).finally(() => { if (!controller.signal.aborted)
            setLoading(false); });
        return () => controller.abort();
    }, [value.path, selected]);
    const update = (i, patch) => onChange({ ...value, routes: routes.map((r, j) => j === i ? { ...r, ...patch } : r) });
    const add = () => {
        const hint = options?.httpHints?.find(h => h.suggestedPort), service = hint?.service || options?.services[0] || '';
        onChange({ ...value, routes: [...routes, { host: suggestedHost(value, service, routes.length), service, port: hint?.suggestedPort || 80 }] });
    };
    return React.createElement("section", { className: "route-editor" },
        React.createElement("div", { className: "panel-heading" },
            React.createElement(Icon, { name: "link" }),
            React.createElement("h3", null, "URL de acceso \u2014 Traefik")),
        React.createElement(LiveStatus, { message: loading ? 'Leyendo servicios para la URL.' : '' }),
        React.createElement("p", null,
            "Configura aqu\u00ED el acceso de la aplicaci\u00F3n. NearProd preparar\u00E1 la ruta y conectar\u00E1 el servicio al proxy al ",
            React.createElement("strong", null, "Iniciar"),
            ", sin editar los Compose originales. Activa el proxy una vez desde ",
            React.createElement("strong", null, "Accesos locales"),
            "."),
        React.createElement("p", { className: "field-help" }, "Frontend y API pueden tener URLs distintas aunque est\u00E9n en el mismo Compose. Bases de datos y workers sin HTTP no necesitan URL. El proxy usa HTTP local; no instala certificados ni modifica DNS."),
        !options && loading && React.createElement(Skeleton, { label: "Detectando servicios para la URL", rows: 2 }),
        routes.map((route, i) => React.createElement("div", { className: "route-form", key: i },
            React.createElement(Field, { label: `Dominio local ${i + 1}`, help: "Nombre completo, sin http:// ni puerto. Ejemplo: proyecto.localhost; para una API, api-proyecto.localhost. Debe ser \u00FAnico en el cat\u00E1logo." },
                React.createElement("input", { required: true, value: route.host, onChange: e => update(i, { host: e.target.value.toLowerCase() }), placeholder: "maximopuntaje.localhost" })),
            React.createElement("div", { className: "form-grid" },
                React.createElement(Field, { label: `Servicio HTTP ${i + 1}`, help: "Servicio que responde a peticiones web. En PHP, selecciona Nginx/Apache, no PHP-FPM ni el worker." },
                    React.createElement("select", { required: true, value: route.service, onChange: e => { const hint = options?.httpHints?.find(v => v.service === e.target.value); update(i, { service: e.target.value, port: hint?.suggestedPort || route.port }); } },
                        React.createElement("option", { value: "" }, "Seleccionar servicio"),
                        [...new Set([...(options?.services || []), route.service].filter(Boolean))].map(s => React.createElement("option", { value: s, key: s }, s)))),
                React.createElement(Field, { label: `Puerto HTTP interno ${i + 1}`, help: "Puerto DENTRO del contenedor: por ejemplo 8000 para Uvicorn o 80 para Nginx. No es el puerto publicado en macOS. El servidor debe escuchar en 0.0.0.0." },
                    React.createElement("input", { type: "number", required: true, min: 1, max: 65535, value: route.port || '', onChange: e => update(i, { port: Number(e.target.value) }) }))),
            options?.httpHints?.find(v => v.service === route.service)?.note.includes('FPM') && React.createElement(Alert, { error: true }, "PHP-FPM habla FastCGI, no HTTP. Debes seleccionar un servicio web delante de FPM."),
            value.modes.verify && React.createElement("details", { className: "advanced" },
                React.createElement("summary", null, "Destino diferente en prueba de imagen"),
                React.createElement("p", { className: "field-help" }, "Por defecto se usa el mismo servicio y puerto. Si Vite usa 5173 en desarrollo y Nginx 80 al probar la imagen, configura aqu\u00ED ese destino."),
                React.createElement("label", { className: "check-label" },
                    React.createElement("input", { type: "checkbox", checked: Boolean(route.verify), onChange: e => update(i, { verify: e.target.checked ? { service: route.service, port: route.port } : undefined }) }),
                    "Cambiar destino para prueba de imagen"),
                route.verify && React.createElement("div", { className: "form-grid" },
                    React.createElement(Field, { label: `Servicio en prueba de imagen ${i + 1}`, help: "Nombre tal como aparece en los Compose de verificaci\u00F3n." },
                        React.createElement("input", { required: true, value: route.verify.service, onChange: e => update(i, { verify: { ...route.verify, service: e.target.value } }) })),
                    React.createElement(Field, { label: `Puerto en prueba de imagen ${i + 1}`, help: "Puerto HTTP interno de ese servicio, no el publicado en el host." },
                        React.createElement("input", { type: "number", min: 1, max: 65535, required: true, value: route.verify.port, onChange: e => update(i, { verify: { ...route.verify, port: Number(e.target.value) } }) })))),
            React.createElement("div", { className: "route-preview" },
                React.createElement("code", null,
                    "http://",
                    route.host),
                React.createElement("span", null,
                    "\u2192 ",
                    route.service || 'servicio',
                    ":",
                    route.port || '?'),
                React.createElement("button", { type: "button", onClick: () => onChange({ ...value, routes: routes.filter((_, j) => j !== i) }) }, "Quitar URL")))),
        !routes.length && React.createElement("p", { className: "hint" }, "Sin URL todav\u00EDa. A\u00F1\u00E1dela para una aplicaci\u00F3n web, o contin\u00FAa sin URL si es \u00FAnicamente DB/worker."),
        React.createElement("button", { type: "button", disabled: routes.length >= 8 || loading, onClick: add },
            React.createElement(Icon, { name: "plus", size: 14 }),
            routes.length ? 'Añadir otra URL' : 'Crear URL sugerida'),
        React.createElement("p", { className: "hint" }, "Las sugerencias no prueban que haya un servidor HTTP. Se conservan tus puertos publicados, redes, datos y variables. Si el proxy usa un puerto distinto de 80, NearProd lo a\u00F1adir\u00E1 al enlace."),
        error && React.createElement(Alert, { error: true }, error));
}
const routeState = { pending: 'Pendiente de Revisar e Iniciar', unknown: 'Estado sin comprobar', 'proxy-unavailable': 'Proxy no disponible', 'application-stopped': 'Servicio detenido', 'network-missing': 'Falta conexión de red · Iniciar de nuevo', 'ready-to-check': 'Conectado a la red · HTTP sin comprobar' };
const checkState = { 'pending': 'La ruta todavía no se aplicó', 'connection-failed': 'El puerto del proxy no responde', 'route-not-loaded': 'Traefik no cargó esta ruta', 'upstream-error': 'El servidor HTTP interno no responde', 'http-response': 'La ruta devolvió una respuesta HTTP' };
export function RouteLinks({ stack, observed, proxyPort = 80 }) {
    const [checking, setChecking] = useState(''), [checks, setChecks] = useState({}), [error, setError] = useState('');
    useEffect(() => { setChecks({}); setError(''); }, [stack.id, JSON.stringify(stack.routes), proxyPort]);
    async function check(host) { setChecking(host); setError(''); try {
        const result = await api('/proxy/check', { target: stack.id, host });
        setChecks(c => ({ ...c, [host]: result }));
    }
    catch (e) {
        setError(message(e));
    }
    finally {
        setChecking('');
    } }
    if (!stack.routes?.length)
        return null;
    return React.createElement("div", { className: "managed-routes" },
        React.createElement("strong", null,
            React.createElement(Icon, { name: "link", size: 15 }),
            "Acceso por Traefik"),
        stack.routes.map(route => {
            const observedRoute = observed?.routes?.find(r => r.host === route.host), result = checks[route.host], url = observedRoute?.url || `http://${route.host}${proxyPort === 80 ? '' : ':' + proxyPort}`;
            return React.createElement("div", { className: "managed-route", key: route.host },
                React.createElement("div", { className: "route-title" },
                    React.createElement("a", { href: url, target: "_blank", rel: "noopener noreferrer" },
                        url,
                        React.createElement(Icon, { name: "link", size: 12 })),
                    React.createElement("button", { disabled: Boolean(checking), onClick: () => void check(route.host) }, checking === route.host ? 'Comprobando…' : 'Comprobar acceso')),
                React.createElement("p", { className: "hint" },
                    routeState[observedRoute?.state || 'pending'],
                    " \u00B7 ",
                    observedRoute?.service || route.service,
                    ":",
                    observedRoute?.port || route.port),
                result && React.createElement("div", { className: "route-check", role: "status" },
                    React.createElement("strong", null,
                        checkState[result.state] || result.state,
                        result.response.status ? ` · HTTP ${result.response.status}` : ''),
                    React.createElement("span", null,
                        "Comprobado ",
                        new Date(result.checkedAt).toLocaleTimeString(),
                        ". No certifica la salud de la aplicaci\u00F3n."),
                    React.createElement("span", null,
                        "DNS del sistema: ",
                        result.dns.state === 'loopback' ? 'loopback (' + result.dns.addresses.join(', ') + ')' : result.dns.state === 'unresolved' ? 'no resuelto; el navegador puede tratar .localhost por separado' : 'no apunta exclusivamente a loopback; revisa el DNS'),
                    result.dns.state !== 'loopback' && React.createElement("details", null,
                        React.createElement("summary", null, "Alternativa cuando tu navegador no resuelve el dominio"),
                        React.createElement("p", null, "A\u00F1ade manualmente esta entrada a /etc/hosts solo despu\u00E9s de comprobar el nombre. NearProd no modifica ese archivo ni necesita sudo:"),
                        React.createElement("code", null, result.hostsEntry),
                        React.createElement("p", null, "No a\u00F1ade comodines; una entrada por nombre. Nunca uses una IP externa."))));
        }),
        error && React.createElement(Alert, { error: true }, error));
}
export function ProxyView({ catalog, status, notify, onSaved, onConfigure }) {
    const [port, setPort] = useState(catalog.proxy?.port || 80), [preview, setPreview] = useState(null), [busy, setBusy] = useState(false), [error, setError] = useState('');
    const proxy = status.proxy, active = catalog.operations.some(o => o.state === 'running'), stacks = catalog.stacks.filter(s => s.routes?.length);
    const titles = { 'not-created': 'Aún no creado', unknown: 'Estado sin comprobar', running: 'Traefik activo', starting: 'Iniciando o sin salud confirmada', unhealthy: 'No saludable', stopped: 'Traefik detenido', conflict: 'Conflicto de identidad' };
    async function task(f) { setBusy(true); setError(''); try {
        await f();
    }
    catch (e) {
        setError(message(e));
    }
    finally {
        setBusy(false);
    } }
    return React.createElement("div", { className: "runtime-view" },
        React.createElement("div", { className: "section-heading" },
            React.createElement("div", null,
                React.createElement("h1", null, "Accesos locales"),
                React.createElement("p", null, "Una URL por servicio web. Un solo Traefik compartido, en tu Colima existente.")),
            React.createElement("button", { disabled: busy, onClick: () => void task(async () => { await api('/status?refresh=1'); onSaved(); }) },
                React.createElement(Icon, { name: "refresh" }),
                "Actualizar estado")),
        React.createElement("section", { className: "panel" },
            React.createElement("div", { className: "panel-heading" },
                React.createElement(Icon, { name: "link" }),
                React.createElement("h2", null, proxy ? titles[proxy.state] || proxy.state : 'Comprobando proxy')),
            !proxy ? React.createElement(Skeleton, { label: "Consultando Traefik", rows: 3 }) : React.createElement(React.Fragment, null,
                React.createElement("p", null,
                    "NearProd administra ",
                    React.createElement("code", null, proxy.image),
                    " sin instalar Traefik en macOS. El panel sigue en el host; el proxy no crea otra VM."),
                React.createElement("div", { className: "form-grid" },
                    React.createElement(Field, { label: "Puerto de acceso local", help: "80 permite URLs sin puerto. Si otro proxy lo ocupa o no tienes permisos, usa 8080: los enlaces incluir\u00E1n :8080. No se detienen servicios ajenos." },
                        React.createElement("input", { type: "number", min: 1, max: 65535, value: port, onChange: e => setPort(Number(e.target.value)) })),
                    React.createElement("div", { className: "proxy-explainer" },
                        React.createElement("strong", null, "Dominios recomendados"),
                        React.createElement("code", null, "proyecto.localhost"),
                        React.createElement("code", null, "api-proyecto.localhost"),
                        React.createElement("span", null, "Accesos locales cortos: proyecto.localhost y api-proyecto.localhost."))),
                React.createElement("div", { className: "button-row" },
                    React.createElement("button", { className: "primary", disabled: busy || active || !status.connected, onClick: () => void task(async () => setPreview(await api('/proxy/preview', { port }))) }, proxy.state === 'running' ? 'Revisar cambio / reparar proxy' : 'Revisar activación de Traefik'),
                    React.createElement("button", { disabled: busy || active || !status.connected || !['running', 'starting', 'unhealthy'].includes(proxy.state), onClick: () => { if (window.confirm('¿Detener Traefik? Todas sus URLs dejarán de responder. Tus aplicaciones y datos se conservan.'))
                            void task(async () => { await api('/proxy/actions', { action: 'stop', confirm: true }); notify('Detención del proxy solicitada. Las aplicaciones no se detienen.'); }); } }, "Detener proxy"))),
            !status.connected && React.createElement(Alert, null, "Inicia el runtime desde Runtime y recursos. Registrar URLs no requiere Engine, pero activar Traefik s\u00ED."),
            React.createElement("p", { className: "hint" }, "HTTP local, enlace de escucha 127.0.0.1. Sin dashboard expuesto, sin socket Docker dentro del proxy, sin cambios en /etc/hosts, DNS o certificados. La red compartida solo se a\u00F1ade a los servicios HTTP elegidos; esos servicios pueden comunicarse entre s\u00ED."),
            error && React.createElement(Alert, { error: true }, error)),
        React.createElement("section", { className: "panel" },
            React.createElement("div", { className: "panel-heading" },
                React.createElement(Icon, { name: "grid" }),
                React.createElement("h2", null, "URLs de tus aplicaciones")),
            React.createElement("p", null,
                "Define la URL desde ",
                React.createElement("strong", null, "Configurar \u2192 URL de acceso"),
                ". Despu\u00E9s revisa, aprueba e inicia esa aplicaci\u00F3n para aplicar la conexi\u00F3n. Guardar una URL no recrea contenedores silenciosamente."),
            stacks.length ? stacks.map(s => React.createElement("div", { className: "proxy-stack", key: s.id },
                React.createElement("div", { className: "panel-heading" },
                    React.createElement("h3", null, s.name),
                    React.createElement("code", null, s.id),
                    React.createElement("button", { onClick: () => onConfigure(s) }, "Configurar URLs")),
                React.createElement(RouteLinks, { stack: s, observed: status.stacks.find(v => v.id === s.id), proxyPort: catalog.proxy?.port }))) : React.createElement(Alert, null, "A\u00FAn no hay URLs administradas. Configura una aplicaci\u00F3n web; DB/Redis/workers sin HTTP no necesitan una."),
            catalog.stacks.filter(s => !s.routes?.length).map(s => React.createElement("div", { className: "proxy-pending", key: s.id },
                React.createElement("span", null,
                    s.name,
                    React.createElement("small", null, s.id)),
                React.createElement("button", { onClick: () => onConfigure(s) }, "Configurar acceso")))),
        preview && React.createElement(Modal, { title: "Activar o actualizar Traefik", subtitle: "Cambio global del punto de acceso local, no de la VM.", onClose: () => setPreview(null) },
            React.createElement("p", null, preview.note),
            React.createElement("p", null,
                React.createElement("strong", null, preview.image),
                " \u00B7 127.0.0.1:",
                preview.port),
            React.createElement("p", null,
                preview.impacted.length,
                " aplicaciones con URL: ",
                preview.impacted.join(', ') || 'ninguna todavía',
                "."),
            React.createElement(Alert, null, "La primera activaci\u00F3n descarga la imagen. Activar/reparar recrea el proxy y puede interrumpir brevemente todas las URLs; cambiar el puerto tambi\u00E9n cambia los enlaces y obliga a actualizar URLs de API/CORS en tus aplicaciones; NearProd no modifica esas variables."),
            preview.blockers.map(b => React.createElement(Alert, { error: true, key: b }, b)),
            error && React.createElement(Alert, { error: true }, error),
            React.createElement("div", { className: "modal-actions" },
                React.createElement("button", { onClick: () => setPreview(null) }, "Cancelar"),
                React.createElement("button", { className: "primary", disabled: busy || active || Boolean(preview.blockers.length), onClick: () => void task(async () => { await api('/proxy/actions', { ...preview, action: 'start', confirm: true }); setPreview(null); notify('Traefik solicitado. Consulta Actividad y comprueba el estado antes de abrir las URLs.'); onSaved(); }) }, "Confirmar y activar Traefik"))));
}
