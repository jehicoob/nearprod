import { api, message } from './api.js';
import { Alert, Badge, Field, Modal, Skeleton, Icon, LiveStatus } from './components.js';
const { useState, useEffect } = React;
function Review({ value, onConfirm, onClose, busy }) {
    const def = value.definition;
    const consumers = value.consumers;
    const services = value.services;
    return React.createElement(Modal, { title: "Revisar cambio de infraestructura", subtitle: "Solo se aplicar\u00E1 cuando confirmes", onClose: onClose, wide: true },
        def && React.createElement("section", { className: "panel" },
            React.createElement("h3", null,
                def.name,
                " \u00B7 ",
                def.requestedImage),
            React.createElement("p", null,
                React.createElement("strong", null, "Persistencia:"),
                " ",
                def.persistence.kind === 'folder' ? def.persistence.path + '/data' : def.persistence.kind === 'volume' ? 'Volumen Docker dentro de la VM' : 'Sin persistencia'),
            React.createElement("p", null,
                React.createElement("strong", null, "Desde tu equipo:"),
                " ",
                def.hostPort ? `127.0.0.1:${def.hostPort}` : 'Puerto no publicado'),
            React.createElement("p", null,
                React.createElement("strong", null, "L\u00EDmite:"),
                " ",
                def.memoryMiB,
                " MiB. Compartido con el resto de la VM.")),
        services && React.createElement("section", { className: "panel" },
            React.createElement("h3", null,
                "Conectar ",
                String(value.target),
                " a ",
                String(value.instance)),
            React.createElement("p", null,
                "Servicios consumidores: ",
                services.join(', '),
                "."),
            React.createElement("p", null,
                "Destino interno: ",
                React.createElement("code", null,
                    String(value.host),
                    ":",
                    String(value.port)),
                ". Las contrase\u00F1as no se muestran en esta revisi\u00F3n.")),
        consumers && React.createElement("section", { className: "panel" },
            React.createElement("h3", null,
                "Detener ",
                String(value.instance)),
            consumers.length ? consumers.map((c, i) => React.createElement("p", { key: i },
                c.stack,
                " \u00B7 ",
                c.active ? 'Consumidor activo' : 'No observado en ejecución')) : React.createElement("p", null, "No hay consumidores registrados. Puede haber clientes externos."),
            React.createElement("p", null, "Se detiene el motor, no se eliminan sus datos.")),
        value.warnings?.map((w, i) => React.createElement(Alert, { key: i }, w)),
        value.note && React.createElement("p", null, value.note),
        React.createElement("details", null,
            React.createElement("summary", null, "Detalles t\u00E9cnicos de lo que se aplicar\u00E1"),
            React.createElement("pre", { className: "console review-json" }, JSON.stringify(value, null, 2))),
        React.createElement("div", { className: "modal-actions" },
            React.createElement("button", { onClick: onClose }, "Volver"),
            React.createElement("button", { className: "primary", disabled: busy, onClick: onConfirm }, "Confirmar y aplicar")));
}
export function InfrastructureView({ catalog, status, notify }) {
    const [data, setData] = useState(null), [error, setError] = useState(''), [loading, setLoading] = useState(false), [busy, setBusy] = useState(false);
    const [create, setCreate] = useState(false), [dbFor, setDbFor] = useState(null), [connection, setConnection] = useState(null), [link, setLink] = useState(null), [logs, setLogs] = useState(null), [backup, setBackup] = useState(null);
    const [review, setReview] = useState(null);
    const active = catalog.operations.some(o => o.state === 'running');
    async function load() {
        setLoading(true);
        try {
            setData(await api('/infra'));
            setError('');
        }
        catch (e) {
            setError(message(e));
        }
        finally {
            setLoading(false);
        }
    }
    useEffect(() => { void load(); }, [status.checkedAt, catalog.operations]);
    async function action(body) {
        setBusy(true);
        try {
            const op = await api('/infra/actions', body);
            notify(`Operación ${op.id} iniciada. Sigue su resultado en Actividad.`);
            setReview(null);
            await load();
        }
        catch (e) {
            setError(message(e));
        }
        finally {
            setBusy(false);
        }
    }
    const canMutate = status.connected && !active && !busy;
    return React.createElement(React.Fragment, null,
        React.createElement("div", { className: "section-heading" },
            React.createElement("div", null,
                React.createElement("span", { className: "eyebrow" }, "SERVICIOS PARA TUS APLICACIONES"),
                React.createElement("h1", null, "Infraestructura compartida"),
                React.createElement("p", null, "Un motor, varias bases independientes. Enciende solo lo necesario.")),
            React.createElement("div", { className: "button-row" },
                React.createElement("button", { onClick: () => void load(), disabled: loading, "aria-busy": loading },
                    React.createElement(Icon, { name: "refresh" }),
                    "Actualizar"),
                React.createElement("button", { className: "primary", disabled: !canMutate, onClick: () => setCreate(true) }, "Crear instancia"))),
        React.createElement(LiveStatus, { message: data && loading ? 'Actualizando infraestructura. Se conserva la última información.' : '' }),
        React.createElement(Alert, null,
            "PostgreSQL/MySQL separan bases y usuarios por proyecto. Redis se crea dedicado a una aplicaci\u00F3n. ",
            React.createElement("strong", null, "Traefik es un componente central"),
            " y se gestiona en Accesos locales, no en esta lista. No se cambia la persistencia de los proyectos que registres."),
        !status.connected && React.createElement(Alert, { error: true }, "Docker no est\u00E1 conectado. Puedes ver el cat\u00E1logo; inicia/comprueba Colima desde Runtime antes de crear o ejecutar infraestructura."),
        error && React.createElement(Alert, { error: true }, error),
        !data && loading && React.createElement(Skeleton, { label: "Consultando infraestructura", rows: 6 }),
        data && !data.instances.length && React.createElement("section", { className: "empty" },
            React.createElement(Icon, { name: "cube", size: 32 }),
            React.createElement("h2", null, "Tus servicios reutilizables"),
            React.createElement("p", null,
                "Crea PostgreSQL o MySQL, a\u00F1ade una base y vincula los backends que deban utilizarla.",
                React.createElement("br", null),
                "La base que un proyecto ya declara en su Compose no se elimina ni sustituye autom\u00E1ticamente."),
            React.createElement("button", { disabled: !canMutate, onClick: () => setCreate(true) }, "Crear mi primera instancia")),
        data?.instances.map(r => React.createElement("section", { className: "panel infra-instance", key: r.uid },
            React.createElement("div", { className: "panel-heading" },
                React.createElement(Icon, { name: "cube" }),
                React.createElement("div", null,
                    React.createElement("h2", null, r.name),
                    React.createElement("span", { className: "mono muted" },
                        r.id,
                        " \u00B7 ",
                        r.requestedImage)),
                React.createElement(Badge, { value: r.execution }),
                React.createElement(Badge, { value: r.health })),
            React.createElement("div", { className: "infra-facts" },
                React.createElement("div", null,
                    React.createElement("span", null, "Persistencia de la instancia"),
                    React.createElement("code", null, r.location),
                    React.createElement("small", null, "Compartida f\u00EDsicamente por sus bases; no es un backup.")),
                React.createElement("div", null,
                    React.createElement("span", null, "Conexi\u00F3n dentro de Docker"),
                    React.createElement("code", null,
                        r.internalHost,
                        ":",
                        r.internalPort),
                    React.createElement("small", null, "Solo servicios unidos a esta red de datos.")),
                React.createElement("div", null,
                    React.createElement("span", null, "Acceso desde tu equipo"),
                    React.createElement("code", null, r.hostPort ? `127.0.0.1:${r.hostPort}` : 'No publicado'),
                    React.createElement("small", null,
                        "L\u00EDmite de contenedor: ",
                        r.memoryMiB,
                        " MiB; no reserva RAM."))),
            React.createElement("div", { className: "button-row" },
                React.createElement("button", { disabled: !canMutate, onClick: () => void action({ action: 'start', instance: r.id }) }, "Iniciar / comprobar"),
                React.createElement("button", { disabled: !canMutate, onClick: () => { void api('/infra/stop-preview', { instance: r.id }).then(value => setReview({ value, body: { action: 'stop', instance: r.id, allowActive: window.confirm('Detener una instancia puede interrumpir TODOS sus consumidores. Se mostrará una revisión antes de aplicar. ¿Permitir detenerla aunque haya consumidores activos?') } })).catch(e => setError(message(e))); } }, "Detener instancia"),
                React.createElement("button", { disabled: !status.connected, onClick: () => setLogs(r) }, "Logs"),
                React.createElement("button", { disabled: !canMutate || (r.engine === 'redis' && r.databases.length > 0), onClick: () => setDbFor(r) }, r.engine === 'redis' ? 'Crear credencial de aplicación' : 'Crear base + usuario')),
            React.createElement("div", { className: "table-wrap" },
                React.createElement("table", null,
                    React.createElement("thead", null,
                        React.createElement("tr", null,
                            React.createElement("th", null, "Base / aplicaci\u00F3n"),
                            React.createElement("th", null, "Usuario"),
                            React.createElement("th", null, "Aprovisionamiento"),
                            React.createElement("th", null, "Conexi\u00F3n y datos"))),
                    React.createElement("tbody", null, r.databases.map(db => React.createElement("tr", { key: db.id },
                        React.createElement("td", null,
                            React.createElement("strong", null, db.name),
                            React.createElement("br", null),
                            React.createElement("small", { className: "mono" }, db.id)),
                        React.createElement("td", { className: "mono" }, db.username),
                        React.createElement("td", null,
                            { ready: 'Lista', pending: 'Pendiente', failed: 'Falló' }[db.state] || db.state,
                            db.error && React.createElement("span", { className: "warning-text" }, db.error.message)),
                        React.createElement("td", null,
                            React.createElement("div", { className: "button-row" },
                                React.createElement("button", { onClick: () => setConnection(db) }, "Credenciales"),
                                React.createElement("button", { disabled: !canMutate, onClick: () => void action({ action: 'check', database: db.id }) }, "Probar conexi\u00F3n"),
                                React.createElement("button", { disabled: !canMutate || db.state !== 'ready', onClick: () => setLink(db) }, "Vincular proyecto"),
                                db.state !== 'ready' && React.createElement("button", { disabled: !canMutate, onClick: () => void action({ action: 'database', instance: r.id, name: db.name, username: db.username, confirm: true }) }, "Reintentar creaci\u00F3n"),
                                r.engine !== 'redis' && React.createElement(React.Fragment, null,
                                    React.createElement("button", { disabled: !canMutate, onClick: () => setBackup({ db, restore: false }) }, "Exportar"),
                                    React.createElement("button", { disabled: !canMutate, onClick: () => setBackup({ db, restore: true }) }, "Restaurar"))))))))),
            !!r.consumers.length && React.createElement("div", { className: "infra-bindings" },
                React.createElement("h3", null, "Vinculaciones"),
                r.consumers.map(b => React.createElement("div", { className: "binding-row", key: b.id },
                    React.createElement("span", null,
                        React.createElement("strong", null, b.stack || b.stackUid),
                        " \u00B7 ",
                        b.mode,
                        " \u00B7 ",
                        b.services.join(', '),
                        React.createElement("br", null),
                        React.createElement("small", null,
                            "Variables: ",
                            Object.values(b.mapping).join(', '),
                            " \u00B7 ID ",
                            b.id)),
                    React.createElement("button", { disabled: !canMutate, onClick: () => void action({ action: 'check-binding', binding: b.id }) }, "Comprobar red y variables"),
                    React.createElement("button", { disabled: !canMutate, onClick: () => {
                            if (window.confirm('Se quitará la vinculación del catálogo, no la base ni sus datos. Revisa e inicia el proyecto después para aplicar.'))
                                void action({ action: 'unbind', binding: b.id, confirm: true });
                        } }, "Desvincular")))),
            r.engine === 'redis' && React.createElement("p", { className: "hint" }, "AOF activado cuando eliges persistencia. Sin exportaci\u00F3n RDB autom\u00E1tica en esta versi\u00F3n: consulta docs/INFRAESTRUCTURA.md para la copia en fr\u00EDo y su restauraci\u00F3n conservadora."),
            React.createElement("details", null,
                React.createElement("summary", null, "Imagen fijada y l\u00EDmites de la persistencia"),
                React.createElement("p", { className: "mono" },
                    r.resolvedImage || 'Pendiente de descargar/fijar',
                    " \u00B7 ",
                    r.platform || 'Plataforma sin comprobar'),
                React.createElement("p", null, "La carpeta, imagen y puerto no se cambian sobre datos existentes. Para otra versi\u00F3n crea una instancia y restaura un backup compatible; no se hace una migraci\u00F3n autom\u00E1tica ni se borra el origen.")))),
        create && data && React.createElement(CreateInstance, { data: data, onClose: () => setCreate(false), onReview: (body, value) => { setCreate(false); setReview({ body: { ...body, action: 'create' }, value }); } }),
        dbFor && React.createElement(CreateDatabase, { instance: dbFor, onClose: () => setDbFor(null), onSubmit: body => { setDbFor(null); void action(body); } }),
        connection && React.createElement(ConnectionDialog, { database: connection, onClose: () => setConnection(null) }),
        link && React.createElement(BindingDialog, { database: link, engine: data?.instances.find(r => r.uid === link.instanceUid)?.engine || 'postgres', stacks: catalog.stacks, onClose: () => setLink(null), onReview: (body, value) => { setLink(null); setReview({ value, body: { ...body, action: 'bind' } }); } }),
        backup && React.createElement(BackupDialog, { ...backup, onClose: () => setBackup(null), onSubmit: body => { setBackup(null); void action(body); } }),
        logs && React.createElement(InfraLogs, { instance: logs, onClose: () => setLogs(null) }),
        review && React.createElement(Review, { value: review.value, busy: busy, onClose: () => setReview(null), onConfirm: () => void action({ ...review.body, fingerprint: review.value.fingerprint, confirm: true }) }));
}
function CreateInstance({ data, onClose, onReview }) {
    const [engine, setEngine] = useState('postgres'), [name, setName] = useState('PostgreSQL principal'), [id, setId] = useState('pg-main'), [version, setVersion] = useState(data.engines.postgres.defaultVersion), [kind, setKind] = useState('volume'), [folder, setFolder] = useState(`${data.defaultDataRoot}/postgres/pg-main`), [memory, setMemory] = useState('384'), [max, setMax] = useState('32');
    const [publish, setPublish] = useState(false), [port, setPort] = useState('15432'), [from, setFrom] = useState('15432'), [to, setTo] = useState('15442'), [ports, setPorts] = useState(null), [portLoading, setPortLoading] = useState(false), [busy, setBusy] = useState(false), [error, setError] = useState('');
    const def = data.engines[engine];
    async function scan() {
        setPortLoading(true);
        setError('');
        try {
            const value = await api('/infra/ports', { start: Number(from), end: Number(to) });
            setPorts(value.ports);
        }
        catch (e) {
            setError(message(e));
        }
        finally {
            setPortLoading(false);
        }
    }
    async function submit() {
        setBusy(true);
        setError('');
        try {
            const body = { engine, id, name, image: `${engine}:${version}`, persistence: { kind, ...(kind === 'folder' ? { path: folder } : {}) }, hostPort: publish ? Number(port) : null, memoryMiB: Number(memory), maxConnections: Number(max) };
            onReview(body, await api('/infra/preview', body));
        }
        catch (e) {
            setError(message(e));
        }
        finally {
            setBusy(false);
        }
    }
    return React.createElement(Modal, { title: "Crear instancia de datos", subtitle: "Una instancia puede contener varias bases PostgreSQL/MySQL, cada una con su cuenta", onClose: onClose, wide: true },
        React.createElement("form", { onSubmit: e => { e.preventDefault(); void submit(); } },
            React.createElement("div", { className: "form-grid" },
                React.createElement(Field, { label: "Motor", help: "Traefik no es un motor de datos: est\u00E1 en Accesos locales." },
                    React.createElement("select", { value: engine, onChange: e => {
                            const v = e.target.value, d = data.engines[v];
                            setEngine(v);
                            setVersion(d.defaultVersion);
                            setName(`${d.name} principal`);
                            setId(`${v}-main`);
                            setFolder(`${data.defaultDataRoot}/${v}/${v}-main`);
                            setMemory(String(d.memoryMiB));
                            setPort(String(d.suggestedPort));
                            setFrom(String(d.suggestedPort));
                            setTo(String(d.suggestedPort + 10));
                            setPorts(null);
                            if (v !== 'redis' && kind === 'none')
                                setKind('volume');
                        } }, Object.entries(data.engines).map(([k, d]) => React.createElement("option", { key: k, value: k }, d.name)))),
                React.createElement(Field, { label: "Familia de versi\u00F3n", help: "Familias admitidas. Se descarga la imagen oficial nativa y se fija su digest; no cambia de versi\u00F3n al reiniciar." },
                    React.createElement("select", { value: version, onChange: e => setVersion(e.target.value) }, def.versions.map(v => React.createElement("option", { key: v }, v)))),
                React.createElement(Field, { label: "Nombre visible", help: "Etiqueta que reconocer\u00E1s en el panel." },
                    React.createElement("input", { required: true, value: name, onChange: e => setName(e.target.value) })),
                React.createElement(Field, { label: "ID estable", help: "Usado por nearprod infra. No es el nombre de una base ni una URL web." },
                    React.createElement("input", { required: true, pattern: "[a-z0-9][a-z0-9-]*", value: id, onChange: e => setId(e.target.value) }))),
            React.createElement(Field, { label: "Persistencia", help: "Se aplica solo al recurso que est\u00E1s creando; no modifica la configuraci\u00F3n de proyectos importados." },
                React.createElement("select", { value: kind, onChange: e => setKind(e.target.value) },
                    React.createElement("option", { value: "volume" }, "Volumen Docker \u2014 dentro de la VM"),
                    React.createElement("option", { value: "folder" }, "Carpeta local dedicada \u2014 visible en tu Mac"),
                    engine === 'redis' && React.createElement("option", { value: "none" }, "Sin persistencia \u2014 cach\u00E9 desechable"))),
            kind === 'folder' && React.createElement(React.Fragment, null,
                React.createElement(Field, { label: "Carpeta local de esta instancia", help: "Debe estar vac\u00EDa o no existir, no ser enlace simb\u00F3lico y estar compartida con Colima. Se crea data/; se comprueban los permisos desde un contenedor. No uses iCloud ni una carpeta con otros archivos." },
                    React.createElement("input", { required: true, value: folder, onChange: e => setFolder(e.target.value) })),
                React.createElement(Alert, null, "Todas las bases l\u00F3gicas de esta instancia usan el mismo directorio f\u00EDsico. Para una carpeta por base debes crear otra instancia. La exportaci\u00F3n de cada base s\u00ED produce un archivo independiente. El rendimiento de un bind mount puede ser distinto al de un volumen dentro de la VM.")),
            React.createElement("label", { className: "check-label" },
                React.createElement("input", { type: "checkbox", checked: publish, onChange: e => setPublish(e.target.checked) }),
                " Permitir acceso desde una herramienta en mi equipo (127.0.0.1)"),
            React.createElement("p", { className: "field-help" }, "Los contenedores conectados a la red de datos no necesitan publicar puertos del host."),
            publish && React.createElement("section", { className: "panel" },
                React.createElement("div", { className: "form-grid" },
                    React.createElement(Field, { label: "Puerto local elegido", help: `El motor escucha internamente en ${def.port}; este número es solo para tu Mac.` },
                        React.createElement("input", { required: true, type: "number", min: "1024", max: "65535", value: port, onChange: e => setPort(e.target.value) })),
                    React.createElement("div", null,
                        React.createElement("label", null, "Rango a consultar (m\u00E1ximo 128 puertos)"),
                        React.createElement("div", { className: "button-row" },
                            React.createElement("input", { "aria-label": "Puerto inicial", type: "number", value: from, onChange: e => setFrom(e.target.value) }),
                            React.createElement("input", { "aria-label": "Puerto final", type: "number", value: to, onChange: e => setTo(e.target.value) }),
                            React.createElement("button", { type: "button", disabled: portLoading, onClick: () => void scan() }, "Buscar libres")))),
                portLoading && React.createElement(Skeleton, { label: "Comprobando puertos", rows: 2 }),
                React.createElement("div", { className: "port-options" }, ports?.map(p => React.createElement("button", { type: "button", key: p.port, disabled: !p.available, className: port === String(p.port) ? 'primary' : '', title: p.reason, onClick: () => setPort(String(p.port)) },
                    p.port,
                    " \u00B7 ",
                    p.available ? 'libre' : 'ocupado'))),
                React.createElement("p", { className: "hint" }, "La consulta no reserva puertos. Se vuelven a comprobar al confirmar e iniciar; otro proceso todav\u00EDa puede ocuparlos.")),
            React.createElement("details", null,
                React.createElement("summary", null, "Recursos y conexiones"),
                React.createElement("div", { className: "form-grid" },
                    React.createElement(Field, { label: "L\u00EDmite de memoria (MiB)", help: "No es una reserva. Debe quedar RAM para Linux, Traefik y tus aplicaciones; bajar demasiado provoca OOM." },
                        React.createElement("input", { required: true, type: "number", min: def.minMemoryMiB, value: memory, onChange: e => setMemory(e.target.value) })),
                    engine !== 'redis' && React.createElement(Field, { label: "Conexiones m\u00E1ximas", help: "Ajusta tambi\u00E9n los pools de API/workers para no agotarlas." },
                        React.createElement("input", { required: true, type: "number", min: "8", max: "300", value: max, onChange: e => setMax(e.target.value) })))),
            error && React.createElement(Alert, { error: true }, error),
            React.createElement("div", { className: "modal-actions" },
                React.createElement("button", { type: "button", onClick: onClose }, "Cancelar"),
                React.createElement("button", { className: "primary", disabled: busy }, busy ? 'Comprobando…' : 'Revisar creación'))));
}
function CreateDatabase({ instance, onClose, onSubmit }) { const [name, setName] = useState(''), [username, setUsername] = useState(''); return React.createElement(Modal, { title: instance.engine === 'redis' ? 'Credencial de aplicación Redis' : 'Crear base y usuario', subtitle: instance.name, onClose: onClose },
    React.createElement("form", { onSubmit: e => { e.preventDefault(); onSubmit({ action: 'database', instance: instance.id, name, ...(username ? { username } : {}), confirm: true }); } },
        React.createElement(Field, { label: "Nombre de la base / aplicaci\u00F3n", help: "Min\u00FAsculas, n\u00FAmeros y underscore; empieza por letra. PostgreSQL/MySQL crean una base independiente, no un schema dentro de una base com\u00FAn." },
            React.createElement("input", { required: true, pattern: "[a-z][a-z0-9_]{0,39}", value: name, onChange: e => setName(e.target.value), placeholder: "tienda_dev" })),
        React.createElement(Field, { label: "Usuario (opcional)", help: "Se propone uno si lo dejas vac\u00EDo; la contrase\u00F1a se genera de forma segura. No se entregan permisos de administrador." },
            React.createElement("input", { pattern: "[a-z][a-z0-9_]{0,39}", value: username, onChange: e => setUsername(e.target.value), placeholder: "Generado por NearProd" })),
        React.createElement(Alert, null, "La operaci\u00F3n crea datos reales y prueba autenticaci\u00F3n y lectura/escritura. No migra la informaci\u00F3n de otras bases. Para pruebas destructivas crea una base separada, por ejemplo tienda_verify."),
        React.createElement("div", { className: "modal-actions" },
            React.createElement("button", { type: "button", onClick: onClose }, "Cancelar"),
            React.createElement("button", { className: "primary" }, "Crear y comprobar")))); }
