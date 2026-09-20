import { api, ApiError, humanBytes, message } from './api.js';
import { Alert, Badge, Busy, Icon, ImageDialog, Modal, OperationsView, ReviewDialog, Skeleton } from './components.js';
import { DiscoverDialog, RuntimeView, LogsDrawer, GroupDialog } from './views.js';
import { StackEditor, BatchEditor } from './editor.js';
import { ProxyView, RouteLinks } from './proxy.js';
import {InfrastructureView} from './infrastructure.js';
import {ToolsView} from './tools.js';
import type { Group, Catalog, Stack, ArchivedStack, Status, Candidate, Operation, Observed, Container } from './types.js';
const { useState, useEffect } = React;
type Page = 'projects' | 'runtime' | 'activity' | 'proxy' | 'infra' | 'tools';
const EMPTY: Status = { connected: false, checkedAt: null, stacks: [] };
interface LifecycleReview { value: Record<string, unknown>; endpoint: string; body: Record<string, unknown> }
function CatalogLifecycleDialog({review,onClose,onApplied}: {review: LifecycleReview; onClose: () => void; onApplied: () => void}) {
  const [busy,setBusy] = useState(false), [error,setError] = useState('');
  const action = String(review.value.lifecycleAction || ''), stack = review.value.stack as {id:string;name:string;path:string;projectName:string} | undefined, group = review.value.group as {id:string;name:string} | undefined;
  const title = action === 'archive-stack' ? 'Revisar archivo de aplicación' : action === 'restore-stack' ? 'Restaurar aplicación' : action === 'delete-group' ? 'Eliminar grupo vacío' : 'Retirar raíz de descubrimiento';
  const confirm = action === 'archive-stack' ? 'Archivar del catálogo' : action === 'restore-stack' ? 'Restaurar en el catálogo' : action === 'delete-group' ? 'Eliminar grupo' : 'Retirar raíz';
  return <Modal title={title} subtitle="Revisa el alcance antes de confirmar" onClose={onClose} wide>
    {action === 'archive-stack' && <Alert><strong>Solo se retirará el registro activo.</strong> No se detendrán ni eliminarán contenedores, redes, volúmenes, checkouts o datos.</Alert>}
    {action === 'restore-stack' && <Alert>La aplicación y sus vinculaciones volverán al catálogo sin iniciar ni detener runtime.</Alert>}
    {stack && <section className="panel"><h3>{stack.name}</h3><p className="mono">{stack.id} · {stack.projectName}</p><p className="mono path">{stack.path}</p><p>Vinculaciones conservadas: {Number(review.value.bindingCount || 0)}</p></section>}
    {group && <section className="panel"><h3>{group.name}</h3><p className="mono">{group.id}</p><p>Aplicaciones activas: {Number(review.value.activeApplications || 0)} · archivadas: {Number(review.value.archivedApplications || 0)}</p><p>Solo se elimina metadata del grupo.</p></section>}
    {Boolean(review.value.root) && <section className="panel"><h3>Raíz sin dependencias</h3><p className="mono path">{String(review.value.root)}</p><p>No se borrará ninguna carpeta, checkout o recurso Docker.</p></section>}
    {error && <Alert error>{error}</Alert>}
    <div className="modal-actions"><button onClick={onClose}>Cancelar</button><button className={action === 'restore-stack' ? 'primary' : 'danger'} disabled={busy} onClick={() => {setBusy(true);setError('');void api(review.endpoint,{...review.body,confirm:true,fingerprint:review.value.fingerprint}).then(onApplied).catch(e => setError(message(e))).finally(() => setBusy(false));}}>{busy ? 'Aplicando…' : confirm}</button></div>
  </Modal>;
}
function App() {
  const [catalog, setCatalog] = useState<Catalog | null>(null), [status, setStatus] = useState<Status>(EMPTY), [auth, setAuth] = useState<boolean | null>(null), [code, setCode] = useState(''), [loginError, setLoginError] = useState(''), [loadError, setLoadError] = useState('');
  const [page, setPage] = useState<Page>('projects'), [search, setSearch] = useState(''), [discover, setDiscover] = useState(false), [editor, setEditor] = useState<{candidate?: Candidate; stack?: Stack} | null>(null);
  const [review, setReview] = useState<{stack: Stack; mode: string} | null>(null), [logs, setLogs] = useState<Stack | null>(null), [image, setImage] = useState<Container | null>(null), [mode, setMode] = useState<Record<string,string>>({});
  const [notice, setNotice] = useState<{text: string; error: boolean} | null>(null), [working, setWorking] = useState(false), [adoption, setAdoption] = useState<{id: string; allowed: boolean; fingerprint: string; containers: Container[]; warning: string} | null>(null);
  const [batch,setBatch] = useState<Candidate[] | null>(null), [groupEditor,setGroupEditor] = useState<{group?: Group} | null>(null), [lifecycleReview,setLifecycleReview] = useState<LifecycleReview | null>(null);
  const notify = (text: string, error = false) => setNotice({text, error});
  async function load() {
    setLoadError('');
    let nextCatalog: Catalog;
    try {
      nextCatalog = await api<Catalog>('/catalog');
    } catch(e) {
      if(e instanceof ApiError && e.code === 'AUTH_REQUIRED') setAuth(false);
      else setLoadError(message(e));
      return;
    }
    setCatalog(nextCatalog);
    setAuth(true);
    try {
      setStatus(await api<Status>('/status'));
    } catch(e) {
      if(e instanceof ApiError && e.code === 'AUTH_REQUIRED') setAuth(false);
      else setStatus(current => ({...current, connected:false, checkedAt:new Date().toISOString(), error:{code:'STATUS_UNAVAILABLE',message:message(e)}}));
    }
  }
  useEffect(() => {void load();}, []);
  useEffect(() => {
    if(!auth) return;
    const events = new EventSource('/api/events');
    events.onmessage = (e) => {
      const event = JSON.parse(e.data) as {type: string; status?: Status; operation?: Operation; id?: string; line?: Operation['lines'][0]; watch?: Observed['watch']};
      if(event.type === 'status' && event.status) setStatus(event.status);
      if(event.type === 'catalog') void api<Catalog>('/catalog').then(setCatalog).catch(e => notify(message(e),true));
      if(event.type === 'operation' && event.operation) {
        const op = event.operation; setCatalog(c => c ? {...c, operations: [op, ...c.operations.filter(v => v.id !== op.id)].slice(0,80)} : c);
        if(op.state === 'failed') notify(`${op.action}: ${(Array.isArray(op.results) ? op.results : []).find(r => r.error)?.error?.message || op.error?.message || 'Operación fallida. Consulta Actividad.'}`, true);
        if(op.state === 'succeeded') {notify(`${op.action}: operación completada. La salud de la aplicación se muestra por separado.`); void api<Catalog>('/catalog').then(setCatalog).catch(() => {});}
      }
      if(event.type === 'operation-line' && event.line) {const line = event.line; setCatalog(c => c ? {...c, operations:c.operations.map(v => v.id === event.id ? {...v, lines:[...v.lines, line].slice(-100)} : v)} : c);}
      if(event.type === 'watch' && event.watch) {const watch = event.watch; setStatus(s => ({...s, stacks:s.stacks.map(v => v.id === event.id ? {...v, watch} : v)}));}
    };
    events.onerror = () => {void api<Catalog>('/catalog').catch(e => {if(e instanceof ApiError && e.code === 'AUTH_REQUIRED') setAuth(false);});};
    return () => events.close();
  }, [auth]);
  async function work(task: () => Promise<void>) {setWorking(true); try {await task();} catch(e) {notify(message(e),true);} finally {setWorking(false);} }
  async function openLifecycle(previewEndpoint: string, endpoint: string, body: Record<string, unknown>) {await work(async () => setLifecycleReview({value:await api<Record<string,unknown>>(previewEndpoint,body),endpoint,body}));}
  async function action(stack: Stack, action: string) {
    const selectedMode = mode[stack.id] || stack.activeMode || 'dev';
    if (['up','rebuild'].includes(action) && stack.routes?.length && !catalog?.proxy?.enabled) {setPage('proxy');notify('Activa Traefik una vez para este catálogo; después revisa e inicia la aplicación.');return;}
    if(['up','rebuild'].includes(action) && !stack.trust[selectedMode]) {setReview({stack, mode:selectedMode}); return;}
    const confirmMode = !stack.activeMode || stack.activeMode === selectedMode || window.confirm('Cambiar de modo recrea los servicios y conserva los mismos datos/volúmenes. ¿Continuar con el stack completo?');
    if(!confirmMode) return;
    if(action === 'rebuild' && !window.confirm('¿Construir y recrear este stack? Puede consumir CPU/RAM y tardar varios minutos. Los volúmenes se conservan.')) return;
    await work(async () => {await api('/actions', {target:stack.id, action, mode:selectedMode, confirmMode:true}); notify(`${action}: solicitud enviada. Consulta Actividad.`);});
  }
  async function groupAction(product: string, action: string) {
    if (action === 'up' && !catalog?.proxy?.enabled && catalog?.stacks.some(s => s.product === product && s.routes?.length)) {setPage('proxy');notify('Activa Traefik antes de iniciar este grupo con URLs.');return;}
    if(!window.confirm(`${action === 'up' ? 'Iniciar' : 'Detener'} las aplicaciones del grupo ${product}. Los stacks son independientes; no es una transacción y no se borran datos. ¿Continuar?`)) return;
    await work(async () => {await api('/actions', {target:product, action}); notify('Operación de grupo enviada. Cada aplicación informa su resultado.');});
  }
  const cancel = (id: string) => {if(window.confirm('Cancelar el cliente no garantiza que el Engine detenga el trabajo ya recibido. ¿Solicitar cancelación?')) void work(async () => {await api('/cancel',{id}); notify('Cancelación solicitada; revisa el estado real.');});};
  if(auth === null && loadError) return <div className="login-page"><section className="login-card"><h1>No se pudo abrir NearProd</h1><Alert error>{loadError}</Alert><button className="primary full" onClick={() => void load()}>Reintentar conexión local</button></section></div>;
  if(auth === null) return <div className="login-page"><div className="login-card"><h2>NearProd</h2><Skeleton label="Abriendo tu entorno local" rows={5}/></div></div>;
  if(!auth) return <div className="login-page"><div className="login-brand"><div className="logo"><Icon name="cube" size={30}/></div><span>NearProd<span className="brand-dot">.</span></span></div><section className="login-card"><span className="eyebrow">TU ENTORNO. BAJO CONTROL.</span><h1>Conecta tu consola local</h1><p>Abre una terminal y ejecuta <code>nearprod ui</code>. Pega el código de acceso de un solo uso.</p><form onSubmit={e => {e.preventDefault(); setWorking(true); void api('/session', {code}).then(() => {setCode(''); return load();}).catch(e => setLoginError(message(e))).finally(() => setWorking(false));}}><label>Código de acceso<input autoFocus autoComplete="off" required value={code} onChange={e => setCode(e.target.value)} placeholder="Código de la terminal"/></label>{loginError && <Alert error>{loginError}</Alert>}<button className="primary full" disabled={working}>{working ? 'Conectando…' : 'Abrir NearProd'}<Icon name="chevron"/></button></form><div className="login-footer"><span className="live-dot"/>Local · Solo loopback · Sin cuenta en la nube</div></section><p className="hint">El panel funciona aunque Docker esté detenido. No inicia tus contenedores automáticamente.</p></div>;
  if(!catalog) return <Busy/>;
  const running = status.stacks.filter(s => s.execution === 'running').length, totalContainers = status.stacks.reduce((n,s) => n+s.containers.length,0), active = catalog.operations.filter(o => o.state === 'running');
  const archivedStacks = catalog.archivedStacks || [];
  const filtered = catalog.stacks.filter(s => `${s.name} ${s.id} ${s.path} ${s.projectName} ${catalog.groups?.find(g => g.id === s.product)?.name || ""}`.toLowerCase().includes(search.toLowerCase()));
  const filteredArchived = archivedStacks.filter(s => `${s.name} ${s.id} ${s.path} ${s.projectName} ${catalog.groups?.find(g => g.id === s.product)?.name || ""}`.toLowerCase().includes(search.toLowerCase()));
  const groups = [...new Set([...(catalog.groups || []).filter(g => !search || g.name.toLowerCase().includes(search.toLowerCase()) || filtered.some(s => s.product === g.id) || filteredArchived.some(s => s.product === g.id)).map(g => g.id), ...filtered.map(s => s.product), ...filteredArchived.map(s => s.product)])];
  const groupName = (id: string) => catalog.groups?.find(g => g.id === id)?.name || id;
  const checked = Boolean(status.checkedAt);
  return (
    <div className="shell">
      <aside className="sidebar">
        <a className="brand" href="/" aria-label="NearProd inicio">
          <span className="logo">
            <Icon name="cube" size={24} />
          </span>
          NearProd<span className="brand-dot">.</span>
        </a>
        <span className="version-label">
          LOCAL CONTROL / v{catalog.version}
        </span>
        <div className="nav-caption">WORKSPACE</div>
        <nav aria-label="Navegación principal">
          <button
            className={page === "projects" ? "active" : ""}
            onClick={() => setPage("projects")}
          >
            <Icon name="grid" />
            Aplicaciones<span>{catalog.stacks.length}</span>
          </button>
          <button
            className={page === "proxy" ? "active" : ""}
            onClick={() => setPage("proxy")}
          >
            <Icon name="link" />
            Accesos locales
          </button>
          <button
            className={page === "infra" ? "active" : ""}
            onClick={() => setPage("infra")}
          >
            <Icon name="cube" />
            Infraestructura
          </button>
          <button
            className={page === "tools" ? "active" : ""}
            onClick={() => setPage("tools")}
          >
            <Icon name="terminal" />
            Herramientas
          </button>
          <button
            className={page === "runtime" ? "active" : ""}
            onClick={() => setPage("runtime")}
          >
            <Icon name="settings" />
            Runtime y recursos
          </button>
          <button
            className={page === "activity" ? "active" : ""}
            onClick={() => setPage("activity")}
          >
            <Icon name="activity" />
            Actividad{active.length > 0 && <span>{active.length}</span>}
          </button>
        </nav>
        <div className="sidebar-bottom">
          <div className="host-card">
            <Icon name="terminal" />
            <div>
              <strong>CONTROLADOR LOCAL</strong>
              <p>
                {catalog.host.runtime.managedVirtualMachine
                  ? "Fuera de la VM"
                  : catalog.host.displayName}
              </p>
            </div>
          </div>
          <p>
            Compose es la fuente de verdad.
            <br />
            NearProd organiza y opera.
          </p>
          <button
            className="text-button"
            onClick={() => void api("/logout", {}).then(() => setAuth(false))}
          >
            Cerrar sesión del panel
          </button>
        </div>
      </aside>
      <div className="main">
        <header className="topbar">
          <div className="breadcrumb">
            Workspace <Icon name="chevron" size={13} />
            <strong>
              {page === "projects"
                ? "Aplicaciones"
                : page === "runtime"
                  ? "Runtime"
                  : page === "proxy"
                    ? "Accesos locales"
                    : page === "infra"
                      ? "Infraestructura"
                      : page === "tools"
                        ? "Herramientas"
                        : "Actividad"}
            </strong>
          </div>
          <div className="topbar-status">
            <span className={status.connected ? "live-dot" : "offline-dot"} />
            {!checked
              ? "Comprobando Engine…"
              : status.connected
                ? "Engine conectado"
                : "Engine no disponible"}
            <span className="context-tag">{catalog.runtime.context}</span>
          </div>
        </header>
        <main>
          {notice && (
            <div
              className={`toast ${notice.error ? "error" : ""}`}
              role={notice.error ? "alert" : "status"}
            >
              <Icon name={notice.error ? "warning" : "check"} />
              <span>{notice.text}</span>
              <button
                className="icon-button"
                aria-label="Cerrar aviso"
                onClick={() => setNotice(null)}
              >
                <Icon name="close" size={16} />
              </button>
            </div>
          )}
          {loadError && <Alert error>{loadError} <button onClick={() => void load()}>Reintentar</button></Alert>}
          {page === "projects" && (
            <>
              <div className="section-heading hero">
                <div>
                  <span className="eyebrow">DE TU CÓDIGO AL CONTENEDOR</span>
                  <h1>
                    Tu entorno, en un solo lugar
                    <span className="brand-dot">.</span>
                  </h1>
                  <p>
                    Descubre, ejecuta y comprueba tus aplicaciones sin perder de
                    vista lo que está pasando.
                  </p>
                </div>
                <button className="primary" onClick={() => setDiscover(true)}>
                  <Icon name="plus" />
                  Descubrir aplicaciones
                </button>
              </div>
              <div className="overview">
                <div>
                  <span>GRUPOS</span>
                  <strong>
                    {catalog.groups.length}
                    <small>agrupaciones locales</small>
                  </strong>
                </div>
                <div>
                  <span>APLICACIONES EN EJECUCIÓN</span>
                  <strong>
                    {!checked ? (
                      <Skeleton
                        compact
                        label="Comprobando aplicaciones"
                        rows={1}
                      />
                    ) : status.connected ? (
                      running
                    ) : (
                      "—"
                    )}
                    <small>de {catalog.stacks.length} registrados</small>
                  </strong>
                </div>
                <div>
                  <span>CONTENEDORES OBSERVADOS</span>
                  <strong>
                    {!checked ? (
                      <Skeleton
                        compact
                        label="Contando contenedores"
                        rows={1}
                      />
                    ) : status.connected ? (
                      totalContainers
                    ) : (
                      "—"
                    )}
                    <small>
                      {status.connected
                        ? "incluye detenidos"
                        : "runtime sin conexión"}
                    </small>
                  </strong>
                </div>
                <div>
                  <span>MEMORIA DEL ENGINE</span>
                  <strong className="compact">
                    {!checked ? (
                      <Skeleton
                        compact
                        label="Consultando memoria del Engine"
                        rows={1}
                      />
                    ) : (
                      humanBytes(status.info?.memoryBytes)
                    )}
                    <small>compartida por todas las cargas</small>
                  </strong>
                </div>
              </div>
              {checked && !status.connected && (
                <div className="runtime-banner">
                  <Icon name="warning" size={23} />
                  <div>
                    <strong>
                      El controlador está disponible; el runtime, no.
                    </strong>
                    <p>
                      {status.error?.message ||
                        `Consulta el diagnóstico y comprueba ${catalog.host.runtime.displayName}.`}{" "}
                      Puedes descubrir y registrar proyectos sin iniciar Docker.
                    </p>
                  </div>
                  <button onClick={() => setPage("runtime")}>
                    Revisar runtime <Icon name="chevron" size={14} />
                  </button>
                </div>
              )}
              <div className="project-toolbar">
                <h2>
                  Aplicaciones{" "}
                  <span className="count">{catalog.stacks.length}</span>
                </h2>
                <div>
                  <label className="search">
                    <Icon name="search" />
                    <input
                      aria-label="Filtrar aplicaciones"
                      value={search}
                      onChange={(e) => setSearch(e.target.value)}
                      placeholder="Grupo, aplicación o ruta…"
                    />
                  </label>
                  <button
                    className="icon-button"
                    aria-label="Actualizar estado"
                    disabled={working}
                    onClick={() =>
                      void work(async () =>
                        setStatus(await api<Status>("/status?refresh=1")),
                      )
                    }
                  >
                    <Icon name="refresh" />
                  </button>
                  <button onClick={() => setGroupEditor({})}>
                    <Icon name="folder" size={14} />
                    Crear grupo
                  </button>
                  <button onClick={() => setEditor({})}>
                    Registrar manualmente
                  </button>
                </div>
              </div>
              {!catalog.stacks.length && !archivedStacks.length && (
                <div className="empty">
                  <div className="empty-icon">
                    <Icon name="folder" size={34} />
                  </div>
                  <h2>Empieza por tu carpeta de proyectos</h2>
                  <p>
                    NearProd detecta archivos Compose y te propone una
                    agrupación.
                    <br />
                    Nada se construye ni se ejecuta sin tu intervención.
                  </p>
                  <button className="primary" onClick={() => setDiscover(true)}>
                    <Icon name="search" />
                    Explorar mi carpeta
                  </button>
                  <code>nearprod init ~/Projects</code>
                </div>
              )}
              {(catalog.stacks.length > 0 || archivedStacks.length > 0) && !groups.length && (
                <div className="empty small">
                  <p>No hay aplicaciones que coincidan con «{search}».</p>
                </div>
              )}
              {groups.map((product) => (
                <section className="product" key={product}>
                  <div className="product-heading">
                    <span className="product-icon">
                      <Icon name="folder" />
                    </span>
                    <div>
                      <h2>{groupName(product)}</h2>
                      <p>
                        {catalog.stacks.filter((s) => s.product === product).length} activas · {archivedStacks.filter((s) => s.product === product).length} archivadas · CLI: <code>{product}</code>
                      </p>
                    </div>
                    <div className="product-actions">
                      <button
                        onClick={() =>
                          setGroupEditor({
                            group: { id: product, name: groupName(product) },
                          })
                        }
                      >
                        Renombrar grupo
                      </button>
                      <button
                        disabled={
                          working ||
                          !status.connected ||
                          active.length > 0 ||
                          !catalog.stacks.some((s) => s.product === product)
                        }
                        onClick={() => void groupAction(product, "up")}
                      >
                        Iniciar grupo
                      </button>
                      <button
                        disabled={
                          working ||
                          !status.connected ||
                          active.length > 0 ||
                          !catalog.stacks.some((s) => s.product === product)
                        }
                        onClick={() => void groupAction(product, "stop")}
                      >
                        Detener grupo
                      </button>
                      <button
                        className="danger"
                        disabled={working || active.length > 0 || catalog.stacks.some(s => s.product === product) || archivedStacks.some(s => s.product === product)}
                        title={catalog.stacks.some(s => s.product === product) || archivedStacks.some(s => s.product === product) ? 'Archiva o mueve todas las aplicaciones antes de eliminar el grupo.' : 'Elimina únicamente el grupo vacío.'}
                        onClick={() => void openLifecycle('/groups/delete-preview','/groups/delete',{id:product})}
                      >Eliminar grupo</button>
                    </div>
                  </div>
                  {(catalog.stacks.some(s => s.product === product) || archivedStacks.some(s => s.product === product)) && <p className="hint lifecycle-hint">Eliminar grupo estará disponible cuando no contenga aplicaciones activas ni archivadas.</p>}
                  <div className="stacks">
                    {!catalog.stacks.some((s) => s.product === product) && !archivedStacks.some((s) => s.product === product) && (
                      <div className="empty small">
                        <p>
                          Grupo vacío. Selecciona aplicaciones desde Descubrir y
                          elige este grupo, o mueve una existente desde
                          Configurar.
                        </p>
                        <button onClick={() => setDiscover(true)}>
                          Añadir aplicaciones
                        </button>
                      </div>
                    )}
                    {filtered
                      .filter((s) => s.product === product)
                      .map((s) => {
                        const observed = status.stacks.find(
                          (v) => v.id === s.id,
                        );
                        const selectedMode =
                          mode[s.id] || s.activeMode || "dev";
                        const isBusy =
                          working ||
                          active.some(
                            (o) =>
                              !o.targets.length || o.targets.includes(s.id),
                          );
                        return (
                          <article className="stack" key={s.id}>
                            <div className="stack-main">
                              <div className="stack-symbol">
                                <Icon name="cube" />
                              </div>
                              <div className="stack-description">
                                <h3>
                                  {s.name}
                                  <span className="mono">{s.slug}</span>
                                </h3>
                                <p className="mono path" title={s.path}>
                                  {s.path}
                                </p>
                                <div className="stack-meta">
                                  <span className="mono">{s.projectName}</span>
                                  <Badge
                                    value={observed?.execution || "unknown"}
                                  />
                                  {observed?.execution === "running" && (
                                    <Badge value={observed.health} />
                                  )}
                                </div>
                              </div>
                              <div className="stack-mode">
                                <label>
                                  Modo
                                  <select
                                    aria-label={`Modo ${s.id}`}
                                    value={selectedMode}
                                    onChange={(e) =>
                                      setMode({
                                        ...mode,
                                        [s.id]: e.target.value,
                                      })
                                    }
                                  >
                                    <option value="dev">Desarrollo</option>
                                    {s.modes.verify && (
                                      <option value="verify">
                                        Prueba de imagen
                                      </option>
                                    )}
                                  </select>
                                </label>
                                <span className="hint">
                                  {s.activeMode
                                    ? `Último solicitado: ${s.activeMode}`
                                    : "Aún no iniciado aquí"}
                                </span>
                              </div>
                            </div>
                            <div className="stack-bottom">
                              <div className="stack-secondary">
                                <button
                                  onClick={() =>
                                    setReview({ stack: s, mode: selectedMode })
                                  }
                                >
                                  <Icon
                                    name={
                                      s.trust[selectedMode] ? "check" : "search"
                                    }
                                    size={14}
                                  />
                                  {s.trust[selectedMode]
                                    ? "Revisar"
                                    : "Revisar y aprobar"}
                                </button>
                                <button
                                  title="Editar archivos, grupo y enlaces locales; no ejecuta Docker"
                                  onClick={() => setEditor({ stack: s })}
                                >
                                  Configurar
                                </button>
                                <button
                                  disabled={!status.connected || isBusy}
                                  onClick={() =>
                                    void work(async () =>
                                      setAdoption(
                                        await api("/adoption", {
                                          target: s.id,
                                        }),
                                      ),
                                    )
                                  }
                                  title="Vincula contenedores existentes de esta aplicación tras comprobar su identidad"
                                >
                                  Vincular existentes
                                </button>
                                <button
                                  className="danger"
                                  title={['running','restarting','paused','starting'].includes(observed?.execution || '') ? 'Detén la aplicación antes de archivarla.' : 'Conserva checkout, runtime y datos; retira solo del catálogo activo.'}
                                  disabled={isBusy || !status.connected || ['running','restarting','paused','starting'].includes(observed?.execution || '')}
                                  onClick={() => void openLifecycle('/stacks/archive-preview','/stacks/archive',{target:s.id})}
                                >
                                  Archivar del catálogo
                                </button>
                              </div>
                              <div className="stack-actions">
                                <button
                                  disabled={!status.connected}
                                  onClick={() => setLogs(s)}
                                >
                                  <Icon name="terminal" size={14} />
                                  Logs
                                </button>
                                <button
                                  disabled={!status.connected || isBusy}
                                  onClick={() => void action(s, "restart")}
                                  title="No aplica cambios del Compose ni variables"
                                >
                                  <Icon name="refresh" size={14} />
                                  Reiniciar
                                </button>
                                <button
                                  disabled={!status.connected || isBusy}
                                  onClick={() => void action(s, "rebuild")}
                                >
                                  Construir
                                </button>
                                <button
                                  className="start"
                                  disabled={!status.connected || isBusy}
                                  onClick={() => void action(s, "up")}
                                >
                                  <Icon name="play" size={13} />
                                  Iniciar
                                </button>
                                <button
                                  className="stop"
                                  disabled={!status.connected || isBusy}
                                  onClick={() => void action(s, "stop")}
                                >
                                  <Icon name="stop" size={13} />
                                  Detener
                                </button>
                              </div>
                            </div>
                            <RouteLinks
                              stack={s}
                              observed={observed}
                              proxyPort={catalog.proxy?.port}
                            />
                            {!!s.links?.length && (
                              <div className="app-links">
                                <span>
                                  Accesos definidos por ti · sin comprobar
                                  conectividad
                                </span>
                                {s.links.map((l) => (
                                  <a
                                    key={l.url}
                                    href={l.url}
                                    target="_blank"
                                    rel="noopener noreferrer"
                                  >
                                    {l.label}
                                    <Icon name="link" size={13} />
                                    <small>{l.url}</small>
                                  </a>
                                ))}
                              </div>
                            )}
                            {(observed?.containers.length || 0) > 0 && (
                              <details className="container-details">
                                <summary>
                                  {observed?.containers.length} contenedores ·
                                  servicios, puertos y artefactos
                                </summary>
                                <div className="container-list">
                                  {observed?.containers.map((c) => (
                                    <div className="container" key={c.id}>
                                      <span className="mono">{c.service}</span>
                                      <Badge value={c.state} />
                                      <Badge value={c.health} />
                                      {!c.owned && (
                                        <span className="warning-text">
                                          Sin adoptar
                                        </span>
                                      )}
                                      {c.oom && (
                                        <span className="warning-text">
                                          OOM
                                        </span>
                                      )}
                                      <span className="mono muted">
                                        {c.image}
                                      </span>
                                      <div className="container-links">
                                        {c.ports
                                          .filter(
                                            (p, i, a) =>
                                              a.findIndex(
                                                (v) =>
                                                  v.port === p.port &&
                                                  v.container === p.container &&
                                                  v.host === p.host,
                                              ) === i,
                                          )
                                          .map((p) =>
                                            p.url ? (
                                              <a
                                                title={`Puerto publicado ${p.container}; protocolo web sugerido, sin comprobar conectividad`}
                                                key={`${p.container}-${p.host}-${p.port}`}
                                                href={p.url}
                                                target="_blank"
                                                rel="noreferrer noopener"
                                              >
                                                :{p.port}
                                                <Icon name="link" size={12} />
                                              </a>
                                            ) : (
                                              <span
                                                className="hint mono"
                                                title="Puerto publicado; no se asume que sea una página web"
                                                key={`${p.container}-${p.host}-${p.port}`}
                                              >
                                                {p.host || "0.0.0.0"}:{p.port} →{" "}
                                                {p.container}
                                              </span>
                                            ),
                                          )}
                                        <button onClick={() => setImage(c)}>
                                          Imagen / plataforma
                                        </button>
                                      </div>
                                    </div>
                                  ))}
                                </div>
                              </details>
                            )}
                            <div className="watch-bar">
                              <span>
                                Recarga: usa los montajes y el servidor dev del
                                proyecto. Watch solo es necesario si el Compose
                                declara develop.watch.
                              </span>
                              <button
                                disabled={
                                  !status.connected ||
                                  isBusy ||
                                  s.activeMode !== "dev"
                                }
                                onClick={() =>
                                  void work(async () => {
                                    await api("/watch", {
                                      target: s.id,
                                      action: "start",
                                    });
                                    notify(
                                      "Watch solicitado; requiere develop.watch y un Compose compatible.",
                                    );
                                  })
                                }
                              >
                                Activar Watch
                              </button>
                              {observed?.watch.state !== "off" &&
                                observed?.watch.state && (
                                  <>
                                    <span>{observed.watch.state}</span>
                                    <button
                                      onClick={() =>
                                        void work(async () => {
                                          await api("/watch", {
                                            target: s.id,
                                            action: "stop",
                                          });
                                        })
                                      }
                                    >
                                      Detener Watch
                                    </button>
                                  </>
                                )}
                              {observed?.watch.error && (
                                <span className="warning-text">
                                  {observed.watch.error.message}
                                </span>
                              )}
                            </div>
                          </article>
                       );
                      })}
                    {filteredArchived.filter(s => s.product === product).map((s: ArchivedStack) => <article className="stack archived-stack" key={s.uid}><div className="stack-main"><div className="stack-symbol"><Icon name="cube"/></div><div className="stack-description"><h3>{s.name}<span className="mono">{s.slug}</span></h3><p className="mono path" title={s.path}>{s.path}</p><div className="stack-meta"><span className="mono">{s.projectName}</span><Badge value="archived"/></div></div></div><div className="stack-bottom"><div className="stack-secondary"><span className="hint">Archivada {new Date(s.archivedAt).toLocaleString()} · {s.bindingCount} vinculaciones conservadas</span></div><div className="stack-actions"><button className="primary" disabled={working || active.length > 0} onClick={() => void openLifecycle('/stacks/restore-preview','/stacks/restore',{target:s.id})}>Restaurar en el catálogo</button></div></div></article>)}
                  </div>
                </section>
              ))}
              <section className="panel recent">
                <div className="panel-heading">
                  <Icon name="activity" />
                  <h2>Actividad reciente</h2>
                  <button
                    className="text-button"
                    onClick={() => setPage("activity")}
                  >
                    Ver toda
                  </button>
                </div>
                <OperationsView
                  operations={catalog.operations.slice(0, 3)}
                  cancel={cancel}
                />
              </section>
            </>
          )}
          {page === "infra" && (
            <InfrastructureView
              catalog={catalog}
              status={status}
              notify={notify}
            />
          )}
          {page === "tools" && (
            <ToolsView operations={catalog.operations} notify={notify} />
          )}
          {page === "proxy" && (
            <ProxyView
              catalog={catalog}
              status={status}
              notify={notify}
              onSaved={() => void load()}
              onConfigure={(s) => setEditor({ stack: s })}
            />
          )}
          {page === "runtime" && (
            <RuntimeView
              runtime={catalog.runtime}
              host={catalog.host}
              connected={status.connected}
              operations={catalog.operations}
              onSaved={() => void load()}
              notify={notify}
            />
          )}
          {page === "activity" && (
            <>
              <div className="section-heading">
                <div>
                  <h1>Actividad local</h1>
                  <p>
                    Interfaz y CLI comparten operaciones, exclusión mutua e
                    historial acotado.
                  </p>
                </div>
                <span className="count">{catalog.operations.length} / 80</span>
              </div>
              <Alert>
                Cancelar solicita detener el proceso cliente. No garantiza que
                Docker abandone una construcción ya enviada, ni revierte lo que
                alcanzó a ejecutar.
              </Alert>
              <OperationsView operations={catalog.operations} cancel={cancel} />
            </>
          )}
        </main>
        <footer className="footer">
          <span>NearProd {catalog.version} · controlador local</span>
          <span>
            {status.checkedAt
              ? `Comprobado: ${new Date(status.checkedAt).toLocaleTimeString()}`
              : "Estado pendiente"}{" "}
            · Traefik{" "}
            {status.proxy?.state === "running"
              ? "activo"
              : "sin conexión confirmada"}
          </span>
        </footer>
      </div>
      {discover && (
        <DiscoverDialog
          roots={catalog.roots}
          stacks={catalog.stacks}
          archivedStacks={archivedStacks}
          onClose={() => setDiscover(false)}
          onSelect={(cs) => {
            setDiscover(false);
            setBatch(cs);
          }}
          onRoots={() => void load()}
          onRemoveRoot={(root) => {setDiscover(false);void openLifecycle('/roots/remove-preview','/roots/remove',{root});}}
        />
      )}
      {editor && (
        <StackEditor
          {...editor}
          groups={catalog.groups}
          onClose={() => setEditor(null)}
          onSaved={() => {
            setEditor(null);
            void load();
            notify("Configuración guardada. No se ejecutó Docker.");
          }}
        />
      )}
      {batch && (
        <BatchEditor
          candidates={batch}
          groups={catalog.groups}
          onClose={() => setBatch(null)}
          onSaved={() => {
            setBatch(null);
            void load();
            notify(
              "Aplicaciones registradas en el grupo. No se ejecutó Docker.",
            );
          }}
        />
      )}
      {groupEditor && (
        <GroupDialog
          {...groupEditor}
          onClose={() => setGroupEditor(null)}
          onSaved={() => {
            setGroupEditor(null);
            void load();
            notify("Grupo guardado. No se modificó Docker.");
          }}
        />
      )}
      {lifecycleReview && <CatalogLifecycleDialog review={lifecycleReview} onClose={() => setLifecycleReview(null)} onApplied={() => {setLifecycleReview(null);void load();notify('Ciclo de vida actualizado sin borrar recursos implícitamente.');}}/>}
      {review && (
        <ReviewDialog
          {...review}
          onClose={() => setReview(null)}
          onApproved={() => {
            setReview(null);
            void load();
            notify(
              "Configuración aprobada. Ahora puedes iniciarla explícitamente.",
            );
          }}
        />
      )}
      {logs && (
        <LogsDrawer
          key={logs.id}
          stack={logs}
          observed={status.stacks.find((s) => s.id === logs.id)}
          onClose={() => setLogs(null)}
        />
      )}
      {image && (
        <ImageDialog container={image} onClose={() => setImage(null)} />
      )}
      {adoption && (
        <Modal
          title="Revisar adopción"
          subtitle={adoption.id}
          onClose={() => setAdoption(null)}
        >
          <Alert error={!adoption.allowed}>{adoption.warning}</Alert>
          {!adoption.allowed && (
            <Alert error>
              Las rutas/propietarios no coinciden. No se adoptarán estos
              recursos.
            </Alert>
          )}
          <ul>
            {adoption.containers.map((c) => (
              <li key={c.id}>
                {c.name} <code>{c.id.slice(0, 12)}</code>
              </li>
            ))}
          </ul>
          {!adoption.containers.length && (
            <p>
              No hay contenedores existentes; se vinculará la identidad del
              Engine.
            </p>
          )}
          <div className="modal-actions">
            <button onClick={() => setAdoption(null)}>Cancelar</button>
            <button
              className="primary"
              disabled={!adoption.allowed || working}
              onClick={() =>
                void work(async () => {
                  await api("/adopt", {
                    target: adoption.id,
                    fingerprint: adoption.fingerprint,
                    confirm: true,
                  });
                  setAdoption(null);
                  await load();
                  notify("Identidad adoptada explícitamente.");
                })
              }
            >
              Confirmar adopción
            </button>
          </div>
        </Modal>
      )}
    </div>
  );
}
ReactDOM.createRoot(document.getElementById('root')!).render(<App/>);
