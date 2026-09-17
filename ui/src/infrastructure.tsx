import { api, message } from './api.js';
import { Alert, Badge, Field, Modal, Skeleton, Icon } from './components.js';
import type { Catalog, Operation, ProjectOptions, Stack, Status } from './types.js';
const { useState, useEffect } = React;
type Engine = 'postgres' | 'mysql' | 'redis';
interface Database {
    id: string;
    name: string;
    username: string;
    state: string;
    instanceUid: string;
    error?: {
        message: string;
    };
}
interface Binding {
    id: string;
    databaseId: string;
    stackUid: string;
    mode: string;
    services: string[];
    mapping: Record<string, string>;
    stack?: string;
}
interface Instance {
    id: string;
    uid: string;
    name: string;
    engine: Engine;
    requestedImage: string;
    resolvedImage?: string;
    platform?: string;
    hostPort: number | null;
    memoryMiB: number;
    maxConnections: number;
    execution: string;
    health: string;
    location: string;
    internalHost: string;
    internalPort: number;
    databases: Database[];
    consumers: Binding[];
    persistence: {
        kind: string;
        path?: string;
    };
}
interface Infra {
    defaultDataRoot: string;
    instances: Instance[];
    bindings: Binding[];
    connected: boolean;
    checkedAt: string | null;
    note: string;
    engines: Record<Engine, {
        name: string;
        versions: string[];
        defaultVersion: string;
        port: number;
        suggestedPort: number;
        memoryMiB: number;
        minMemoryMiB: number;
    }>;
}
interface Connection {
    databaseId: string;
    engine: string;
    database: string | number;
    user: string;
    password: string;
    internal: {
        host: string;
        port: number;
        url: string;
    };
    local: {
        host: string;
        port: number;
        url: string;
    } | null;
    revealed: boolean;
    note: string;
}
interface Preview {
    fingerprint: string;
    warnings?: string[];
    note?: string;
    [key: string]: unknown;
}
type Data = Record<string, unknown>;
interface Props {
    catalog: Catalog;
    status: Status;
    notify: (m: string, error?: boolean) => void;
}
function Review({ value, onConfirm, onClose, busy }: {
    value: Preview;
    onConfirm: () => void;
    onClose: () => void;
    busy: boolean;
}) {
    const def = value.definition as {
        name: string;
        engine: string;
        requestedImage: string;
        hostPort: number | null;
        memoryMiB: number;
        persistence: {
            kind: string;
            path?: string;
        };
    } | undefined;
    const consumers = value.consumers as {
        stack: string;
        active: boolean;
    }[] | undefined;
    const services = value.services as string[] | undefined;
    return <Modal title="Revisar cambio de infraestructura" subtitle="Solo se aplicará cuando confirmes" onClose={onClose} wide>
    {def && <section className="panel"><h3>{def.name} · {def.requestedImage}</h3><p><strong>Persistencia:</strong> {def.persistence.kind === 'folder' ? def.persistence.path + '/data' : def.persistence.kind === 'volume' ? 'Volumen Docker dentro de la VM' : 'Sin persistencia'}</p><p><strong>Desde tu equipo:</strong> {def.hostPort ? `127.0.0.1:${def.hostPort}` : 'Puerto no publicado'}</p><p><strong>Límite:</strong> {def.memoryMiB} MiB. Compartido con el resto de la VM.</p></section>}
    {services && <section className="panel"><h3>Conectar {String(value.target)} a {String(value.instance)}</h3><p>Servicios consumidores: {services.join(', ')}.</p><p>Destino interno: <code>{String(value.host)}:{String(value.port)}</code>. Las contraseñas no se muestran en esta revisión.</p></section>}
    {consumers && <section className="panel"><h3>Detener {String(value.instance)}</h3>{consumers.length ? consumers.map((c, i) => <p key={i}>{c.stack} · {c.active ? 'Consumidor activo' : 'No observado en ejecución'}</p>) : <p>No hay consumidores registrados. Puede haber clientes externos.</p>}<p>Se detiene el motor, no se eliminan sus datos.</p></section>}
    {value.warnings?.map((w, i) => <Alert key={i}>{w}</Alert>)}{value.note && <p>{value.note}</p>}
    <details><summary>Detalles técnicos de lo que se aplicará</summary><pre className="console review-json">{JSON.stringify(value, null, 2)}</pre></details>
    <div className="modal-actions"><button onClick={onClose}>Volver</button><button className="primary" disabled={busy} onClick={onConfirm}>Confirmar y aplicar</button></div>
  </Modal>;
}
export function InfrastructureView({ catalog, status, notify }: Props) {
    const [data, setData] = useState<Infra | null>(null), [error, setError] = useState(''), [loading, setLoading] = useState(false), [busy, setBusy] = useState(false);
    const [create, setCreate] = useState(false), [dbFor, setDbFor] = useState<Instance | null>(null), [connection, setConnection] = useState<Database | null>(null), [link, setLink] = useState<Database | null>(null), [logs, setLogs] = useState<Instance | null>(null), [backup, setBackup] = useState<{
        db: Database;
        restore: boolean;
    } | null>(null);
    const [review, setReview] = useState<{
        value: Preview;
        body: Data;
    } | null>(null);
    const active = catalog.operations.some(o => o.state === 'running');
    async function load() { setLoading(true); try {
        setData(await api<Infra>('/infra'));
        setError('');
    }
    catch (e) {
        setError(message(e));
    }
    finally {
        setLoading(false);
    } }
    useEffect(() => { void load(); }, [status.checkedAt, catalog.operations]);
    async function action(body: Data) { setBusy(true); try {
        const op = await api<Operation>('/infra/actions', body);
        notify(`Operación ${op.id} iniciada. Sigue su resultado en Actividad.`);
        setReview(null);
        await load();
    }
    catch (e) {
        setError(message(e));
    }
    finally {
        setBusy(false);
    } }
    const canMutate = status.connected && !active && !busy;
    return <><div className="section-heading"><div><span className="eyebrow">SERVICIOS PARA TUS APLICACIONES</span><h1>Infraestructura compartida</h1><p>Un motor, varias bases independientes. Enciende solo lo necesario.</p></div><div className="button-row"><button onClick={() => void load()} disabled={loading}><Icon name="refresh"/>Actualizar</button><button className="primary" disabled={!canMutate} onClick={() => setCreate(true)}>Crear instancia</button></div></div>
    <Alert>PostgreSQL/MySQL separan bases y usuarios por proyecto. Redis se crea dedicado a una aplicación. <strong>Traefik es un componente central</strong> y se gestiona en Accesos locales, no en esta lista. No se cambia la persistencia de los proyectos que registres.</Alert>
    {!status.connected && <Alert error>Docker no está conectado. Puedes ver el catálogo; inicia/comprueba Colima desde Runtime antes de crear o ejecutar infraestructura.</Alert>}
    {error && <Alert error>{error}</Alert>}{!data && loading && <Skeleton label="Consultando infraestructura" rows={6}/>} {data && loading && <p className="hint" role="status">Actualizando… se conserva la última información.</p>}
    {data && !data.instances.length && <section className="empty"><Icon name="cube" size={32}/><h2>Tus servicios reutilizables</h2><p>Crea PostgreSQL o MySQL, añade una base y vincula los backends que deban utilizarla.<br />La base que un proyecto ya declara en su Compose no se elimina ni sustituye automáticamente.</p><button disabled={!canMutate} onClick={() => setCreate(true)}>Crear mi primera instancia</button></section>}
    {data?.instances.map(r => <section className="panel infra-instance" key={r.uid}><div className="panel-heading"><Icon name="cube"/><div><h2>{r.name}</h2><span className="mono muted">{r.id} · {r.requestedImage}</span></div><Badge value={r.execution}/><Badge value={r.health}/></div><div className="infra-facts"><div><span>Persistencia de la instancia</span><code>{r.location}</code><small>Compartida físicamente por sus bases; no es un backup.</small></div><div><span>Conexión dentro de Docker</span><code>{r.internalHost}:{r.internalPort}</code><small>Solo servicios unidos a esta red de datos.</small></div><div><span>Acceso desde tu equipo</span><code>{r.hostPort ? `127.0.0.1:${r.hostPort}` : 'No publicado'}</code><small>Límite de contenedor: {r.memoryMiB} MiB; no reserva RAM.</small></div></div><div className="button-row"><button disabled={!canMutate} onClick={() => void action({ action: 'start', instance: r.id })}>Iniciar / comprobar</button><button disabled={!canMutate} onClick={() => { void api<Preview>('/infra/stop-preview', { instance: r.id }).then(value => setReview({ value, body: { action: 'stop', instance: r.id, allowActive: window.confirm('Detener una instancia puede interrumpir TODOS sus consumidores. Se mostrará una revisión antes de aplicar. ¿Permitir detenerla aunque haya consumidores activos?') } })).catch(e => setError(message(e))); }}>Detener instancia</button><button disabled={!status.connected} onClick={() => setLogs(r)}>Logs</button><button disabled={!canMutate || (r.engine === 'redis' && r.databases.length > 0)} onClick={() => setDbFor(r)}>{r.engine === 'redis' ? 'Crear credencial de aplicación' : 'Crear base + usuario'}</button></div>
      <div className="table-wrap"><table><thead><tr><th>Base / aplicación</th><th>Usuario</th><th>Aprovisionamiento</th><th>Conexión y datos</th></tr></thead><tbody>{r.databases.map(db => <tr key={db.id}><td><strong>{db.name}</strong><br /><small className="mono">{db.id}</small></td><td className="mono">{db.username}</td><td>{{ ready: 'Lista', pending: 'Pendiente', failed: 'Falló' }[db.state] || db.state}{db.error && <span className="warning-text">{db.error.message}</span>}</td><td><div className="button-row"><button onClick={() => setConnection(db)}>Credenciales</button><button disabled={!canMutate} onClick={() => void action({ action: 'check', database: db.id })}>Probar conexión</button><button disabled={!canMutate || db.state !== 'ready'} onClick={() => setLink(db)}>Vincular proyecto</button>{db.state !== 'ready' && <button disabled={!canMutate} onClick={() => void action({ action: 'database', instance: r.id, name: db.name, username: db.username, confirm: true })}>Reintentar creación</button>}{r.engine !== 'redis' && <><button disabled={!canMutate} onClick={() => setBackup({ db, restore: false })}>Exportar</button><button disabled={!canMutate} onClick={() => setBackup({ db, restore: true })}>Restaurar</button></>}</div></td></tr>)}</tbody></table></div>
      {!!r.consumers.length && <div className="infra-bindings"><h3>Vinculaciones</h3>{r.consumers.map(b => <div className="binding-row" key={b.id}><span><strong>{b.stack || b.stackUid}</strong> · {b.mode} · {b.services.join(', ')}<br /><small>Variables: {Object.values(b.mapping).join(', ')} · ID {b.id}</small></span><button disabled={!canMutate} onClick={() => void action({ action: 'check-binding', binding: b.id })}>Comprobar red y variables</button><button disabled={!canMutate} onClick={() => { if (window.confirm('Se quitará la vinculación del catálogo, no la base ni sus datos. Revisa e inicia el proyecto después para aplicar.'))
            void action({ action: 'unbind', binding: b.id, confirm: true }); }}>Desvincular</button></div>)}</div>}
      {r.engine === 'redis' && <p className="hint">AOF activado cuando eliges persistencia. Sin exportación RDB automática en esta versión: consulta docs/INFRAESTRUCTURA.md para la copia en frío y su restauración conservadora.</p>}
      <details><summary>Imagen fijada y límites de la persistencia</summary><p className="mono">{r.resolvedImage || 'Pendiente de descargar/fijar'} · {r.platform || 'Plataforma sin comprobar'}</p><p>La carpeta, imagen y puerto no se cambian sobre datos existentes. Para otra versión crea una instancia y restaura un backup compatible; no se hace una migración automática ni se borra el origen.</p></details>
    </section>)}
    {create && data && <CreateInstance data={data} onClose={() => setCreate(false)} onReview={(body, value) => { setCreate(false); setReview({ body: { ...body, action: 'create' }, value }); }}/>}
    {dbFor && <CreateDatabase instance={dbFor} onClose={() => setDbFor(null)} onSubmit={body => { setDbFor(null); void action(body); }}/>}
    {connection && <ConnectionDialog database={connection} onClose={() => setConnection(null)}/>}
    {link && <BindingDialog database={link} engine={data?.instances.find(r => r.uid === link.instanceUid)?.engine || 'postgres'} stacks={catalog.stacks} onClose={() => setLink(null)} onReview={(body, value) => { setLink(null); setReview({ value, body: { ...body, action: 'bind' } }); }}/>}
    {backup && <BackupDialog {...backup} onClose={() => setBackup(null)} onSubmit={body => { setBackup(null); void action(body); }}/>}
    {logs && <InfraLogs instance={logs} onClose={() => setLogs(null)}/>}
    {review && <Review value={review.value} busy={busy} onClose={() => setReview(null)} onConfirm={() => void action({ ...review.body, fingerprint: review.value.fingerprint, confirm: true })}/>}
  </>;
}
function CreateInstance({ data, onClose, onReview }: {
    data: Infra;
    onClose: () => void;
    onReview: (body: Data, value: Preview) => void;
}) {
    const [engine, setEngine] = useState<Engine>('postgres'), [name, setName] = useState('PostgreSQL principal'), [id, setId] = useState('pg-main'), [version, setVersion] = useState(data.engines.postgres.defaultVersion), [kind, setKind] = useState('volume'), [folder, setFolder] = useState(`${data.defaultDataRoot}/postgres/pg-main`), [memory, setMemory] = useState('384'), [max, setMax] = useState('32');
    const [publish, setPublish] = useState(false), [port, setPort] = useState('15432'), [from, setFrom] = useState('15432'), [to, setTo] = useState('15442'), [ports, setPorts] = useState<{
        port: number;
        available: boolean;
        reason: string;
    }[] | null>(null), [portLoading, setPortLoading] = useState(false), [busy, setBusy] = useState(false), [error, setError] = useState('');
    const def = data.engines[engine];
    async function scan() { setPortLoading(true); setError(''); try {
        const value = await api<{
            ports: {
                port: number;
                available: boolean;
                reason: string;
            }[];
            note: string;
        }>('/infra/ports', { start: Number(from), end: Number(to) });
        setPorts(value.ports);
    }
    catch (e) {
        setError(message(e));
    }
    finally {
        setPortLoading(false);
    } }
    async function submit() { setBusy(true); setError(''); try {
        const body = { engine, id, name, image: `${engine}:${version}`, persistence: { kind, ...(kind === 'folder' ? { path: folder } : {}) }, hostPort: publish ? Number(port) : null, memoryMiB: Number(memory), maxConnections: Number(max) };
        onReview(body, await api<Preview>('/infra/preview', body));
    }
    catch (e) {
        setError(message(e));
    }
    finally {
        setBusy(false);
    } }
    return <Modal title="Crear instancia de datos" subtitle="Una instancia puede contener varias bases PostgreSQL/MySQL, cada una con su cuenta" onClose={onClose} wide><form onSubmit={e => { e.preventDefault(); void submit(); }}><div className="form-grid"><Field label="Motor" help="Traefik no es un motor de datos: está en Accesos locales."><select value={engine} onChange={e => { const v = e.target.value as Engine, d = data.engines[v]; setEngine(v); setVersion(d.defaultVersion); setName(`${d.name} principal`); setId(`${v}-main`); setFolder(`${data.defaultDataRoot}/${v}/${v}-main`); setMemory(String(d.memoryMiB)); setPort(String(d.suggestedPort)); setFrom(String(d.suggestedPort)); setTo(String(d.suggestedPort + 10)); setPorts(null); if (v !== 'redis' && kind === 'none')
        setKind('volume'); }}>{Object.entries(data.engines).map(([k, d]) => <option key={k} value={k}>{d.name}</option>)}</select></Field><Field label="Familia de versión" help="Familias admitidas. Se descarga la imagen oficial nativa y se fija su digest; no cambia de versión al reiniciar."><select value={version} onChange={e => setVersion(e.target.value)}>{def.versions.map(v => <option key={v}>{v}</option>)}</select></Field><Field label="Nombre visible" help="Etiqueta que reconocerás en el panel."><input required value={name} onChange={e => setName(e.target.value)}/></Field><Field label="ID estable" help="Usado por nearprod infra. No es el nombre de una base ni una URL web."><input required pattern="[a-z0-9][a-z0-9-]*" value={id} onChange={e => setId(e.target.value)}/></Field></div>
    <Field label="Persistencia" help="Se aplica solo al recurso que estás creando; no modifica la configuración de proyectos importados."><select value={kind} onChange={e => setKind(e.target.value)}><option value="volume">Volumen Docker — dentro de la VM</option><option value="folder">Carpeta local dedicada — visible en tu Mac</option>{engine === 'redis' && <option value="none">Sin persistencia — caché desechable</option>}</select></Field>
    {kind === 'folder' && <><Field label="Carpeta local de esta instancia" help="Debe estar vacía o no existir, no ser enlace simbólico y estar compartida con Colima. Se crea data/; se comprueban los permisos desde un contenedor. No uses iCloud ni una carpeta con otros archivos."><input required value={folder} onChange={e => setFolder(e.target.value)}/></Field><Alert>Todas las bases lógicas de esta instancia usan el mismo directorio físico. Para una carpeta por base debes crear otra instancia. La exportación de cada base sí produce un archivo independiente. El rendimiento de un bind mount puede ser distinto al de un volumen dentro de la VM.</Alert></>}
    <label className="check-label"><input type="checkbox" checked={publish} onChange={e => setPublish(e.target.checked)}/> Permitir acceso desde una herramienta en mi equipo (127.0.0.1)</label><p className="field-help">Los contenedores conectados a la red de datos no necesitan publicar puertos del host.</p>
    {publish && <section className="panel"><div className="form-grid"><Field label="Puerto local elegido" help={`El motor escucha internamente en ${def.port}; este número es solo para tu Mac.`}><input required type="number" min="1024" max="65535" value={port} onChange={e => setPort(e.target.value)}/></Field><div><label>Rango a consultar (máximo 128 puertos)</label><div className="button-row"><input aria-label="Puerto inicial" type="number" value={from} onChange={e => setFrom(e.target.value)}/><input aria-label="Puerto final" type="number" value={to} onChange={e => setTo(e.target.value)}/><button type="button" disabled={portLoading} onClick={() => void scan()}>Buscar libres</button></div></div></div>{portLoading && <Skeleton label="Comprobando puertos" rows={2}/>}<div className="port-options">{ports?.map(p => <button type="button" key={p.port} disabled={!p.available} className={port === String(p.port) ? 'primary' : ''} title={p.reason} onClick={() => setPort(String(p.port))}>{p.port} · {p.available ? 'libre' : 'ocupado'}</button>)}</div><p className="hint">La consulta no reserva puertos. Se vuelven a comprobar al confirmar e iniciar; otro proceso todavía puede ocuparlos.</p></section>}
    <details><summary>Recursos y conexiones</summary><div className="form-grid"><Field label="Límite de memoria (MiB)" help="No es una reserva. Debe quedar RAM para Linux, Traefik y tus aplicaciones; bajar demasiado provoca OOM."><input required type="number" min={def.minMemoryMiB} value={memory} onChange={e => setMemory(e.target.value)}/></Field>{engine !== 'redis' && <Field label="Conexiones máximas" help="Ajusta también los pools de API/workers para no agotarlas."><input required type="number" min="8" max="300" value={max} onChange={e => setMax(e.target.value)}/></Field>}</div></details>
    {error && <Alert error>{error}</Alert>}<div className="modal-actions"><button type="button" onClick={onClose}>Cancelar</button><button className="primary" disabled={busy}>{busy ? 'Comprobando…' : 'Revisar creación'}</button></div></form></Modal>;
}
function CreateDatabase({ instance, onClose, onSubmit }: {
    instance: Instance;
    onClose: () => void;
    onSubmit: (body: Data) => void;
}) { const [name, setName] = useState(''), [username, setUsername] = useState(''); return <Modal title={instance.engine === 'redis' ? 'Credencial de aplicación Redis' : 'Crear base y usuario'} subtitle={instance.name} onClose={onClose}><form onSubmit={e => { e.preventDefault(); onSubmit({ action: 'database', instance: instance.id, name, ...(username ? { username } : {}), confirm: true }); }}><Field label="Nombre de la base / aplicación" help="Minúsculas, números y underscore; empieza por letra. PostgreSQL/MySQL crean una base independiente, no un schema dentro de una base común."><input required pattern="[a-z][a-z0-9_]{0,39}" value={name} onChange={e => setName(e.target.value)} placeholder="tienda_dev"/></Field><Field label="Usuario (opcional)" help="Se propone uno si lo dejas vacío; la contraseña se genera de forma segura. No se entregan permisos de administrador."><input pattern="[a-z][a-z0-9_]{0,39}" value={username} onChange={e => setUsername(e.target.value)} placeholder="Generado por NearProd"/></Field><Alert>La operación crea datos reales y prueba autenticación y lectura/escritura. No migra la información de otras bases. Para pruebas destructivas crea una base separada, por ejemplo tienda_verify.</Alert><div className="modal-actions"><button type="button" onClick={onClose}>Cancelar</button><button className="primary">Crear y comprobar</button></div></form></Modal>; }
function ConnectionDialog({ database, onClose }: {
    database: Database;
    onClose: () => void;
}) {
    const [value, setValue] = useState<Connection | null>(null), [error, setError] = useState('');
    async function load(reveal = false) { try {
        setValue(await api<Connection>('/infra/connection', { database: database.id, reveal }));
    }
    catch (e) {
        setError(message(e));
    } }
    useEffect(() => { void load(); }, [database.id]);
    useEffect(() => { if (!value?.revealed)
        return; const t = setTimeout(() => void load(), 30000); return () => clearTimeout(t); }, [value?.revealed]);
    return <Modal title="Datos de conexión" subtitle={database.name} onClose={onClose} wide>{error && <Alert error>{error}</Alert>}{!value && <Skeleton label="Leyendo conexión" rows={5}/>} {value && <><Alert>{value.note}</Alert><div className="form-grid"><Field label="Usuario"><input readOnly value={value.user}/></Field><Field label="Contraseña" help="Se oculta de nuevo después de 30 segundos. No se envía al historial de operaciones."><input type={value.revealed ? 'text' : 'password'} readOnly value={value.password}/></Field></div><button onClick={() => void load(!value.revealed)}>{value.revealed ? 'Ocultar secreto' : 'Revelar credencial'}</button><Field label="Dentro de Docker" help="Usa este host desde el backend/worker conectado. localhost dentro de ese contenedor no es la base."><input readOnly value={value.internal.url}/></Field><Field label="Desde este equipo" help="Este puerto sirve para un cliente SQL/Redis local. No es una URL de navegador ni una ruta Traefik."><input readOnly value={value.local?.url || 'No publicado: solo conexión interna Docker'}/></Field></>}</Modal>;
}
function BindingDialog({ database, engine, stacks, onClose, onReview }: {
    database: Database;
    engine: Engine;
    stacks: Stack[];
    onClose: () => void;
    onReview: (body: Data, p: Preview) => void;
}) {
    const [target, setTarget] = useState(''), [mode, setMode] = useState('dev'), [services, setServices] = useState<string[]>([]), [choices, setChoices] = useState<string[]>([]), [format, setFormat] = useState('url'), [vars, setVars] = useState<Record<string, string>>({ url: engine === 'redis' ? 'REDIS_URL' : 'DATABASE_URL', host: 'DB_HOST', port: 'DB_PORT', database: 'DB_DATABASE', user: 'DB_USERNAME', password: 'DB_PASSWORD' }), [error, setError] = useState(''), [busy, setBusy] = useState(false), [loading, setLoading] = useState(false);
    const stack = stacks.find(s => s.id === target);
    useEffect(() => { setChoices([]); setServices([]); if (!stack)
        return; const controller = new AbortController(); setLoading(true); const files = stack.modes[mode as 'dev' | 'verify']?.files || []; void api<ProjectOptions>('/project-options', { path: stack.path, files }, controller.signal).then(o => setChoices(o.services)).catch(e => { if (!controller.signal.aborted)
        setError(message(e)); }).finally(() => setLoading(false)); return () => controller.abort(); }, [target, mode]);
    async function submit() { setBusy(true); try {
        const mapping = Object.fromEntries(Object.entries(vars).filter(([k]) => format === 'url' ? k === 'url' : k !== 'url'));
        const body = { target, database: database.id, mode, services, mapping };
        onReview(body, await api<Preview>('/infra/binding-preview', body));
    }
    catch (e) {
        setError(message(e));
    }
    finally {
        setBusy(false);
    } }
    return <Modal title="Vincular con un proyecto" subtitle={`${database.name} · solo se aplica después de revisar e iniciar el proyecto`} onClose={onClose} wide><form onSubmit={e => { e.preventDefault(); void submit(); }}><Field label="Aplicación registrada" help="No se elimina la base que ya pueda existir en su Compose; el usuario decide cuál utilizar."><select required value={target} onChange={e => { setTarget(e.target.value); setMode('dev'); }}><option value="">Selecciona aplicación</option>{stacks.map(s => <option key={s.id}>{s.id}</option>)}</select></Field><Field label="Modo" help="Desarrollo y prueba de imagen pueden apuntar a bases distintas. Este vínculo no separa datos por sí solo."><select value={mode} onChange={e => setMode(e.target.value)}><option value="dev">Desarrollo</option>{stack?.modes.verify && <option value="verify">Prueba de imagen</option>}</select></Field><fieldset><legend>Servicios consumidores</legend><p className="hint">Selecciona API y workers. No envíes contraseñas a código de frontend servido al navegador.</p>{loading && <Skeleton label="Leyendo servicios Compose" rows={2}/>} {choices.map(s => <label className="check-label" key={s}><input type="checkbox" checked={services.includes(s)} onChange={e => setServices(e.target.checked ? [...services, s] : services.filter(v => v !== s))}/>{s}</label>)}</fieldset><Field label="Formato que espera tu aplicación" help="Comprueba los nombres en su configuración. NearProd entrega variables, no modifica el código del framework."><select value={format} onChange={e => setFormat(e.target.value)}><option value="url">Una URL de conexión</option><option value="fields">Campos separados</option></select></Field><div className="form-grid">{Object.keys(vars).filter(k => format === 'url' ? k === 'url' : k !== 'url').map(k => <Field key={k} label={`Variable para ${k}`}><input required pattern="[A-Z][A-Z0-9_]*" value={vars[k]} onChange={e => setVars({ ...vars, [k]: e.target.value })}/></Field>)}</div>{error && <Alert error>{error}</Alert>}<div className="modal-actions"><button type="button" onClick={onClose}>Cancelar</button><button className="primary" disabled={busy || !services.length}>Revisar vinculación</button></div></form></Modal>;
}
function BackupDialog({ db, restore, onClose, onSubmit }: {
    db: Database;
    restore: boolean;
    onClose: () => void;
    onSubmit: (body: Data) => void;
}) { const [value, setValue] = useState(''), [trusted, setTrusted] = useState(false); return <Modal title={restore ? 'Restaurar en base vacía' : 'Exportar backup lógico'} subtitle={db.name} onClose={onClose}><form onSubmit={e => { e.preventDefault(); onSubmit({ action: restore ? 'restore' : 'backup', database: db.id, confirm: true, ...(restore ? { file: value, trustedBackup: trusted } : { directory: value || undefined }) }); }}><Alert>Un volumen o carpeta persistente no es un backup. El archivo puede contener datos sensibles: se guarda con permisos privados fuera de la VM. El resultado y la ruta aparecen en Actividad.</Alert><Field label={restore ? 'Archivo de backup' : 'Carpeta de destino (opcional)'} help={restore ? 'Selecciona un dump creado por NearProd con su archivo .nearprod.json al lado. Debe corresponder al motor y versión compatibles.' : 'Vacío usa ~/.nearprod/backups/databases (o tu NEARPROD_HOME). No se sobrescriben archivos existentes.'}><input required={restore} value={value} onChange={e => setValue(e.target.value)} placeholder={restore ? '/Users/usuario/Backups/tienda.dump' : '/Users/usuario/Backups'}/></Field>{restore && <><Alert error>Solo se acepta una base vacía y sin vínculos. Un fallo puede dejar objetos parciales; no hay rollback automático. MySQL no restaura rutinas/eventos y puede rechazar definers ajenos.</Alert><label className="check-label"><input required type="checkbox" checked={trusted} onChange={e => setTrusted(e.target.checked)}/> Confío en este backup y acepto importarlo en este destino vacío.</label></>}<div className="modal-actions"><button type="button" onClick={onClose}>Cancelar</button><button className="primary">{restore ? 'Restaurar como usuario limitado' : 'Exportar'}</button></div></form></Modal>; }
function InfraLogs({ instance, onClose }: {
    instance: Instance;
    onClose: () => void;
}) { const [lines, setLines] = useState<string[]>([]), [error, setError] = useState(''); useEffect(() => { const stream = new EventSource(`/api/infra/logs?instance=${encodeURIComponent(instance.id)}`); stream.addEventListener('line', e => { const value = JSON.parse((e as MessageEvent).data) as {
    text: string;
}; setLines(l => [...l, value.text].slice(-500)); }); stream.addEventListener('log-error', e => setError((JSON.parse((e as MessageEvent).data) as {
    message: string;
}).message)); stream.onerror = () => setError('Reconectando; pueden existir repeticiones o huecos.'); return () => stream.close(); }, [instance.id]); return <Modal title="Logs de infraestructura" subtitle={instance.name} onClose={onClose} wide>{error && <Alert error>{error}</Alert>}<p className="hint">Máximo 500 líneas visibles. Pueden contener datos sensibles. Cerrar solo libera el seguimiento, no detiene el motor.</p><pre className="console infra-console">{lines.join('\n') || 'Conectando…'}</pre></Modal>; }
