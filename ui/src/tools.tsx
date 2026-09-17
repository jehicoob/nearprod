import { api, message } from './api.js';
import { Alert, Field, Modal, Skeleton, Icon } from './components.js';
import type { Operation } from './types.js';
const { useState, useEffect } = React;
interface Tool {
    id: string;
    name: string;
    version: string | null;
    path: string | null;
    provider: string;
    status: string;
    available: boolean;
    purpose: string;
    required: boolean;
}
interface Inventory {
    platform: string;
    managers: {
        id: string;
        scope: string;
        path: string;
        evidence: string;
        managed: boolean;
    }[];
    binary: { language: string; version: string; path: string; requiresNode: boolean };
    projectNode?: { path: string; manager: string } | null;
    tools: Tool[];
    recommendation: string;
}
interface Versions {
    tool: string;
    provider: string;
    options: {
        formula: string;
        version: string;
        installed: string[];
        pinned: boolean;
        kegOnly: boolean;
        deprecated: boolean;
        dependencies: string[];
    }[];
    note: string;
}
interface Plan {
    fingerprint: string;
    command: string[] | null;
    warnings: string[];
    note: string;
    [key: string]: unknown;
}
export function ToolsView({ notify, operations }: {
    notify: (m: string, e?: boolean) => void;
    operations: Operation[];
}) {
    const [data, setData] = useState<Inventory | null>(null), [error, setError] = useState(''), [loading, setLoading] = useState(false), [selected, setSelected] = useState<Tool | null>(null), [startup, setStartup] = useState<{
        supported: boolean;
        enabled: boolean;
        loaded?: boolean;
        message: string;
    } | null>(null);
    async function load() { setLoading(true); try {
        setData(await api<Inventory>('/tools'));
        setStartup(await api('/startup'));
        setError('');
    }
    catch (e) {
        setError(message(e));
    }
    finally {
        setLoading(false);
    } }
    useEffect(() => { void load(); }, []);
    const active = operations.some(o => o.state === 'running');
    return <><div className="section-heading"><div><span className="eyebrow">DEPENDENCIAS Y ARRANQUE</span><h1>Herramientas del equipo</h1><p>Detectar primero. Conservar lo que ya funciona. Instalar solo con una revisión explícita.</p></div><button disabled={loading} onClick={() => void load()}><Icon name="refresh"/>Actualizar inventario</button></div>{error && <Alert error>{error}</Alert>}{!data && loading && <Skeleton label="Detectando gestores y herramientas" rows={7}/>} {data && <><Alert>{data.recommendation}</Alert><section className="panel"><h2>Tu entorno detectado</h2><p>Agente nativo: <strong>{data.binary.language}</strong> · compilador: <strong>{data.binary.version}</strong></p><code className="break-all">{data.binary.path}</code><p className="hint">NearProd no necesita Node, npm ni un gestor de versiones para ejecutarse. Los gestores detectados pertenecen a tus proyectos y se conservan sin cambios.</p><div className="table-wrap"><table><thead><tr><th>Gestor</th><th>Ámbito / evidencia</th><th>Ruta</th><th>Instalación guiada</th></tr></thead><tbody>{data.managers.map((m, i) => <tr key={`${m.id}-${i}`}><td>{m.id}</td><td>{{system:'Herramientas del sistema',node:'Node.js',runtimes:'Runtimes','node-packages':'Paquetes Node'}[m.scope]||m.scope} · {{executable:'ejecutable encontrado','shell-file':'archivo de shell','pinned-runtime-path':'ruta del Node fijado'}[m.evidence]||m.evidence}</td><td className="mono break-all">{m.path}</td><td>{m.managed ? 'Homebrew disponible' : 'Solo detección'}</td></tr>)}</tbody></table></div></section><div className="tool-cards">{data.tools.map(t => <section className="panel" key={t.id}><div className="panel-heading"><Icon name="terminal"/><h2>{t.name}</h2><span className={`badge ${t.available ? 'badge-healthy' : 'badge-unknown'}`}>{t.available ? 'Disponible' : t.status === 'plugin-not-registered' ? 'Plugin sin registrar' : 'No disponible'}</span></div><p>{t.purpose}</p><p className="mono break-all">{t.version || 'Versión sin comprobar'}<br />{t.path || 'No localizado'}</p><small>Origen: {{existing:'Instalación existente',homebrew:'Homebrew',none:'No detectado'}[t.provider]||t.provider} · {t.required ? 'Necesario para operar' : 'Necesario al construir, no para leer logs'}</small><div className="modal-actions"><button disabled={active || data.platform !== 'darwin'} onClick={() => setSelected(t)}>Versiones / instalar / reparar</button></div></section>)}</div><section className="panel"><h2>Traefik · componente central</h2><p>Traefik proporciona las URLs <code>proyecto.localhost</code>. Se aprovisiona en la VM existente desde <strong>Accesos locales</strong>. No se instala mediante Homebrew ni se mezcla con los motores de datos opcionales.</p></section></>}
    <StoragePanel/>
    <section className="panel"><h2>Iniciar NearProd al entrar en macOS</h2>{startup ? <p>{startup.message}</p> : <Skeleton label="Comprobando inicio automático" rows={2}/>}<pre className="console">nearprod startup enable<br />nearprod startup status<br />nearprod startup disable</pre><p>Ejecuta estos comandos como tu usuario, sin sudo. Se inicia el agente después del inicio de sesión, no antes de desbloquear el equipo. No abre el navegador ni enciende Colima, Traefik o proyectos automáticamente.</p><p className="hint">Deshabilitar impide próximos arranques y conserva el agente actual. Para cerrarlo usa nearprod agent stop. El instalador nativo no depende de Node ni de fnm.</p></section>
    {selected && <ToolDialog tool={selected} onClose={() => setSelected(null)} onSubmitted={id => { setSelected(null); notify(`Operación ${id} iniciada. Consulta Actividad y vuelve a comprobar el inventario al terminar.`); }}/>}
  </>;
}
function ToolDialog({ tool, onClose, onSubmitted }: {
    tool: Tool;
    onClose: () => void;
    onSubmitted: (id: string) => void;
}) {
    const [versions, setVersions] = useState<Versions | null>(null), [formula, setFormula] = useState(''), [action, setAction] = useState(tool.status === 'plugin-not-registered' ? 'repair' : 'install'), [preview, setPreview] = useState<Plan | null>(null), [error, setError] = useState(''), [busy, setBusy] = useState(false);
    useEffect(() => { const controller = new AbortController(); void api<Versions>('/tools/versions', { tool: tool.id }, controller.signal).then(v => { setVersions(v); setFormula(v.options[0]?.formula || ''); }).catch(e => { if (!controller.signal.aborted)
        setError(message(e)); }); return () => controller.abort(); }, [tool.id]);
    async function submit() { setBusy(true); try {
        if (!preview)
            setPreview(await api<Plan>('/tools/preview', { tool: tool.id, formula, action }));
        else {
            const op = await api<Operation>('/tools/actions', { tool: tool.id, formula, action, fingerprint: preview.fingerprint, confirm: true });
            onSubmitted(op.id);
        }
    }
    catch (e) {
        setError(message(e));
    }
    finally {
        setBusy(false);
    } }
    return <Modal title={`Gestionar ${tool.name}`} subtitle="Proveedor inicial: Homebrew macOS; no sudo ni actualización global" onClose={onClose} wide>{error && <Alert error>{error}</Alert>}{!versions && !error && <Skeleton label="Consultando versiones realmente disponibles" rows={5}/>} {versions && <form onSubmit={e => { e.preventDefault(); void submit(); }}><Alert>{versions.note} Cancelar conserva la instalada. No se ofrece cualquier versión histórica.</Alert><Field label="Fórmula / versión" help="Se muestran únicamente fórmulas disponibles consultadas en Homebrew. Una versión fijada no se desfija automáticamente."><select disabled={!!preview} required value={formula} onChange={e => setFormula(e.target.value)}>{versions.options.map(o => <option key={o.formula} value={o.formula}>{o.formula} · {o.version}{o.installed.length ? ' · instalada ' + o.installed.join(', ') : ''}{o.pinned ? ' · FIJADA' : ''}</option>)}</select></Field><Field label="Acción" help="Reparar registra plugins sin reinstalar. Instalar/actualizar puede descargar dependencias y requerir red."><select disabled={!!preview} value={action} onChange={e => setAction(e.target.value)}><option value="install">Instalar la opción elegida</option><option value="upgrade">Actualizar la fórmula elegida</option>{['compose', 'buildx'].includes(tool.id) && <option value="repair">Reparar registro del plugin</option>}</select></Field>{preview && <>{preview.warnings.map((w, i) => <Alert key={i}>{w}</Alert>)}<p>{preview.command ? "Comando aprobado: " + preview.command.join(" ") : "Se añadirá el directorio de plugins a la configuración Docker, preservando el resto y guardando un backup cuando exista."}</p><details><summary>Ruta, dependencias y detalles técnicos</summary><pre className="console review-json">{JSON.stringify(preview, null, 2)}</pre></details></>}<div className="modal-actions"><button type="button" onClick={onClose}>Conservar instalada / cerrar</button>{preview && <button type="button" onClick={() => setPreview(null)}>Cambiar selección</button>}<button className="primary" disabled={busy || !formula}>{busy ? 'Consultando…' : preview ? 'Confirmar instalación / reparación' : 'Revisar cambios'}</button></div></form>}</Modal>;
}

