import { api, message } from './api.js';
const { useState, useEffect, useRef } = React;
export function Icon({ name, size = 18 }) {
    const paths = {
        cube: React.createElement(React.Fragment, null,
            React.createElement("path", { d: "m12 3 9 5v8l-9 5-9-5V8z" }),
            React.createElement("path", { d: "m3 8 9 5 9-5M12 13v8M7.5 5.5l9 5" })),
        grid: React.createElement(React.Fragment, null,
            React.createElement("rect", { x: "3", y: "3", width: "7", height: "7", rx: "1.5" }),
            React.createElement("rect", { x: "14", y: "3", width: "7", height: "7", rx: "1.5" }),
            React.createElement("rect", { x: "3", y: "14", width: "7", height: "7", rx: "1.5" }),
            React.createElement("rect", { x: "14", y: "14", width: "7", height: "7", rx: "1.5" })),
        terminal: React.createElement(React.Fragment, null,
            React.createElement("rect", { x: "3", y: "4", width: "18", height: "16", rx: "3" }),
            React.createElement("path", { d: "m7 9 3 3-3 3m6 0h4" })),
        network: React.createElement(React.Fragment, null,
            React.createElement("circle", { cx: "12", cy: "5", r: "2.5" }),
            React.createElement("circle", { cx: "5", cy: "18", r: "2.5" }),
            React.createElement("circle", { cx: "19", cy: "18", r: "2.5" }),
            React.createElement("path", { d: "m10.5 7-4 8.5M13.5 7l4 8.5M7.5 18h9" })),
        folder: React.createElement("path", { d: "M3 7V5a2 2 0 0 1 2-2h5l2 3h7a2 2 0 0 1 2 2v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7z" }),
        settings: React.createElement(React.Fragment, null,
            React.createElement("path", { d: "M4 7h16M4 17h16" }),
            React.createElement("circle", { cx: "8", cy: "7", r: "3" }),
            React.createElement("circle", { cx: "16", cy: "17", r: "3" })),
        activity: React.createElement("path", { d: "M2 12h5l3-8 4 16 3-8h5" }),
        refresh: React.createElement(React.Fragment, null,
            React.createElement("path", { d: "M20 7a8 8 0 1 0 0 10M20 3v5h-5" })),
        search: React.createElement(React.Fragment, null,
            React.createElement("circle", { cx: "10", cy: "10", r: "6" }),
            React.createElement("path", { d: "m15 15 6 6" })),
        plus: React.createElement("path", { d: "M12 4v16M4 12h16" }), close: React.createElement("path", { d: "m6 6 12 12M18 6 6 18" }),
        play: React.createElement("path", { d: "m7 4 14 8-14 8z" }), stop: React.createElement("rect", { x: "6", y: "6", width: "12", height: "12", rx: "2" }),
        check: React.createElement("path", { d: "m4 12 5 5L20 6" }), warning: React.createElement(React.Fragment, null,
            React.createElement("path", { d: "m12 3 10 18H2zM12 9v5m0 3v.5" })),
        link: React.createElement(React.Fragment, null,
            React.createElement("path", { d: "M14 3h7v7m0-7L10 14" }),
            React.createElement("path", { d: "M10 4H4v16h16v-6" })),
        chevron: React.createElement("path", { d: "m8 4 8 8-8 8" }), clock: React.createElement(React.Fragment, null,
            React.createElement("circle", { cx: "12", cy: "12", r: "9" }),
            React.createElement("path", { d: "M12 6v6l4 2" })),
    };
    return React.createElement("svg", { "aria-hidden": "true", width: size, height: size, viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", strokeWidth: "1.6", strokeLinecap: "round", strokeLinejoin: "round" }, paths[name] || paths.cube);
}
export function Badge({ value }) {
    const labels = { running: 'En ejecución', healthy: 'Saludable', unhealthy: 'No saludable', unchecked: 'Salud sin comprobar', stopped: 'Detenido', 'not-created': 'No creado', archived: 'Archivada', unknown: 'Sin conexión', failed: 'Falló', succeeded: 'Completada', cancelled: 'Cancelada', interrupted: 'Interrumpida', partial: 'Parcial', restarting: 'Reiniciando', starting: 'Iniciando', paused: 'Pausado', off: 'Apagado', dev: 'Desarrollo', verify: 'Prueba de imagen' };
    return React.createElement("span", { className: `badge badge-${value}` },
        React.createElement("span", { className: "dot" }),
        labels[value] || value);
}
export function Modal({ title, subtitle, children, onClose, wide = false }) {
    const ref = useRef(null);
    useEffect(() => { ref.current?.showModal(); }, []);
    return React.createElement("dialog", { ref: ref, className: wide ? 'modal wide' : 'modal', onCancel: e => { e.preventDefault(); onClose(); }, onClick: e => { if (e.target === ref.current) {
            const r = ref.current.getBoundingClientRect();
            if (e.clientX < r.left || e.clientX > r.right || e.clientY < r.top || e.clientY > r.bottom)
                onClose();
        } }, "aria-label": title },
        React.createElement("div", { className: "modal-heading" },
            React.createElement("div", null,
                React.createElement("h2", null, title),
                subtitle && React.createElement("p", null, subtitle)),
            React.createElement("button", { className: "icon-button", "aria-label": "Cerrar di\u00E1logo", onClick: onClose },
                React.createElement(Icon, { name: "close" }))),
        children);
}
export function Alert({ children, error = false, warning = false }) { return React.createElement("div", { className: `alert ${error ? 'error' : warning ? 'warning' : ''}`, role: error || warning ? 'alert' : 'note' },
    React.createElement(Icon, { name: error || warning ? 'warning' : 'terminal' }),
    React.createElement("div", null, children)); }
export function Busy() { return React.createElement("div", { className: "busy" },
    React.createElement("span", { className: "spinner" }),
    " Consultando\u2026"); }
export function LiveStatus({ message }) {
    return React.createElement("span", { className: "sr-only", role: "status", "aria-live": "polite", "aria-atomic": "true" }, message);
}
export function OperationsView({ operations, cancel }) {
    if (!operations.length)
        return React.createElement("div", { className: "empty small" },
            React.createElement(Icon, { name: "activity", size: 30 }),
            React.createElement("h3", null, "La actividad aparecer\u00E1 aqu\u00ED"),
            React.createElement("p", null, "Operaciones de la interfaz y del CLI, en un mismo lugar."));
    return React.createElement("div", { className: "operation-list" }, operations.map(op => React.createElement("details", { className: "operation", key: op.id },
        React.createElement("summary", null,
            React.createElement("span", { className: "operation-title" },
                React.createElement(Icon, { name: "terminal" }),
                React.createElement("strong", null, op.action),
                React.createElement("span", { className: "mono muted" }, op.targets.join(', ') || 'runtime')),
            React.createElement("span", { className: "operation-meta" },
                React.createElement("time", null, new Date(op.startedAt).toLocaleTimeString()),
                React.createElement(Badge, { value: op.state }))),
        React.createElement("div", { className: "operation-details" },
            React.createElement("p", { className: "mono muted" },
                "ID: ",
                op.id),
            op.error && React.createElement(Alert, { error: true }, op.error.message),
            (Array.isArray(op.results) ? op.results : []).filter(r => r.error).map(r => React.createElement(Alert, { key: r.id, error: true },
                React.createElement("strong", null, r.id),
                ": ",
                r.error?.message)),
            React.createElement("pre", { className: "console" }, op.lines.map(l => l.text).join('\n') || 'Sin salida de comandos.'),
            op.state === 'succeeded' && op.results && React.createElement("details", null,
                React.createElement("summary", null, "Resultado y rutas generadas"),
                React.createElement("pre", { className: "console review-json" }, JSON.stringify(op.results, null, 2).slice(0, 65536))),
            op.state === 'running' && React.createElement("button", { className: "danger", onClick: () => cancel(op.id) }, "Solicitar cancelaci\u00F3n")))));
}
export function ReviewDialog({ stack, mode, onClose, onApproved }) {
    const [preview, setPreview] = useState(null), [error, setError] = useState(''), [unsafe, setUnsafe] = useState(false), [busy, setBusy] = useState(false);
    useEffect(() => { const controller = new AbortController(); void api('/preview', { target: stack.id, mode }, controller.signal).then(setPreview).catch(e => { if (!controller.signal.aborted)
        setError(message(e)); }); return () => controller.abort(); }, [stack.id, mode]);
    async function approve() { if (!preview)
        return; setBusy(true); try {
        await api('/trust', { target: stack.id, mode, fingerprint: preview.fingerprint, allowUnsafe: unsafe });
        onApproved();
    }
    catch (e) {
        setError(message(e));
    }
    finally {
        setBusy(false);
    } }
    return React.createElement(Modal, { title: "Revisar configuraci\u00F3n", subtitle: `${stack.id} · ${mode === 'dev' ? 'Desarrollo' : 'Verificación'}`, onClose: onClose, wide: true },
        !preview && !error && React.createElement(Skeleton, { label: "Preparando revisi\u00F3n de Compose", rows: 6 }),
        error && React.createElement(Alert, { error: true }, error),
        preview && React.createElement(React.Fragment, null,
            React.createElement("div", { className: "review-summary" },
                React.createElement(Badge, { value: mode }),
                React.createElement("span", null,
                    preview.services.length,
                    " servicios"),
                React.createElement("span", null, preview.approved ? 'Revisión vigente' : 'Requiere aprobación')),
            React.createElement("h3", null, "Archivos en orden"),
            React.createElement("ol", { className: "file-list" }, preview.files.map(f => React.createElement("li", { key: f, className: "mono" }, f))),
            React.createElement("div", { className: "table-wrap" },
                React.createElement("table", null,
                    React.createElement("thead", null,
                        React.createElement("tr", null,
                            React.createElement("th", null, "Servicio"),
                            React.createElement("th", null, "Imagen / construcci\u00F3n"),
                            React.createElement("th", null, "Salud"),
                            React.createElement("th", null, "Plataforma"))),
                    React.createElement("tbody", null, preview.services.map(s => React.createElement("tr", { key: s.name },
                        React.createElement("td", null, s.name),
                        React.createElement("td", { className: "mono" }, s.image || (s.build ? 'Dockerfile local' : 'Sin imagen')),
                        React.createElement("td", null, s.hasHealthcheck ? 'Healthcheck' : 'Sin comprobar'),
                        React.createElement("td", null, s.platform || 'No fijada')))))),
            preview.blockers.map((v, i) => React.createElement(Alert, { error: true, key: i }, v)),
            preview.risks.length > 0 && React.createElement("div", { className: "risk-box" },
                React.createElement("h3", null, "Capacidades que debes revisar"),
                React.createElement("ul", null, preview.risks.map((v, i) => React.createElement("li", { key: i }, v))),
                React.createElement("label", { className: "check-label" },
                    React.createElement("input", { type: "checkbox", checked: unsafe, onChange: e => setUnsafe(e.target.checked) }),
                    " He revisado y acepto estas capacidades sensibles.")),
            React.createElement("details", null,
                React.createElement("summary", null, "Advertencias y comando previsto"),
                React.createElement("ul", { className: "hints" }, preview.warnings.map((v, i) => React.createElement("li", { key: i }, v))),
                React.createElement("pre", { className: "console" },
                    "docker ",
                    preview.command.map(a => JSON.stringify(a)).join(' '))),
            !!preview.routes?.length && React.createElement("section", { className: "review-routes" },
                React.createElement("h3", null, "URLs que NearProd configurar\u00E1"),
                preview.routes.map(r => React.createElement("p", { key: r.host },
                    React.createElement("code", null, r.host),
                    " \u2192 ",
                    r.service,
                    ":",
                    r.port)),
                React.createElement("p", null, "Se a\u00F1adir\u00E1 un override administrado para la red de Traefik. Los Compose del repositorio no cambian; los puertos publicados se conservan. Activa el proxy desde Accesos locales.")),
            React.createElement(Alert, null,
                preview.note,
                " Aprobar permite ejecutar c\u00F3digo del repositorio con tus permisos Docker; no crea una zona aislada de confianza."),
            React.createElement("div", { className: "modal-actions" },
                React.createElement("button", { onClick: onClose }, "Cerrar"),
                React.createElement("button", { className: "primary", disabled: busy || preview.blockers.length > 0 || (preview.risks.length > 0 && !unsafe), onClick: () => void approve() }, busy ? 'Aprobando…' : 'Aprobar esta configuración'))));
}
export function ImageDialog({ container, onClose }) {
    const [info, setInfo] = useState(null), [error, setError] = useState('');
    useEffect(() => { void api(`/images?id=${encodeURIComponent(container.imageId)}`).then(setInfo).catch(e => setError(message(e))); }, [container.imageId]);
    return React.createElement(Modal, { title: "Artefacto en ejecuci\u00F3n", subtitle: container.name, onClose: onClose },
        error && React.createElement(Alert, { error: true }, error),
        !info && !error && React.createElement(Skeleton, { label: "Consultando imagen", rows: 4 }),
        info && React.createElement(React.Fragment, null,
            React.createElement("label", null,
                "Imagen",
                React.createElement("input", { readOnly: true, value: container.image })),
            React.createElement("label", null,
                "ID de imagen",
                React.createElement("input", { readOnly: true, value: info.id })),
            React.createElement("label", null,
                "Plataforma",
                React.createElement("input", { readOnly: true, value: info.platform || 'Sin comprobar' })),
            React.createElement("label", null,
                "Digests",
                React.createElement("textarea", { readOnly: true, rows: 3, value: info.digests.join('\n') || 'No disponible (una imagen local puede no tener digest de registro).' })),
            React.createElement(Alert, null, "El mismo Dockerfile o etiqueta no demuestra que sea el mismo artefacto del despliegue. ARM64 y AMD64 son plataformas diferentes.")));
}
/** Stable space, screen-reader status and reduced-motion support in CSS. */
export function Skeleton({ label = 'Cargando datos', rows = 3, compact = false }) {
    return React.createElement("span", { className: `skeleton-block ${compact ? 'compact' : ''}`, role: "status", "aria-label": label, "aria-busy": "true" },
        React.createElement("span", { className: "sr-only" },
            label,
            "\u2026"),
        Array.from({ length: rows }, (_, i) => React.createElement("span", { "aria-hidden": "true", key: i, className: `skeleton-line ${i === rows - 1 ? 'short' : ''}` })));
}
export function Field({ label, help, children }) {
    const id = React.useId();
    return React.createElement("div", { className: "field" },
        React.createElement("label", { htmlFor: id }, label),
        React.cloneElement(children, { id, 'aria-describedby': help ? `${id}-help` : undefined }),
        help && React.createElement("p", { className: "field-help", id: `${id}-help` }, help));
}