function ConnectionDialog({ database, onClose }) {
    const [value, setValue] = useState(null), [error, setError] = useState('');
    async function load(reveal = false) {
        try {
            setValue(await api('/infra/connection', { database: database.id, reveal }));
        }
        catch (e) {
            setError(message(e));
        }
    }
    useEffect(() => { void load(); }, [database.id]);
    useEffect(() => {
        if (!value?.revealed)
            return;
        const t = setTimeout(() => void load(), 30000);
        return () => clearTimeout(t);
    }, [value?.revealed]);
    return React.createElement(Modal, { title: "Datos de conexi\u00F3n", subtitle: database.name, onClose: onClose, wide: true },
        error && React.createElement(Alert, { error: true }, error),
        !value && React.createElement(Skeleton, { label: "Leyendo conexi\u00F3n", rows: 5 }),
        " ",
        value && React.createElement(React.Fragment, null,
            React.createElement(Alert, null, value.note),
            React.createElement("div", { className: "form-grid" },
                React.createElement(Field, { label: "Usuario" },
                    React.createElement("input", { readOnly: true, value: value.user })),
                React.createElement(Field, { label: "Contrase\u00F1a", help: "Se oculta de nuevo despu\u00E9s de 30 segundos. No se env\u00EDa al historial de operaciones." },
                    React.createElement("input", { type: value.revealed ? 'text' : 'password', readOnly: true, value: value.password }))),
            React.createElement("button", { onClick: () => void load(!value.revealed) }, value.revealed ? 'Ocultar secreto' : 'Revelar credencial'),
            React.createElement(Field, { label: "Dentro de Docker", help: "Usa este host desde el backend/worker conectado. localhost dentro de ese contenedor no es la base." },
                React.createElement("input", { readOnly: true, value: value.internal.url })),
            React.createElement(Field, { label: "Desde este equipo", help: "Este puerto sirve para un cliente SQL/Redis local. No es una URL de navegador ni una ruta Traefik." },
                React.createElement("input", { readOnly: true, value: value.local?.url || 'No publicado: solo conexión interna Docker' }))));
}
function BindingDialog({ database, engine, stacks, onClose, onReview }) {
    const [target, setTarget] = useState(''), [mode, setMode] = useState('dev'), [services, setServices] = useState([]), [choices, setChoices] = useState([]), [format, setFormat] = useState('url'), [vars, setVars] = useState({ url: engine === 'redis' ? 'REDIS_URL' : 'DATABASE_URL', host: 'DB_HOST', port: 'DB_PORT', database: 'DB_DATABASE', user: 'DB_USERNAME', password: 'DB_PASSWORD' }), [error, setError] = useState(''), [busy, setBusy] = useState(false), [loading, setLoading] = useState(false);
    const stack = stacks.find(s => s.id === target);
    useEffect(() => {
        setChoices([]);
        setServices([]);
        if (!stack)
            return;
        const controller = new AbortController();
        setLoading(true);
        const files = stack.modes[mode]?.files || [];
        void api('/project-options', { path: stack.path, files }, controller.signal).then(o => setChoices(o.services)).catch(e => {
            if (!controller.signal.aborted)
                setError(message(e));
        }).finally(() => setLoading(false));
        return () => controller.abort();
    }, [target, mode]);
    async function submit() {
        setBusy(true);
        try {
            const mapping = Object.fromEntries(Object.entries(vars).filter(([k]) => format === 'url' ? k === 'url' : k !== 'url'));
            const body = { target, database: database.id, mode, services, mapping };
            onReview(body, await api('/infra/binding-preview', body));
        }
        catch (e) {
            setError(message(e));
        }
        finally {
            setBusy(false);
        }
    }
    return React.createElement(Modal, { title: "Vincular con un proyecto", subtitle: `${database.name} · solo se aplica después de revisar e iniciar el proyecto`, onClose: onClose, wide: true },
        React.createElement("form", { onSubmit: e => { e.preventDefault(); void submit(); } },
            React.createElement(Field, { label: "Aplicaci\u00F3n registrada", help: "No se elimina la base que ya pueda existir en su Compose; el usuario decide cu\u00E1l utilizar." },
                React.createElement("select", { required: true, value: target, onChange: e => { setTarget(e.target.value); setMode('dev'); } },
                    React.createElement("option", { value: "" }, "Selecciona aplicaci\u00F3n"),
                    stacks.map(s => React.createElement("option", { key: s.id }, s.id)))),
            React.createElement(Field, { label: "Modo", help: "Desarrollo y prueba de imagen pueden apuntar a bases distintas. Este v\u00EDnculo no separa datos por s\u00ED solo." },
                React.createElement("select", { value: mode, onChange: e => setMode(e.target.value) },
                    React.createElement("option", { value: "dev" }, "Desarrollo"),
                    stack?.modes.verify && React.createElement("option", { value: "verify" }, "Prueba de imagen"))),
            React.createElement("fieldset", null,
                React.createElement("legend", null, "Servicios consumidores"),
                React.createElement("p", { className: "hint" }, "Selecciona API y workers. No env\u00EDes contrase\u00F1as a c\u00F3digo de frontend servido al navegador."),
                loading && React.createElement(Skeleton, { label: "Leyendo servicios Compose", rows: 2 }),
                " ",
                choices.map(s => React.createElement("label", { className: "check-label", key: s },
                    React.createElement("input", { type: "checkbox", checked: services.includes(s), onChange: e => setServices(e.target.checked ? [...services, s] : services.filter(v => v !== s)) }),
                    s))),
            React.createElement(Field, { label: "Formato que espera tu aplicaci\u00F3n", help: "Comprueba los nombres en su configuraci\u00F3n. NearProd entrega variables, no modifica el c\u00F3digo del framework." },
                React.createElement("select", { value: format, onChange: e => setFormat(e.target.value) },
                    React.createElement("option", { value: "url" }, "Una URL de conexi\u00F3n"),
                    React.createElement("option", { value: "fields" }, "Campos separados"))),
            React.createElement("div", { className: "form-grid" }, Object.keys(vars).filter(k => format === 'url' ? k === 'url' : k !== 'url').map(k => React.createElement(Field, { key: k, label: `Variable para ${k}` },
                React.createElement("input", { required: true, pattern: "[A-Z][A-Z0-9_]*", value: vars[k], onChange: e => setVars({ ...vars, [k]: e.target.value }) })))),
            error && React.createElement(Alert, { error: true }, error),
            React.createElement("div", { className: "modal-actions" },
                React.createElement("button", { type: "button", onClick: onClose }, "Cancelar"),
                React.createElement("button", { className: "primary", disabled: busy || !services.length }, "Revisar vinculaci\u00F3n"))));
}
function BackupDialog({ db, restore, onClose, onSubmit }) { const [value, setValue] = useState(''), [trusted, setTrusted] = useState(false); return React.createElement(Modal, { title: restore ? 'Restaurar en base vacía' : 'Exportar backup lógico', subtitle: db.name, onClose: onClose },
    React.createElement("form", { onSubmit: e => { e.preventDefault(); onSubmit({ action: restore ? 'restore' : 'backup', database: db.id, confirm: true, ...(restore ? { file: value, trustedBackup: trusted } : { directory: value || undefined }) }); } },
        React.createElement(Alert, null, "Un volumen o carpeta persistente no es un backup. El archivo puede contener datos sensibles: se guarda con permisos privados fuera de la VM. El resultado y la ruta aparecen en Actividad."),
        React.createElement(Field, { label: restore ? 'Archivo de backup' : 'Carpeta de destino (opcional)', help: restore ? 'Selecciona un dump creado por NearProd con su archivo .nearprod.json al lado. Debe corresponder al motor y versión compatibles.' : 'Vacío usa ~/.nearprod/backups/databases (o tu NEARPROD_HOME). No se sobrescriben archivos existentes.' },
            React.createElement("input", { required: restore, value: value, onChange: e => setValue(e.target.value), placeholder: restore ? '/Users/usuario/Backups/tienda.dump' : '/Users/usuario/Backups' })),
        restore && React.createElement(React.Fragment, null,
            React.createElement(Alert, { error: true }, "Solo se acepta una base vac\u00EDa y sin v\u00EDnculos. Un fallo puede dejar objetos parciales; no hay rollback autom\u00E1tico. MySQL no restaura rutinas/eventos y puede rechazar definers ajenos."),
            React.createElement("label", { className: "check-label" },
                React.createElement("input", { required: true, type: "checkbox", checked: trusted, onChange: e => setTrusted(e.target.checked) }),
                " Conf\u00EDo en este backup y acepto importarlo en este destino vac\u00EDo.")),
        React.createElement("div", { className: "modal-actions" },
            React.createElement("button", { type: "button", onClick: onClose }, "Cancelar"),
            React.createElement("button", { className: "primary" }, restore ? 'Restaurar como usuario limitado' : 'Exportar')))); }
function InfraLogs({ instance, onClose }) {
    const [lines, setLines] = useState([]), [error, setError] = useState('');
    useEffect(() => {
        const stream = new EventSource(`/api/infra/logs?instance=${encodeURIComponent(instance.id)}`);
        stream.addEventListener('line', e => {
            const value = JSON.parse(e.data);
            setLines(l => [...l, value.text].slice(-500));
        });
        stream.addEventListener('log-error', e => setError(JSON.parse(e.data).message));
        stream.onerror = () => setError('Reconectando; pueden existir repeticiones o huecos.');
        return () => stream.close();
    }, [instance.id]);
    return React.createElement(Modal, { title: "Logs de infraestructura", subtitle: instance.name, onClose: onClose, wide: true },
        error && React.createElement(Alert, { error: true }, error),
        React.createElement("p", { className: "hint" }, "M\u00E1ximo 500 l\u00EDneas visibles. Pueden contener datos sensibles. Cerrar solo libera el seguimiento, no detiene el motor."),
        React.createElement("pre", { className: "console infra-console" }, lines.join('\n') || 'Conectando…'));
}
