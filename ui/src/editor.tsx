import type { FormEvent } from 'react';
import { api, message } from './api.js';
import { RouteEditor } from './proxy.js';
import { Alert, Field, Icon, LiveStatus, Modal, Skeleton } from './components.js';
import type { Candidate, Group, Mode, ProjectOptions, Stack, StackDraft } from './types.js';
const { useState, useEffect } = React;
export const slugify = (v: string) => v.normalize('NFKD').replace(/[\u0300-\u036f]/g, '').toLowerCase().replace(/[^a-z0-9_-]+/g, '-').replace(/^[-_]+|[-_]+$/g, '').slice(0,48) || 'proyecto';
const blankMode = (): Mode => ({ files: [], envFiles: [], profiles: [] });
const absolute = (base: string, name: string) => name.startsWith('/') ? name : `${base.replace(/\/$/, '')}/${name}`;
const shortName = (base: string, file: string) => file.startsWith(base + '/') ? file.slice(base.length + 1) : file;
function fromCandidate(c?: Candidate, stack?: Stack): StackDraft {
  return stack ? {product: stack.product, slug: stack.slug, name: stack.name, path: stack.path, projectName: stack.projectName, modes: structuredClone(stack.modes), links: stack.links || [], routes: structuredClone(stack.routes || [])} : {
    product: c?.product || '', slug: c?.slug || 'app', name: c?.relative.split('/').at(-1) || '', path: c?.path || '', projectName: '',
    modes: {dev: {files: (c?.suggested || []).map(f => absolute(c!.path,f)), envFiles: [], profiles: []}}, links: [],
  };
}
export function GroupPicker({ groups, value, name, onChange }: {groups: Group[]; value: string; name: string; onChange: (id: string, name: string) => void}) {
  const [choice, setChoice] = useState(groups.some(g => g.id === value) ? value : '__new');
  return <div className="group-picker"><Field label="Grupo" help="Reúne frontend, backend y otros componentes de un producto. Agrupar no crea redes, dominios ni mezcla sus archivos Compose.">
    <select value={choice} onChange={e => { const id = e.target.value; setChoice(id); const g = groups.find(g => g.id === id); onChange(g?.id || slugify(name || 'Mi proyecto'), g?.name || name || 'Mi proyecto'); }}>
      {groups.map(g => <option key={g.id} value={g.id}>{g.name}</option>)}<option value="__new">＋ Crear un grupo nuevo</option>
    </select>
  </Field>{choice === '__new' && <Field label="Nombre del nuevo grupo" help="Por ejemplo, Máximo Puntaje. Es un nombre de organización dentro de NearProd."><input required maxLength={120} value={name} onChange={e => onChange(slugify(e.target.value), e.target.value)} placeholder="Máximo Puntaje"/></Field>}</div>;
}
function OrderedFiles({ label, help, choices, value, onChange, allowEmpty = false }: {label: string; help: string; choices: {name: string; path: string; badge?: string}[]; value: string[]; onChange: (v: string[]) => void; allowEmpty?: boolean}) {
  const [manual, setManual] = useState('');
  const all = [...choices, ...value.filter(v => !choices.some(c => c.path === v)).map(path => ({path, name: path, badge: 'Ruta explícita'}))];
  const move = (i: number, offset: number) => { const next = [...value]; [next[i],next[i+offset]] = [next[i+offset],next[i]]; onChange(next); };
  return <div className="file-picker"><h4>{label}</h4><p className="field-help">{help}</p>
    {all.map(file => <label className="file-option" key={file.path}><input type="checkbox" checked={value.includes(file.path)} onChange={e => onChange(e.target.checked ? [...value,file.path] : value.filter(v => v !== file.path))}/><span><code>{file.name}</code>{file.badge && <small>{file.badge}</small>}</span></label>)}
    {value.length > 1 && <div className="ordered-selection"><strong>Orden de aplicación</strong>{value.map((v,i) => <div key={v}><span className="file-number">{i+1}</span><code>{all.find(c => c.path === v)?.name || v}</code><button type="button" disabled={i === 0} aria-label={`Subir ${all.find(c => c.path === v)?.name || v}`} onClick={() => move(i,-1)}>↑</button><button type="button" disabled={i === value.length-1} aria-label={`Bajar ${all.find(c => c.path === v)?.name || v}`} onClick={() => move(i,1)}>↓</button></div>)}</div>}
    {!value.length && !allowEmpty && <p className="warning-text">Selecciona al menos un archivo antes de continuar.</p>}
    <details className="advanced compact-details"><summary>Añadir una ruta que no aparece</summary><p className="field-help">Para archivos en otra subcarpeta o raíz autorizada. La ruta se valida al guardar; no se copia el archivo.</p><div className="inline-input"><input aria-label={`Ruta adicional: ${label}`} value={manual} onChange={e => setManual(e.target.value)} placeholder="config/compose.local.yaml"/><button type="button" disabled={!manual.trim() || value.includes(manual.trim())} onClick={() => {onChange([...value,manual.trim()]); setManual('');}}>Añadir</button></div></details>
  </div>;
}
function ModeEditor({ title, path, mode, onChange, initialOptions }: {title: string; path: string; mode: Mode; onChange: (m: Mode) => void; initialOptions: ProjectOptions | null}) {
  const [options, setOptions] = useState(initialOptions), [loading, setLoading] = useState(false), [error, setError] = useState('');
  const key = JSON.stringify(mode.files);
  useEffect(() => {
    if (!path) return;
    const controller = new AbortController(); setLoading(true); setError('');
    const timer = setTimeout(() => { void api<ProjectOptions>('/project-options', {path, files: mode.files}, controller.signal).then(o => {if (!controller.signal.aborted) setOptions(o);}).catch(e => {if (!controller.signal.aborted) setError(message(e));}).finally(() => {if (!controller.signal.aborted) setLoading(false);}); }, 150);
    return () => {clearTimeout(timer); controller.abort();};
  }, [path,key]);
  const [customEnv, setCustomEnv] = useState(mode.envFiles.length > 0);
  const profiles = options?.profiles || [];
  return <fieldset className="mode-editor"><legend>{title}</legend>
    {loading && !options ? <Skeleton label="Leyendo nombres de archivos y perfiles" rows={4}/> : <>
      <OrderedFiles label="Configuración Docker Compose" help="Estos archivos describen los servicios, puertos y volúmenes. Con uno usas su configuración; con varios Docker los combina EN ORDEN: el primero es la base y los siguientes pueden añadir o reemplazar ajustes. No son aplicaciones separadas y no se editan aquí. Solo se pasan los seleccionados: compose.override.yaml no se añade automáticamente."
        choices={(options?.composeFiles || []).map(f => ({...f,badge: f.base ? 'Archivo base' : 'Variante · revisar su propósito'}))} value={mode.files.map(f => absolute(path,f))} onChange={files => onChange({...mode,files})}/>
      <div className="env-selector"><h4>Variables para configurar Compose</h4><p className="field-help">Sirven para completar referencias como <code>{'${APP_PORT}'}</code> en Compose. Esto no inyecta automáticamente todas las variables dentro del contenedor: eso lo declara <code>environment</code> o <code>env_file</code> en el YAML.</p>
        <label className="file-option"><input type="radio" checked={!customEnv} onChange={() => {setCustomEnv(false); onChange({...mode,envFiles:[]});}}/><span>Automático <small>{options?.envFiles.some(f => f.automatic) ? 'Usar .env de esta carpeta, según las reglas de Compose.' : 'No se encontró .env aquí. No se añaden archivos explícitos.'}</small></span></label>
        <label className="file-option"><input type="radio" checked={customEnv} onChange={() => setCustomEnv(true)}/><span>Elegir archivos de entorno <small>Solo se listan nombres; sus valores y contraseñas no se muestran.</small></span></label>
        {customEnv && <><OrderedFiles label="Archivos .env" help="Se pasan como --env-file en el orden elegido. Si una variable se repite, el último archivo tiene prioridad. Al elegir .env.local no se añade .env automáticamente; selecciónalos ambos si necesitas combinar sus valores."
          choices={(options?.envFiles || []).map(f => ({...f,badge: f.template ? 'Plantilla; no contiene necesariamente valores utilizables' : f.automatic ? 'Predeterminado de Compose' : 'Archivo local'}))} value={mode.envFiles.map(f => absolute(path,f))} onChange={envFiles => onChange({...mode,envFiles})}/>
          {!options?.envFiles.length && <p className="hint">No hay archivos de entorno en esta carpeta. Puedes usar el modo automático o añadir una ruta explícita.</p>}
          {!mode.envFiles.length && <p className="hint">Sin una selección se guardará el comportamiento automático.</p>}</>}
        {!!options?.serviceEnvFiles.length && <p className="hint">El YAML ya declara <code>env_file</code>: {options.serviceEnvFiles.join(', ')}. Compose se encarga de esos archivos; no hace falta seleccionarlos otra vez aquí.</p>}
      </div>
      <div className="profiles-selector"><h4>Servicios opcionales — perfiles Compose</h4><p className="field-help">No son grupos de NearProd ni perfiles del runtime. Activan servicios que el proyecto marcó como opcionales, por ejemplo un worker o una herramienta de administración.</p>
        <LiveStatus message={loading ? 'Actualizando perfiles según los archivos seleccionados.' : ''}/>
        {!profiles.length && !mode.profiles.length && !loading && <p className="hint">Este conjunto de archivos no declara perfiles. No necesitas configurar nada aquí.</p>}
        {profiles.map(p => <label className="file-option" key={p.name}><input type="checkbox" checked={mode.profiles.includes(p.name)} onChange={e => onChange({...mode,profiles:e.target.checked ? [...mode.profiles,p.name] : mode.profiles.filter(v => v !== p.name)})}/><span><code>{p.name}</code><small>Activa: {p.services.join(', ')}</small></span></label>)}
        {mode.profiles.filter(p => !profiles.some(v => v.name === p)).map(p => <label className="file-option" key={p}><input type="checkbox" checked onChange={() => onChange({...mode,profiles:mode.profiles.filter(v => v !== p)})}/><span>{p}<small>No detectado en la lectura estática; revisar con Compose.</small></span></label>)}
        {!!options?.defaultServices.length && <p className="hint">Sin activar perfiles, se incluyen: {options.defaultServices.join(', ')}. La revisión con Compose valida el modelo efectivo.</p>}
      </div>
    </>}{error && <Alert error>{error}</Alert>}{options?.warnings.map(w => <Alert key={w}>{w}</Alert>)}
  </fieldset>;
}
export function StackFields({ value, onChange, existing, onReady }: {value: StackDraft; onChange: (d: StackDraft) => void; existing?: Stack; onReady: (ready: boolean) => void}) {
  const [options,setOptions] = useState<ProjectOptions | null>(null), [loading,setLoading] = useState(false), [error,setError] = useState('');
  const latest = React.useRef(value); latest.current = value;
  useEffect(() => {onReady(Boolean(options && !loading && !error && value.projectName));}, [options,loading,error,value.projectName,onReady]);
  useEffect(() => {
    setOptions(null); if (!value.path) return;
    const controller = new AbortController(); setLoading(true); setError('');
    const timer = setTimeout(() => { void api<ProjectOptions>('/project-options', {path: value.path, ...(value.modes.dev.files.length ? {files:value.modes.dev.files} : {})}, controller.signal).then(o => {
      if (controller.signal.aborted) return;
      setOptions(o); const d = latest.current;
      if (!existing && (!d.projectName || !d.modes.dev.files.length)) onChange({...d, projectName:d.projectName || o.projectName, modes:{...d.modes, dev:{...d.modes.dev,files:d.modes.dev.files.length ? d.modes.dev.files : o.suggestedFiles}}});
    }).catch(e => {if (!controller.signal.aborted) setError(message(e));}).finally(() => {if (!controller.signal.aborted) setLoading(false);}); }, 150);
    return () => {clearTimeout(timer); controller.abort();};
  }, [value.path]);
  const setMode = (key: 'dev' | 'verify', mode: Mode) => onChange({...value,modes:{...value.modes,[key]:mode}});
  return <div className="stack-fields">
    <Field label="Nombre de la aplicación" help="El nombre visible de esta pieza del grupo; por ejemplo Frontend o API. Un Compose con varios servicios sigue siendo una sola aplicación aquí."><input required value={value.name} onChange={e => onChange({...value,name:e.target.value})} maxLength={120} placeholder="Backend de Máximo Puntaje"/></Field>
    {existing || options ? <div className="checkout-summary"><Icon name="folder"/><span><strong>Carpeta de la aplicación</strong><code>{value.path}</code></span>{!existing && <button type="button" onClick={() => {setOptions(null); onChange({...value,path:'',projectName:'',modes:{dev:blankMode()}});}}>Cambiar</button>}</div> : <Field label="Carpeta de la aplicación" help="Directorio donde está el Compose. Debe estar dentro de una raíz autorizada desde Descubrir."><input required value={value.path} onChange={e => onChange({...value,path:e.target.value,projectName:'',modes:{dev:blankMode()}})} placeholder="/ruta/absoluta/proyecto/backend"/></Field>}
    {loading && !options ? <Skeleton label="Buscando configuraciones de esta aplicación" rows={5}/> : <>
      <ModeEditor title="Desarrollo local" path={value.path} mode={value.modes.dev} onChange={m => setMode('dev',m)} initialOptions={options}/>
      <details className="advanced" open={value.modes.verify ? true : undefined}><summary>Prueba de imagen — opcional</summary><p>Desarrollo permite editar código y recargarlo. La prueba de imagen ejecuta el código empaquetado dentro de la imagen, sin montajes del código ni recarga de desarrollo. No hace falta activarla para empezar a usar NearProd.</p>
        <label className="check-label"><input type="checkbox" checked={Boolean(value.modes.verify)} onChange={e => {const modes = {...value.modes}; if (e.target.checked) modes.verify = {...blankMode(),files:options?.suggestedFiles || []}; else delete modes.verify; onChange({...value,modes});}}/>Habilitar prueba de imagen</label>
        {value.modes.verify && <><ModeEditor title="Configuración para probar la imagen" path={value.path} mode={value.modes.verify} onChange={m => setMode('verify',m)} initialOptions={options}/><Alert>Solo se guarda una configuración adicional; no se ejecuta ni se construye ahora. Este modo NO certifica producción y comparte los volúmenes/datos del modo de desarrollo. La versión actual rechaza todos los bind mounts en esta prueba.</Alert></>}
      </details>
      <RouteEditor value={value} onChange={onChange} initialOptions={options}/>
      <details className="advanced"><summary>Identidad técnica y opciones avanzadas</summary><Field label="Identificador de aplicación" help="Se usa en la CLI, por ejemplo grupo/backend. Se propone automáticamente; no es un dominio."><input value={value.slug} disabled={Boolean(existing)} pattern="[a-z0-9][a-z0-9_-]{0,62}" onChange={e => onChange({...value,slug:e.target.value})}/></Field>
        <Field label="Nombre de proyecto Compose" help="Docker usa este nombre para identificar contenedores, redes y volúmenes. Si la aplicación ya existe, conserva su nombre exacto. Cambiar de grupo no lo cambia."><input disabled={Boolean(existing)} pattern="[a-z0-9][a-z0-9_-]{0,62}" value={value.projectName} onChange={e => onChange({...value,projectName:e.target.value})}/></Field>
        <p className="hint">Sugerencia: {options?.projectNameSource === 'existing' ? 'identidad encontrada en el registro o contenedores observados' : options?.projectNameSource === 'compose-name' ? 'campo name del Compose' : 'nombre de la carpeta'}. COMPOSE_PROJECT_NAME o ejecuciones con -p pueden haber usado otro nombre: compruébalo antes de iniciar.</p>
      </details>
      <details className="advanced"><summary>Enlaces manuales existentes — avanzado</summary><p>Añade direcciones que ya funcionan, por ejemplo <code>http://proyecto.localhost</code> y <code>http://api-proyecto.localhost</code>. NearProd las guarda como accesos directos; no crea el dominio, el proxy ni la conexión entre frontend y backend.</p>
        {value.links.map((link,i) => <div className="link-editor" key={i}><Field label={`Nombre del enlace ${i+1}`} help="Por ejemplo, Sitio o API."><input required maxLength={80} value={link.label} onChange={e => onChange({...value,links:value.links.map((v,j) => j===i ? {...v,label:e.target.value} : v)})}/></Field><Field label={`URL local ${i+1}`} help="HTTP o HTTPS; localhost, .localhost o loopback. Sin contraseñas ni parámetros."><input required type="url" value={link.url} onChange={e => onChange({...value,links:value.links.map((v,j) => j===i ? {...v,url:e.target.value} : v)})} placeholder="http://api-proyecto.localhost"/></Field><button type="button" onClick={() => onChange({...value,links:value.links.filter((_,j) => j!==i)})}>Quitar enlace</button></div>)}
        <button type="button" disabled={value.links.length >= 8} onClick={() => onChange({...value,links:[...value.links,{label:'Abrir aplicación',url:''}]})}><Icon name="plus" size={14}/>Añadir enlace existente</button>
        <p className="hint">Para que NearProd configure el proxy utiliza la sección URL de acceso anterior; estos son solo marcadores. Los puertos publicados por Docker se muestran por separado.</p>
      </details>
    </>}{error && <Alert error>{error}</Alert>}
  </div>;
}
function validateDraft(d: StackDraft): string | null {
  if (!d.name.trim() || !d.path.trim() || !d.projectName.trim() || !d.product.trim()) return 'Completa nombre, grupo y carpeta. Espera la lectura de archivos o revisa la identidad técnica.';
  if (!/^[a-z0-9][a-z0-9_-]{0,62}$/.test(d.slug) || !/^[a-z0-9][a-z0-9_-]{0,62}$/.test(d.projectName)) return 'Revisa los identificadores en Opciones avanzadas: usa letras minúsculas, números, guiones o guiones bajos.';
  if (!d.modes.dev.files.length || d.modes.verify && !d.modes.verify.files.length) return 'Selecciona al menos un archivo Compose para cada modo habilitado.';
  if ((d.routes || []).some(v => !v.host.trim() || !v.service || !Number.isInteger(v.port) || v.port < 1 || v.port > 65535)) return 'Completa dominio, servicio HTTP y puerto interno de cada URL, o quita la URL que no necesitas.';
  if (d.links.some(l => !l.label.trim() || !l.url.trim())) return 'Completa o elimina los enlaces vacíos.';
  return null;
}
export function StackEditor({ candidate, stack, groups, onClose, onSaved }: {candidate?: Candidate; stack?: Stack; groups: Group[]; onClose: () => void; onSaved: () => void}) {
  const [draft,setDraft] = useState<StackDraft>(() => fromCandidate(candidate,stack));
  const [groupName,setGroupName] = useState(groups.find(g => g.id === draft.product)?.name || candidate?.product || 'Mi proyecto');
  const [busy,setBusy] = useState(false), [error,setError] = useState(''), [ready,setReady] = useState(false);
  async function save(e: FormEvent) {
    e.preventDefault(); const d = {...draft, product: draft.product || slugify(groupName),groupName}; const invalid = validateDraft(d); if (invalid) {setError(invalid);return;}
    setBusy(true);setError(''); try {await api(stack ? '/stacks/edit' : '/stacks',stack ? {target:stack.id,definition:d} : d);onSaved();} catch(e) {setError(message(e));} finally {setBusy(false);}
  }
  return <Modal title={stack ? 'Configurar aplicación' : 'Registrar aplicación'} subtitle="Elige el grupo y revisa la configuración encontrada. Guardar no inicia contenedores ni modifica tus repositorios." onClose={busy ? () => {} : onClose} wide><form onSubmit={e => void save(e)}>
    <GroupPicker groups={groups} value={draft.product} name={groupName} onChange={(product,name) => {setDraft({...draft,product});setGroupName(name);}}/>
    {stack && <p className="hint">Cambiar de grupo conserva el nombre Compose, los volúmenes y la propiedad de los contenedores. Actualiza la referencia grupo/aplicación de tus comandos CLI.</p>}
    <StackFields value={draft} onChange={setDraft} existing={stack} onReady={setReady}/>
    {error && <Alert error>{error}</Alert>}<div className="modal-actions"><button type="button" disabled={busy} onClick={onClose}>Cancelar</button><button className="primary" disabled={busy || !ready}>{busy ? 'Guardando…' : stack ? 'Guardar cambios' : 'Registrar sin ejecutar'}</button></div>
  </form></Modal>;
}
export function BatchEditor({ candidates, groups, onClose, onSaved }: {candidates: Candidate[]; groups: Group[]; onClose: () => void; onSaved: () => void}) {
  const [drafts,setDrafts] = useState(() => candidates.map(c => fromCandidate(c)));
  const [product,setProduct] = useState(candidates[0]?.product || 'mi-proyecto');
  const [groupName,setGroupName] = useState(groups.find(g => g.id === product)?.name || product);
  const [step,setStep] = useState(-1), [error,setError] = useState(''), [busy,setBusy] = useState(false), [ready,setReady] = useState(false);
  useEffect(() => {setReady(false);},[step]);
  const summary = step === drafts.length;
  async function next(e: FormEvent) {
    e.preventDefault(); setError('');
    if (step === -1) { if (!groupName.trim()) {setError('Escribe el nombre del grupo.');return;} setDrafts(ds => ds.map(d => ({...d,product,groupName}))); setStep(0);return; }
    if (!summary) {const invalid = validateDraft({...drafts[step],product}); if (invalid) {setError(invalid);return;} setStep(step+1);return;}
    setBusy(true);try {await api('/stacks/batch',{definitions:drafts.map(d => ({...d,product})),groupName});onSaved();} catch(e) {setError(message(e));} finally {setBusy(false);}
  }
  return <Modal title="Registrar selección en un grupo" subtitle="Un solo registro para tu selección. Si hay un conflicto, no se guarda parcialmente ni se ejecuta Docker." onClose={busy ? () => {} : onClose} wide><form onSubmit={e => void next(e)}>
    <div className="wizard-steps" aria-label="Progreso del registro"><span className={step === -1 ? 'current' : ''}>1 · Grupo</span><span className={step >= 0 && !summary ? 'current' : ''}>2 · Configuración</span><span className={summary ? 'current' : ''}>3 · Resumen</span></div>
    {step === -1 && <><GroupPicker groups={groups} value={product} name={groupName} onChange={(id,name) => {setProduct(id);setGroupName(name);}}/><h3>{drafts.length} aplicaciones seleccionadas</h3><ul className="hints">{drafts.map(d => <li key={d.path}>{d.path}</li>)}</ul><Alert>Agrupar no conecta los servicios entre sí ni combina sus Compose. Cada aplicación conserva su identidad Docker.</Alert></>}
    {step >= 0 && !summary && <><div className="wizard-context"><strong>{groupName}</strong><span>Aplicación {step+1} de {drafts.length}</span></div><StackFields key={drafts[step].path || step} value={drafts[step]} onReady={setReady} onChange={d => setDrafts(ds => ds.map((v,i) => i === step ? d : v))}/></>}
    {summary && <><h3>Todo irá al grupo «{groupName}»</h3>{drafts.map((d,i) => <article className="registration-summary" key={d.path}><div><strong>{d.name}</strong><button type="button" onClick={() => setStep(i)}>Revisar</button></div><code>{d.projectName}</code><p>Compose: {d.modes.dev.files.map(f => shortName(d.path,f)).join(' → ')}</p><p>Entorno: {d.modes.dev.envFiles.length ? d.modes.dev.envFiles.map(f => shortName(d.path,f)).join(' → ') : 'Automático (.env si existe)'}</p><p>URLs: {(d.routes || []).map(v => v.host + ' → ' + v.service + ':' + v.port).join(' · ') || 'Sin acceso HTTP (DB/worker o pendiente de configurar)'}</p><p>Servicios opcionales: {d.modes.dev.profiles.join(', ') || 'Ninguno'} · Prueba de imagen: {d.modes.verify ? 'Configurada' : 'No configurada'}</p></article>)}<Alert>Después de registrar podrás revisar, aprobar e iniciar las aplicaciones. No se crearán dominios ni se activarán servicios ahora.</Alert></>}
    {error && <Alert error>{error}</Alert>}<div className="modal-actions">{step > -1 && <button type="button" disabled={busy} onClick={() => {setError('');setStep(step-1);}}>Atrás</button>}<button type="button" disabled={busy} onClick={onClose}>Cancelar</button><button className="primary" disabled={busy || step >= 0 && !summary && (!ready || !drafts[step]?.projectName)}>{busy ? 'Registrando…' : summary ? 'Registrar grupo sin ejecutar' : step === -1 ? 'Revisar aplicaciones' : 'Continuar'}</button></div>
  </form></Modal>;
}