function StoragePanel() {
 const [paths,setPaths]=useState<Record<string, string>|null>(null),[error,setError]=useState(''),[busy,setBusy]=useState(false);
 useEffect(()=>{void api<Record<string,string>>('/config/paths').then(setPaths).catch(e=>setError(message(e)));},[]);
 async function backup(){setBusy(true);try{const v=await api<{file:string}>('/config/backup', {confirm:true});setError('Backup privado guardado en '+v.file);}catch(e){setError(message(e));}finally{setBusy(false);}}
 return <section className="panel"><h2>Configuración y datos persistentes</h2><p>Actualizar el ejecutable no borra el catálogo. Tus proyectos, URLs y vínculos se conservan. Los volúmenes y carpetas existentes no se mueven.</p>{!paths?<Skeleton label="Consultando almacenamiento" rows={3}/>:<dl><dt>Catálogo de proyectos</dt><dd className="mono break-all">{paths.catalog}</dd><dt>Carpeta para nuevas instancias de datos</dt><dd className="mono break-all">{paths.defaultDatabaseRoot}</dd><dt>Backups de configuración</dt><dd className="mono break-all">{paths.configurationBackups}</dd></dl>}<p className="hint">Una carpeta de datos corresponde a una instancia, no a cada base lógica. El backup de configuración contiene credenciales privadas, no los datos de los motores. No lo compartas.</p><button disabled={busy} onClick={()=>void backup()}>{busy?'Guardando…':'Guardar backup de configuración'}</button>{error&&<p role="status" className="break-all">{error}</p>}</section>;
}
