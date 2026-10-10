import { api, message } from './api.js';
import { Alert, Busy, Field, Modal } from './components.js';
const { useEffect, useState } = React;
const resourceLabels = { container: 'Contenedor', volume: 'Volumen y datos', network: 'Red', image: 'Imagen construida', database: 'Base, cuenta y credencial', file: 'Archivo generado' };
export function StackPurgeDialog({ preview: initial, onClose, onApplied }) {
    const [preview, setPreview] = useState(initial), [typedId, setTypedId] = useState(''), [acknowledged, setAcknowledged] = useState(false);
    const [busy, setBusy] = useState(false), [error, setError] = useState(''), [operation, setOperation] = useState(null);
    const running = operation?.state === 'running' || operation?.state === 'queued';
    const succeeded = operation?.state === 'succeeded';
    const failed = operation && !running && !succeeded;
    useEffect(() => {
        if (!operation?.id || !running)
            return;
        const controller = new AbortController();
        let timer;
        async function poll() {
            try {
                const next = await api(`/operations/${operation.id}`, undefined, controller.signal);
                if (!controller.signal.aborted) {
                    setOperation(next);
                    setError('');
                }
                if (!controller.signal.aborted && ['running', 'queued'].includes(next.state))
                    timer = setTimeout(() => void poll(), 800);
            }
            catch (e) {
                if (!controller.signal.aborted) {
                    setError(`No se pudo consultar el progreso: ${message(e)}. La eliminación puede seguir en curso.`);
                    timer = setTimeout(() => void poll(), 2000);
                }
            }
        }
        void poll();
        return () => { controller.abort(); clearTimeout(timer); };
    }, [operation?.id, running]);
    async function submit() {
        setBusy(true);
        setError('');
        try {
            setOperation(await api('/stacks/purge', { target: preview.stack.id, fingerprint: preview.fingerprint, confirm: true, typedId, acknowledgeDataLoss: acknowledged }));
        }
        catch (e) {
            setError(message(e));
        }
        finally {
            setBusy(false);
        }
    }
    async function reviewAgain() {
        setBusy(true);
        setError('');
        try {
            setPreview(await api('/stacks/purge-preview', { target: preview.stack.id }));
            setOperation(null);
            setTypedId('');
            setAcknowledged(false);
        }
        catch (e) {
            setError(message(e));
        }
        finally {
            setBusy(false);
        }
    }
    return React.createElement(Modal, { title: "Eliminar aplicaci\u00F3n y recursos", subtitle: `${preview.stack.name} · ${preview.stack.id}`, wide: true, onClose: () => { if (!busy)
            onClose(); } },
        succeeded ? React.createElement(Alert, null, "Eliminaci\u00F3n completada y verificada. Se retiraron los recursos y datos de la lista y el registro de la aplicaci\u00F3n. El c\u00F3digo se conserv\u00F3.") : React.createElement(React.Fragment, null,
            React.createElement(Alert, { warning: true }, preview.note),
            React.createElement("p", null,
                React.createElement("strong", null, "Se conserva el c\u00F3digo:"),
                " ",
                React.createElement("span", { className: "mono path" }, preview.stack.path)),
            preview.retry && React.createElement(Alert, { warning: true }, "Hay una eliminaci\u00F3n incompleta. Se comprobar\u00E1 cada recurso del plan original y se reintentar\u00E1 \u00FAnicamente lo pendiente."),
            React.createElement("section", { className: "panel" },
                React.createElement("h3", null,
                    "Recursos que se eliminar\u00E1n (",
                    preview.resources.length,
                    ")"),
                preview.resources.length ? React.createElement("div", { className: "table-wrap" },
                    React.createElement("table", null,
                        React.createElement("thead", null,
                            React.createElement("tr", null,
                                React.createElement("th", null, "Tipo"),
                                React.createElement("th", null, "Recurso"))),
                        React.createElement("tbody", null, preview.resources.map(r => React.createElement("tr", { key: `${r.kind}:${r.id}` },
                            React.createElement("td", null, resourceLabels[r.kind] || r.kind),
                            React.createElement("td", { className: "mono" }, r.name)))))) : React.createElement("p", null, "No se encontraron recursos f\u00EDsicos propios. Se retirar\u00E1n las rutas, vinculaciones y metadata de la aplicaci\u00F3n.")),
            !!preview.preserved.length && React.createElement("details", null,
                React.createElement("summary", null, "Recursos externos o independientes que se conservan"),
                React.createElement("ul", null, preview.preserved.map((r, i) => React.createElement("li", { key: i },
                    React.createElement("strong", null, r.name),
                    ": ",
                    r.reason)))),
            !!preview.blockers.length && React.createElement(Alert, { error: true },
                React.createElement("strong", null, "No se puede completar la eliminaci\u00F3n:"),
                React.createElement("ul", null, preview.blockers.map((b, i) => React.createElement("li", { key: i }, b)))),
            !operation && React.createElement(React.Fragment, null,
                React.createElement(Field, { label: "Escribe exactamente el ID de la aplicaci\u00F3n", help: preview.stack.id },
                    React.createElement("input", { autoComplete: "off", spellCheck: false, value: typedId, disabled: busy, onChange: e => setTypedId(e.target.value) })),
                React.createElement("label", { className: "check-label" },
                    React.createElement("input", { type: "checkbox", checked: acknowledged, disabled: busy, onChange: e => setAcknowledged(e.target.checked) }),
                    " Entiendo que se eliminar\u00E1n los datos y recursos listados y que esta operaci\u00F3n no se puede deshacer."))),
        operation && React.createElement("section", { "aria-live": "polite", className: "panel" },
            React.createElement("h3", null, running ? 'Eliminando recursos…' : succeeded ? 'Limpieza verificada' : 'Eliminación incompleta'),
            running && React.createElement(Busy, null),
            operation.error && React.createElement(Alert, { error: true }, operation.error.message),
            React.createElement("pre", { className: "console" }, operation.lines.map(l => l.text).join('\n') || 'Comprobando el plan de eliminación…'),
            running && React.createElement("p", { className: "hint" }, "Puedes cerrar este di\u00E1logo y seguir el progreso en Actividad.")),
        error && React.createElement(Alert, { error: true }, error),
        React.createElement("div", { className: "modal-actions" },
            React.createElement("button", { disabled: busy, onClick: succeeded ? onApplied : onClose }, succeeded ? 'Cerrar' : operation ? 'Cerrar y ver actividad' : 'Cancelar'),
            !operation && React.createElement(React.Fragment, null,
                React.createElement("button", { disabled: busy, onClick: () => void reviewAgain() }, "Actualizar revisi\u00F3n"),
                React.createElement("button", { className: "danger", disabled: busy || !!preview.blockers.length || typedId !== preview.stack.id || !acknowledged, onClick: () => void submit() }, busy ? 'Comprobando…' : preview.retry ? 'Reintentar recursos pendientes' : 'Eliminar definitivamente')),
            failed && React.createElement("button", { className: "danger", disabled: busy, onClick: () => void reviewAgain() }, "Revisar y reintentar recursos pendientes")));
}
