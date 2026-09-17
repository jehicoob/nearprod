import { api, message } from './api.js';
import { RouteEditor } from './proxy.js';
import { Alert, Field, Icon, LiveStatus, Modal, Skeleton } from './components.js';
const { useState, useEffect } = React;
export const slugify = (v) => v.normalize('NFKD').replace(/[\u0300-\u036f]/g, '').toLowerCase().replace(/[^a-z0-9_-]+/g, '-').replace(/^[-_]+|[-_]+$/g, '').slice(0, 48) || 'proyecto';
const blankMode = () => ({ files: [], envFiles: [], profiles: [] });
const absolute = (base, name) => name.startsWith('/') ? name : `${base.replace(/\/$/, '')}/${name}`;
const shortName = (base, file) => file.startsWith(base + '/') ? file.slice(base.length + 1) : file;
function fromCandidate(c, stack) {
    return stack ? { product: stack.product, slug: stack.slug, name: stack.name, path: stack.path, projectName: stack.projectName, modes: structuredClone(stack.modes), links: stack.links || [], routes: structuredClone(stack.routes || []) } : {
        product: c?.product || '', slug: c?.slug || 'app', name: c?.relative.split('/').at(-1) || '', path: c?.path || '', projectName: '',
        modes: { dev: { files: (c?.suggested || []).map(f => absolute(c.path, f)), envFiles: [], profiles: [] } }, links: [],
    };
}
export function GroupPicker({ groups, value, name, onChange }) {
    const [choice, setChoice] = useState(groups.some(g => g.id === value) ? value : '__new');
    return React.createElement("div", { className: "group-picker" },
        React.createElement(Field, { label: "Grupo", help: "Re\u00FAne frontend, backend y otros componentes de un producto. Agrupar no crea redes, dominios ni mezcla sus archivos Compose." },
            React.createElement("select", { value: choice, onChange: e => { const id = e.target.value; setChoice(id); const g = groups.find(g => g.id === id); onChange(g?.id || slugify(name || 'Mi proyecto'), g?.name || name || 'Mi proyecto'); } },
                groups.map(g => React.createElement("option", { key: g.id, value: g.id }, g.name)),
                React.createElement("option", { value: "__new" }, "\uFF0B Crear un grupo nuevo"))),
        choice === '__new' && React.createElement(Field, { label: "Nombre del nuevo grupo", help: "Por ejemplo, M\u00E1ximo Puntaje. Es un nombre de organizaci\u00F3n dentro de NearProd." },
            React.createElement("input", { required: true, maxLength: 120, value: name, onChange: e => onChange(slugify(e.target.value), e.target.value), placeholder: "M\u00E1ximo Puntaje" })));
}
function OrderedFiles({ label, help, choices, value, onChange, allowEmpty = false }) {
    const [manual, setManual] = useState('');
    const all = [...choices, ...value.filter(v => !choices.some(c => c.path === v)).map(path => ({ path, name: path, badge: 'Ruta explícita' }))];
    const move = (i, offset) => { const next = [...value]; [next[i], next[i + offset]] = [next[i + offset], next[i]]; onChange(next); };
    return React.createElement("div", { className: "file-picker" },
        React.createElement("h4", null, label),
        React.createElement("p", { className: "field-help" }, help),
        all.map(file => React.createElement("label", { className: "file-option", key: file.path },
            React.createElement("input", { type: "checkbox", checked: value.includes(file.path), onChange: e => onChange(e.target.checked ? [...value, file.path] : value.filter(v => v !== file.path)) }),
            React.createElement("span", null,
                React.createElement("code", null, file.name),
                file.badge && React.createElement("small", null, file.badge)))),
        value.length > 1 && React.createElement("div", { className: "ordered-selection" },
            React.createElement("strong", null, "Orden de aplicaci\u00F3n"),
            value.map((v, i) => React.createElement("div", { key: v },
                React.createElement("span", { className: "file-number" }, i + 1),
                React.createElement("code", null, all.find(c => c.path === v)?.name || v),
                React.createElement("button", { type: "button", disabled: i === 0, "aria-label": `Subir ${all.find(c => c.path === v)?.name || v}`, onClick: () => move(i, -1) }, "\u2191"),
                React.createElement("button", { type: "button", disabled: i === value.length - 1, "aria-label": `Bajar ${all.find(c => c.path === v)?.name || v}`, onClick: () => move(i, 1) }, "\u2193")))),
        !value.length && !allowEmpty && React.createElement("p", { className: "warning-text" }, "Selecciona al menos un archivo antes de continuar."),
        React.createElement("details", { className: "advanced compact-details" },
            React.createElement("summary", null, "A\u00F1adir una ruta que no aparece"),
            React.createElement("p", { className: "field-help" }, "Para archivos en otra subcarpeta o ra\u00EDz autorizada. La ruta se valida al guardar; no se copia el archivo."),
            React.createElement("div", { className: "inline-input" },
                React.createElement("input", { "aria-label": `Ruta adicional: ${label}`, value: manual, onChange: e => setManual(e.target.value), placeholder: "config/compose.local.yaml" }),
                React.createElement("button", { type: "button", disabled: !manual.trim() || value.includes(manual.trim()), onClick: () => { onChange([...value, manual.trim()]); setManual(''); } }, "A\u00F1adir"))));
}
function ModeEditor({ title, path, mode, onChange, initialOptions }) {
    const [options, setOptions] = useState(initialOptions), [loading, setLoading] = useState(false), [error, setError] = useState('');
    const key = JSON.stringify(mode.files);
    useEffect(() => {
        if (!path)
            return;
        const controller = new AbortController();
        setLoading(true);
        setError('');
        const timer = setTimeout(() => { void api('/project-options', { path, files: mode.files }, controller.signal).then(o => { if (!controller.signal.aborted)
            setOptions(o); }).catch(e => { if (!controller.signal.aborted)
            setError(message(e)); }).finally(() => { if (!controller.signal.aborted)
            setLoading(false); }); }, 150);
        return () => { clearTimeout(timer); controller.abort(); };
    }, [path, key]);
    const [customEnv, setCustomEnv] = useState(mode.envFiles.length > 0);
    const profiles = options?.profiles || [];
    return React.createElement("fieldset", { className: "mode-editor" },
        React.createElement("legend", null, title),
        loading && !options ? React.createElement(Skeleton, { label: "Leyendo nombres de archivos y perfiles", rows: 4 }) : React.createElement(React.Fragment, null,
            React.createElement(OrderedFiles, { label: "Configuraci\u00F3n Docker Compose", help: "Estos archivos describen los servicios, puertos y vol\u00FAmenes. Con uno usas su configuraci\u00F3n; con varios Docker los combina EN ORDEN: el primero es la base y los siguientes pueden a\u00F1adir o reemplazar ajustes. No son aplicaciones separadas y no se editan aqu\u00ED. Solo se pasan los seleccionados: compose.override.yaml no se a\u00F1ade autom\u00E1ticamente.", choices: (options?.composeFiles || []).map(f => ({ ...f, badge: f.base ? 'Archivo base' : 'Variante · revisar su propósito' })), value: mode.files.map(f => absolute(path, f)), onChange: files => onChange({ ...mode, files }) }),
            React.createElement("div", { className: "env-selector" },
                React.createElement("h4", null, "Variables para configurar Compose"),
                React.createElement("p", { className: "field-help" },
                    "Sirven para completar referencias como ",
                    React.createElement("code", null, '${APP_PORT}'),
                    " en Compose. Esto no inyecta autom\u00E1ticamente todas las variables dentro del contenedor: eso lo declara ",
                    React.createElement("code", null, "environment"),
                    " o ",
                    React.createElement("code", null, "env_file"),
                    " en el YAML."),
                React.createElement("label", { className: "file-option" },
                    React.createElement("input", { type: "radio", checked: !customEnv, onChange: () => { setCustomEnv(false); onChange({ ...mode, envFiles: [] }); } }),
                    React.createElement("span", null,
                        "Autom\u00E1tico ",
                        React.createElement("small", null, options?.envFiles.some(f => f.automatic) ? 'Usar .env de esta carpeta, según las reglas de Compose.' : 'No se encontró .env aquí. No se añaden archivos explícitos.'))),
                React.createElement("label", { className: "file-option" },
                    React.createElement("input", { type: "radio", checked: customEnv, onChange: () => setCustomEnv(true) }),
                    React.createElement("span", null,
                        "Elegir archivos de entorno ",
                        React.createElement("small", null, "Solo se listan nombres; sus valores y contrase\u00F1as no se muestran."))),
                customEnv && React.createElement(React.Fragment, null,
                    React.createElement(OrderedFiles, { label: "Archivos .env", help: "Se pasan como --env-file en el orden elegido. Si una variable se repite, el \u00FAltimo archivo tiene prioridad. Al elegir .env.local no se a\u00F1ade .env autom\u00E1ticamente; selecci\u00F3nalos ambos si necesitas combinar sus valores.", choices: (options?.envFiles || []).map(f => ({ ...f, badge: f.template ? 'Plantilla; no contiene necesariamente valores utilizables' : f.automatic ? 'Predeterminado de Compose' : 'Archivo local' })), value: mode.envFiles.map(f => absolute(path, f)), onChange: envFiles => onChange({ ...mode, envFiles }) }),
                    !options?.envFiles.length && React.createElement("p", { className: "hint" }, "No hay archivos de entorno en esta carpeta. Puedes usar el modo autom\u00E1tico o a\u00F1adir una ruta expl\u00EDcita."),
                    !mode.envFiles.length && React.createElement("p", { className: "hint" }, "Sin una selecci\u00F3n se guardar\u00E1 el comportamiento autom\u00E1tico.")),
                !!options?.serviceEnvFiles.length && React.createElement("p", { className: "hint" },
                    "El YAML ya declara ",
                    React.createElement("code", null, "env_file"),
                    ": ",
                    options.serviceEnvFiles.join(', '),
                    ". Compose se encarga de esos archivos; no hace falta seleccionarlos otra vez aqu\u00ED.")),
            React.createElement("div", { className: "profiles-selector" },
                React.createElement("h4", null, "Servicios opcionales \u2014 perfiles Compose"),
                React.createElement("p", { className: "field-help" }, "No son grupos de NearProd ni perfiles de Colima. Activan servicios que el proyecto marc\u00F3 como opcionales, por ejemplo un worker o una herramienta de administraci\u00F3n."),
                React.createElement(LiveStatus, { message: loading ? 'Actualizando perfiles según los archivos seleccionados.' : '' }),
                !profiles.length && !mode.profiles.length && !loading && React.createElement("p", { className: "hint" }, "Este conjunto de archivos no declara perfiles. No necesitas configurar nada aqu\u00ED."),
                profiles.map(p => React.createElement("label", { className: "file-option", key: p.name },
                    React.createElement("input", { type: "checkbox", checked: mode.profiles.includes(p.name), onChange: e => onChange({ ...mode, profiles: e.target.checked ? [...mode.profiles, p.name] : mode.profiles.filter(v => v !== p.name) }) }),
                    React.createElement("span", null,
                        React.createElement("code", null, p.name),
                        React.createElement("small", null,
                            "Activa: ",
                            p.services.join(', '))))),
                mode.profiles.filter(p => !profiles.some(v => v.name === p)).map(p => React.createElement("label", { className: "file-option", key: p },
                    React.createElement("input", { type: "checkbox", checked: true, onChange: () => onChange({ ...mode, profiles: mode.profiles.filter(v => v !== p) }) }),
                    React.createElement("span", null,
                        p,
                        React.createElement("small", null, "No detectado en la lectura est\u00E1tica; revisar con Compose.")))),
                !!options?.defaultServices.length && React.createElement("p", { className: "hint" },
                    "Sin activar perfiles, se incluyen: ",
                    options.defaultServices.join(', '),
                    ". La revisi\u00F3n con Compose valida el modelo efectivo."))),
        error && React.createElement(Alert, { error: true }, error),
        options?.warnings.map(w => React.createElement(Alert, { key: w }, w)));
}
export function StackFields({ value, onChange, existing, onReady }) {
    const [options, setOptions] = useState(null), [loading, setLoading] = useState(false), [error, setError] = useState('');
    const latest = React.useRef(value);
    latest.current = value;
    useEffect(() => { onReady(Boolean(options && !loading && !error && value.projectName)); }, [options, loading, error, value.projectName, onReady]);
    useEffect(() => {
        setOptions(null);
        if (!value.path)
            return;
        const controller = new AbortController();
        setLoading(true);
        setError('');
        const timer = setTimeout(() => {
            void api('/project-options', { path: value.path, ...(value.modes.dev.files.length ? { files: value.modes.dev.files } : {}) }, controller.signal).then(o => {
                if (controller.signal.aborted)
                    return;
                setOptions(o);
                const d = latest.current;
                if (!existing && (!d.projectName || !d.modes.dev.files.length))
                    onChange({ ...d, projectName: d.projectName || o.projectName, modes: { ...d.modes, dev: { ...d.modes.dev, files: d.modes.dev.files.length ? d.modes.dev.files : o.suggestedFiles } } });
            }).catch(e => { if (!controller.signal.aborted)
                setError(message(e)); }).finally(() => { if (!controller.signal.aborted)
                setLoading(false); });
        }, 150);
        return () => { clearTimeout(timer); controller.abort(); };
    }, [value.path]);
    const setMode = (key, mode) => onChange({ ...value, modes: { ...value.modes, [key]: mode } });
    return React.createElement("div", { className: "stack-fields" },
        React.createElement(Field, { label: "Nombre de la aplicaci\u00F3n", help: "El nombre visible de esta pieza del grupo; por ejemplo Frontend o API. Un Compose con varios servicios sigue siendo una sola aplicaci\u00F3n aqu\u00ED." },
            React.createElement("input", { required: true, value: value.name, onChange: e => onChange({ ...value, name: e.target.value }), maxLength: 120, placeholder: "Backend de M\u00E1ximo Puntaje" })),
        existing || options ? React.createElement("div", { className: "checkout-summary" },
            React.createElement(Icon, { name: "folder" }),
            React.createElement("span", null,
                React.createElement("strong", null, "Carpeta de la aplicaci\u00F3n"),
                React.createElement("code", null, value.path)),
            !existing && React.createElement("button", { type: "button", onClick: () => { setOptions(null); onChange({ ...value, path: '', projectName: '', modes: { dev: blankMode() } }); } }, "Cambiar")) : React.createElement(Field, { label: "Carpeta de la aplicaci\u00F3n", help: "Directorio donde est\u00E1 el Compose. Debe estar dentro de una ra\u00EDz autorizada desde Descubrir." },
            React.createElement("input", { required: true, value: value.path, onChange: e => onChange({ ...value, path: e.target.value, projectName: '', modes: { dev: blankMode() } }), placeholder: "/Users/tu-usuario/Projects/proyecto/backend" })),
        loading && !options ? React.createElement(Skeleton, { label: "Buscando configuraciones de esta aplicaci\u00F3n", rows: 5 }) : React.createElement(React.Fragment, null,
            React.createElement(ModeEditor, { title: "Desarrollo local", path: value.path, mode: value.modes.dev, onChange: m => setMode('dev', m), initialOptions: options }),
            React.createElement("details", { className: "advanced", open: value.modes.verify ? true : undefined },
                React.createElement("summary", null, "Prueba de imagen \u2014 opcional"),
                React.createElement("p", null, "Desarrollo permite editar c\u00F3digo y recargarlo. La prueba de imagen ejecuta el c\u00F3digo empaquetado dentro de la imagen, sin montajes del c\u00F3digo ni recarga de desarrollo. No hace falta activarla para empezar a usar NearProd."),
                React.createElement("label", { className: "check-label" },
                    React.createElement("input", { type: "checkbox", checked: Boolean(value.modes.verify), onChange: e => { const modes = { ...value.modes }; if (e.target.checked)
                            modes.verify = { ...blankMode(), files: options?.suggestedFiles || [] };
                        else
                            delete modes.verify; onChange({ ...value, modes }); } }),
                    "Habilitar prueba de imagen"),
                value.modes.verify && React.createElement(React.Fragment, null,
                    React.createElement(ModeEditor, { title: "Configuraci\u00F3n para probar la imagen", path: value.path, mode: value.modes.verify, onChange: m => setMode('verify', m), initialOptions: options }),
                    React.createElement(Alert, null, "Solo se guarda una configuraci\u00F3n adicional; no se ejecuta ni se construye ahora. Este modo NO certifica producci\u00F3n y comparte los vol\u00FAmenes/datos del modo de desarrollo. La versi\u00F3n actual rechaza todos los bind mounts en esta prueba."))),
            React.createElement(RouteEditor, { value: value, onChange: onChange, initialOptions: options }),
            React.createElement("details", { className: "advanced" },
                React.createElement("summary", null, "Identidad t\u00E9cnica y opciones avanzadas"),
                React.createElement(Field, { label: "Identificador de aplicaci\u00F3n", help: "Se usa en la CLI, por ejemplo grupo/backend. Se propone autom\u00E1ticamente; no es un dominio." },
                    React.createElement("input", { value: value.slug, disabled: Boolean(existing), pattern: "[a-z0-9][a-z0-9_-]{0,62}", onChange: e => onChange({ ...value, slug: e.target.value }) })),
                React.createElement(Field, { label: "Nombre de proyecto Compose", help: "Docker usa este nombre para identificar contenedores, redes y vol\u00FAmenes. Si la aplicaci\u00F3n ya existe, conserva su nombre exacto. Cambiar de grupo no lo cambia." },
                    React.createElement("input", { disabled: Boolean(existing), pattern: "[a-z0-9][a-z0-9_-]{0,62}", value: value.projectName, onChange: e => onChange({ ...value, projectName: e.target.value }) })),
                React.createElement("p", { className: "hint" },
                    "Sugerencia: ",
                    options?.projectNameSource === 'existing' ? 'identidad encontrada en el registro o contenedores observados' : options?.projectNameSource === 'compose-name' ? 'campo name del Compose' : 'nombre de la carpeta',
                    ". COMPOSE_PROJECT_NAME o ejecuciones con -p pueden haber usado otro nombre: compru\u00E9balo antes de iniciar.")),
            React.createElement("details", { className: "advanced" },
                React.createElement("summary", null, "Enlaces manuales existentes \u2014 avanzado"),
                React.createElement("p", null,
                    "A\u00F1ade direcciones que ya funcionan, por ejemplo ",
                    React.createElement("code", null, "http://proyecto.localhost"),
                    " y ",
                    React.createElement("code", null, "http://api-proyecto.localhost"),
                    ". NearProd las guarda como accesos directos; no crea el dominio, el proxy ni la conexi\u00F3n entre frontend y backend."),
                value.links.map((link, i) => React.createElement("div", { className: "link-editor", key: i },
                    React.createElement(Field, { label: `Nombre del enlace ${i + 1}`, help: "Por ejemplo, Sitio o API." },
                        React.createElement("input", { required: true, maxLength: 80, value: link.label, onChange: e => onChange({ ...value, links: value.links.map((v, j) => j === i ? { ...v, label: e.target.value } : v) }) })),
                    React.createElement(Field, { label: `URL local ${i + 1}`, help: "HTTP o HTTPS; localhost, .localhost o loopback. Sin contrase\u00F1as ni par\u00E1metros." },
                        React.createElement("input", { required: true, type: "url", value: link.url, onChange: e => onChange({ ...value, links: value.links.map((v, j) => j === i ? { ...v, url: e.target.value } : v) }), placeholder: "http://api-proyecto.localhost" })),
                    React.createElement("button", { type: "button", onClick: () => onChange({ ...value, links: value.links.filter((_, j) => j !== i) }) }, "Quitar enlace"))),
                React.createElement("button", { type: "button", disabled: value.links.length >= 8, onClick: () => onChange({ ...value, links: [...value.links, { label: 'Abrir aplicación', url: '' }] }) },
                    React.createElement(Icon, { name: "plus", size: 14 }),
                    "A\u00F1adir enlace existente"),
                React.createElement("p", { className: "hint" }, "Para que NearProd configure el proxy utiliza la secci\u00F3n URL de acceso anterior; estos son solo marcadores. Los puertos publicados por Docker se muestran por separado."))),
        error && React.createElement(Alert, { error: true }, error));
}
function validateDraft(d) {
    if (!d.name.trim() || !d.path.trim() || !d.projectName.trim() || !d.product.trim())
        return 'Completa nombre, grupo y carpeta. Espera la lectura de archivos o revisa la identidad técnica.';
    if (!/^[a-z0-9][a-z0-9_-]{0,62}$/.test(d.slug) || !/^[a-z0-9][a-z0-9_-]{0,62}$/.test(d.projectName))
        return 'Revisa los identificadores en Opciones avanzadas: usa letras minúsculas, números, guiones o guiones bajos.';
    if (!d.modes.dev.files.length || d.modes.verify && !d.modes.verify.files.length)
        return 'Selecciona al menos un archivo Compose para cada modo habilitado.';
    if ((d.routes || []).some(v => !v.host.trim() || !v.service || !Number.isInteger(v.port) || v.port < 1 || v.port > 65535))
        return 'Completa dominio, servicio HTTP y puerto interno de cada URL, o quita la URL que no necesitas.';
    if (d.links.some(l => !l.label.trim() || !l.url.trim()))
        return 'Completa o elimina los enlaces vacíos.';
    return null;
}
export function StackEditor({ candidate, stack, groups, onClose, onSaved }) {
    const [draft, setDraft] = useState(() => fromCandidate(candidate, stack));
    const [groupName, setGroupName] = useState(groups.find(g => g.id === draft.product)?.name || candidate?.product || 'Mi proyecto');
    const [busy, setBusy] = useState(false), [error, setError] = useState(''), [ready, setReady] = useState(false);
    async function save(e) {
        e.preventDefault();
        const d = { ...draft, product: draft.product || slugify(groupName), groupName };
        const invalid = validateDraft(d);
        if (invalid) {
            setError(invalid);
            return;
        }
        setBusy(true);
        setError('');
        try {
            await api(stack ? '/stacks/edit' : '/stacks', stack ? { target: stack.id, definition: d } : d);
            onSaved();
        }
        catch (e) {
            setError(message(e));
        }
        finally {
            setBusy(false);
        }
    }
    return React.createElement(Modal, { title: stack ? 'Configurar aplicación' : 'Registrar aplicación', subtitle: "Elige el grupo y revisa la configuraci\u00F3n encontrada. Guardar no inicia contenedores ni modifica tus repositorios.", onClose: busy ? () => { } : onClose, wide: true },
        React.createElement("form", { onSubmit: e => void save(e) },
            React.createElement(GroupPicker, { groups: groups, value: draft.product, name: groupName, onChange: (product, name) => { setDraft({ ...draft, product }); setGroupName(name); } }),
            stack && React.createElement("p", { className: "hint" }, "Cambiar de grupo conserva el nombre Compose, los vol\u00FAmenes y la propiedad de los contenedores. Actualiza la referencia grupo/aplicaci\u00F3n de tus comandos CLI."),
            React.createElement(StackFields, { value: draft, onChange: setDraft, existing: stack, onReady: setReady }),
            error && React.createElement(Alert, { error: true }, error),
            React.createElement("div", { className: "modal-actions" },
                React.createElement("button", { type: "button", disabled: busy, onClick: onClose }, "Cancelar"),
                React.createElement("button", { className: "primary", disabled: busy || !ready }, busy ? 'Guardando…' : stack ? 'Guardar cambios' : 'Registrar sin ejecutar'))));
}
export function BatchEditor({ candidates, groups, onClose, onSaved }) {
    const [drafts, setDrafts] = useState(() => candidates.map(c => fromCandidate(c)));
    const [product, setProduct] = useState(candidates[0]?.product || 'mi-proyecto');
    const [groupName, setGroupName] = useState(groups.find(g => g.id === product)?.name || product);
    const [step, setStep] = useState(-1), [error, setError] = useState(''), [busy, setBusy] = useState(false), [ready, setReady] = useState(false);
    useEffect(() => { setReady(false); }, [step]);
    const summary = step === drafts.length;
    async function next(e) {
        e.preventDefault();
        setError('');
        if (step === -1) {
            if (!groupName.trim()) {
                setError('Escribe el nombre del grupo.');
                return;
            }
            setDrafts(ds => ds.map(d => ({ ...d, product, groupName })));
            setStep(0);
            return;
        }
        if (!summary) {
            const invalid = validateDraft({ ...drafts[step], product });
            if (invalid) {
                setError(invalid);
                return;
            }
            setStep(step + 1);
            return;
        }
        setBusy(true);
        try {
            await api('/stacks/batch', { definitions: drafts.map(d => ({ ...d, product })), groupName });
            onSaved();
        }
        catch (e) {
            setError(message(e));
        }
        finally {
            setBusy(false);
        }
    }
    return React.createElement(Modal, { title: "Registrar selecci\u00F3n en un grupo", subtitle: "Un solo registro para tu selecci\u00F3n. Si hay un conflicto, no se guarda parcialmente ni se ejecuta Docker.", onClose: busy ? () => { } : onClose, wide: true },
        React.createElement("form", { onSubmit: e => void next(e) },
            React.createElement("div", { className: "wizard-steps", "aria-label": "Progreso del registro" },
                React.createElement("span", { className: step === -1 ? 'current' : '' }, "1 \u00B7 Grupo"),
                React.createElement("span", { className: step >= 0 && !summary ? 'current' : '' }, "2 \u00B7 Configuraci\u00F3n"),
                React.createElement("span", { className: summary ? 'current' : '' }, "3 \u00B7 Resumen")),
            step === -1 && React.createElement(React.Fragment, null,
                React.createElement(GroupPicker, { groups: groups, value: product, name: groupName, onChange: (id, name) => { setProduct(id); setGroupName(name); } }),
                React.createElement("h3", null,
                    drafts.length,
                    " aplicaciones seleccionadas"),
                React.createElement("ul", { className: "hints" }, drafts.map(d => React.createElement("li", { key: d.path }, d.path))),
                React.createElement(Alert, null, "Agrupar no conecta los servicios entre s\u00ED ni combina sus Compose. Cada aplicaci\u00F3n conserva su identidad Docker.")),
            step >= 0 && !summary && React.createElement(React.Fragment, null,
                React.createElement("div", { className: "wizard-context" },
                    React.createElement("strong", null, groupName),
                    React.createElement("span", null,
                        "Aplicaci\u00F3n ",
                        step + 1,
                        " de ",
                        drafts.length)),
                React.createElement(StackFields, { key: drafts[step].path || step, value: drafts[step], onReady: setReady, onChange: d => setDrafts(ds => ds.map((v, i) => i === step ? d : v)) })),
            summary && React.createElement(React.Fragment, null,
                React.createElement("h3", null,
                    "Todo ir\u00E1 al grupo \u00AB",
                    groupName,
                    "\u00BB"),
                drafts.map((d, i) => React.createElement("article", { className: "registration-summary", key: d.path },
                    React.createElement("div", null,
                        React.createElement("strong", null, d.name),
                        React.createElement("button", { type: "button", onClick: () => setStep(i) }, "Revisar")),
                    React.createElement("code", null, d.projectName),
                    React.createElement("p", null,
                        "Compose: ",
                        d.modes.dev.files.map(f => shortName(d.path, f)).join(' → ')),
                    React.createElement("p", null,
                        "Entorno: ",
                        d.modes.dev.envFiles.length ? d.modes.dev.envFiles.map(f => shortName(d.path, f)).join(' → ') : 'Automático (.env si existe)'),
                    React.createElement("p", null,
                        "URLs: ",
                        (d.routes || []).map(v => v.host + ' → ' + v.service + ':' + v.port).join(' · ') || 'Sin acceso HTTP (DB/worker o pendiente de configurar)'),
                    React.createElement("p", null,
                        "Servicios opcionales: ",
                        d.modes.dev.profiles.join(', ') || 'Ninguno',
                        " \u00B7 Prueba de imagen: ",
                        d.modes.verify ? 'Configurada' : 'No configurada'))),
                React.createElement(Alert, null, "Despu\u00E9s de registrar podr\u00E1s revisar, aprobar e iniciar las aplicaciones. No se crear\u00E1n dominios ni se activar\u00E1n servicios ahora.")),
            error && React.createElement(Alert, { error: true }, error),
            React.createElement("div", { className: "modal-actions" },
                step > -1 && React.createElement("button", { type: "button", disabled: busy, onClick: () => { setError(''); setStep(step - 1); } }, "Atr\u00E1s"),
                React.createElement("button", { type: "button", disabled: busy, onClick: onClose }, "Cancelar"),
                React.createElement("button", { className: "primary", disabled: busy || step >= 0 && !summary && (!ready || !drafts[step]?.projectName) }, busy ? 'Registrando…' : summary ? 'Registrar grupo sin ejecutar' : step === -1 ? 'Revisar aplicaciones' : 'Continuar'))));
}
