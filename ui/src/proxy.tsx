import { suggestedHost } from './route-suggestions.js';
import { api, message } from './api.js';
import { Alert, Field, Icon, LiveStatus, Modal, Skeleton } from './components.js';
import type { Catalog, Status, Stack, StackDraft, ProjectOptions, WebRoute, Observed, ProxyPreview, RouteCheck } from './types.js';
const { useState, useEffect } = React;
export function RouteEditor({ value, onChange, initialOptions }: {value: StackDraft; onChange: (v: StackDraft) => void; initialOptions: ProjectOptions | null}) {
  const [options,setOptions] = useState(initialOptions), [loading,setLoading] = useState(false), [error,setError] = useState('');
  const selected = JSON.stringify(value.modes.dev.files), routes = value.routes || [];
  useEffect(() => {
    if (!value.path) return;
    const controller = new AbortController(); setLoading(true);
    void api<ProjectOptions>('/project-options',{path:value.path,files:value.modes.dev.files},controller.signal).then(v => {if(!controller.signal.aborted) {setOptions(v);setError('');}}).catch(e => {if(!controller.signal.aborted) setError(message(e));}).finally(() => {if(!controller.signal.aborted) setLoading(false);});
    return () => controller.abort();
  },[value.path,selected]);
  const update = (i: number, patch: Partial<WebRoute>) => onChange({...value,routes:routes.map((r,j) => j === i ? {...r,...patch} : r)});
  const add = () => {
    const hint = options?.httpHints?.find(h => h.suggestedPort), service = hint?.service || options?.services[0] || '';
    onChange({...value,routes:[...routes,{host:suggestedHost(value,service,routes.length),service,port:hint?.suggestedPort || 80}]});
  };
  return <section className="route-editor"><div className="panel-heading"><Icon name="link"/><h3>URL de acceso — Traefik</h3></div><LiveStatus message={loading ? 'Leyendo servicios para la URL.' : ''}/>
    <p>Configura aquí el acceso de la aplicación. NearProd preparará la ruta y conectará el servicio al proxy al <strong>Iniciar</strong>, sin editar los Compose originales. Activa el proxy una vez desde <strong>Accesos locales</strong>.</p>
    <p className="field-help">Frontend y API pueden tener URLs distintas aunque estén en el mismo Compose. Bases de datos y workers sin HTTP no necesitan URL. El proxy usa HTTP local; no instala certificados ni modifica DNS.</p>
    {!options && loading && <Skeleton label="Detectando servicios para la URL" rows={2}/>}
    {routes.map((route,i) => <div className="route-form" key={i}>
      <Field label={`Dominio local ${i+1}`} help="Nombre completo, sin http:// ni puerto. Ejemplo: proyecto.localhost; para una API, api-proyecto.localhost. Debe ser único en el catálogo."><input required value={route.host} onChange={e => update(i,{host:e.target.value.toLowerCase()})} placeholder="maximopuntaje.localhost"/></Field>
      <div className="form-grid"><Field label={`Servicio HTTP ${i+1}`} help="Servicio que responde a peticiones web. En PHP, selecciona Nginx/Apache, no PHP-FPM ni el worker."><select required value={route.service} onChange={e => {const hint = options?.httpHints?.find(v => v.service === e.target.value);update(i,{service:e.target.value,port:hint?.suggestedPort || route.port});}}><option value="">Seleccionar servicio</option>{[...new Set([...(options?.services || []),route.service].filter(Boolean))].map(s => <option value={s} key={s}>{s}</option>)}</select></Field>
      <Field label={`Puerto HTTP interno ${i+1}`} help="Puerto DENTRO del contenedor: por ejemplo 8000 para Uvicorn o 80 para Nginx. No es el puerto publicado en el host. El servidor debe escuchar en 0.0.0.0."><input type="number" required min={1} max={65535} value={route.port || ''} onChange={e => update(i,{port:Number(e.target.value)})}/></Field></div>
      {options?.httpHints?.find(v => v.service === route.service)?.note.includes('FPM') && <Alert error>PHP-FPM habla FastCGI, no HTTP. Debes seleccionar un servicio web delante de FPM.</Alert>}
      {value.modes.verify && <details className="advanced"><summary>Destino diferente en prueba de imagen</summary><p className="field-help">Por defecto se usa el mismo servicio y puerto. Si Vite usa 5173 en desarrollo y Nginx 80 al probar la imagen, configura aquí ese destino.</p><label className="check-label"><input type="checkbox" checked={Boolean(route.verify)} onChange={e => update(i,{verify:e.target.checked ? {service:route.service,port:route.port} : undefined})}/>Cambiar destino para prueba de imagen</label>{route.verify && <div className="form-grid"><Field label={`Servicio en prueba de imagen ${i+1}`} help="Nombre tal como aparece en los Compose de verificación."><input required value={route.verify.service} onChange={e => update(i,{verify:{...route.verify!,service:e.target.value}})}/></Field><Field label={`Puerto en prueba de imagen ${i+1}`} help="Puerto HTTP interno de ese servicio, no el publicado en el host."><input type="number" min={1} max={65535} required value={route.verify.port} onChange={e => update(i,{verify:{...route.verify!,port:Number(e.target.value)}})}/></Field></div>}</details>}
      <div className="route-preview"><code>http://{route.host}</code><span>→ {route.service || 'servicio'}:{route.port || '?'}</span><button type="button" onClick={() => onChange({...value,routes:routes.filter((_,j) => j !== i)})}>Quitar URL</button></div>
    </div>)}
    {!routes.length && <p className="hint">Sin URL todavía. Añádela para una aplicación web, o continúa sin URL si es únicamente DB/worker.</p>}
    <button type="button" disabled={routes.length >= 8 || loading} onClick={add}><Icon name="plus" size={14}/>{routes.length ? 'Añadir otra URL' : 'Crear URL sugerida'}</button>
    <p className="hint">Las sugerencias no prueban que haya un servidor HTTP. Se conservan tus puertos publicados, redes, datos y variables. Si el proxy usa un puerto distinto de 80, NearProd lo añadirá al enlace.</p>{error && <Alert error>{error}</Alert>}
  </section>;
}
const routeState: Record<string,string> = {pending:'Pendiente de Revisar e Iniciar',unknown:'Estado sin comprobar','proxy-unavailable':'Proxy no disponible','application-stopped':'Servicio detenido','network-missing':'Falta conexión de red · Iniciar de nuevo','ready-to-check':'Conectado a la red · HTTP sin comprobar'};
const checkState: Record<string,string> = {'pending':'La ruta todavía no se aplicó','connection-failed':'El puerto del proxy no responde','route-not-loaded':'Traefik no cargó esta ruta','upstream-error':'El servidor HTTP interno no responde','http-response':'La ruta devolvió una respuesta HTTP'};
export function RouteLinks({ stack, observed, proxyPort = 80 }: {stack: Stack; observed?: Observed; proxyPort?: number}) {
  const [checking,setChecking] = useState(''), [checks,setChecks] = useState<Record<string,RouteCheck>>({}), [error,setError] = useState('');
  useEffect(() => {setChecks({});setError('');},[stack.id,JSON.stringify(stack.routes),proxyPort]);
  async function check(host: string) {setChecking(host);setError('');try {const result = await api<RouteCheck>('/proxy/check',{target:stack.id,host});setChecks(c => ({...c,[host]:result}));} catch(e) {setError(message(e));} finally {setChecking('');}}
  if (!stack.routes?.length) return null;
  return <div className="managed-routes"><strong><Icon name="link" size={15}/>Acceso por Traefik</strong>{stack.routes.map(route => {
    const observedRoute = observed?.routes?.find(r => r.host === route.host), result = checks[route.host], url = observedRoute?.url || `http://${route.host}${proxyPort === 80 ? '' : ':'+proxyPort}`;
    return <div className="managed-route" key={route.host}><div className="route-title"><a href={url} target="_blank" rel="noopener noreferrer">{url}<Icon name="link" size={12}/></a><button disabled={Boolean(checking)} onClick={() => void check(route.host)}>{checking === route.host ? 'Comprobando…' : 'Comprobar acceso'}</button></div><p className="hint">{routeState[observedRoute?.state || 'pending']} · {observedRoute?.service || route.service}:{observedRoute?.port || route.port}</p>
      {result && <div className="route-check" role="status"><strong>{checkState[result.state] || result.state}{result.response.status ? ` · HTTP ${result.response.status}` : ''}</strong><span>Comprobado {new Date(result.checkedAt).toLocaleTimeString()}. No certifica la salud de la aplicación.</span><span>DNS del sistema: {result.dns.state === 'loopback' ? 'loopback ('+result.dns.addresses.join(', ')+')' : result.dns.state === 'unresolved' ? 'no resuelto; el navegador puede tratar .localhost por separado' : 'no apunta exclusivamente a loopback; revisa el DNS'}</span>{result.dns.state !== 'loopback' && <details><summary>Alternativa cuando tu navegador no resuelve el dominio</summary><p>Añade manualmente esta entrada a /etc/hosts solo después de comprobar el nombre. NearProd no modifica ese archivo ni necesita sudo:</p><code>{result.hostsEntry}</code><p>No añade comodines; una entrada por nombre. Nunca uses una IP externa.</p></details>}</div>}
    </div>;
  })}{error && <Alert error>{error}</Alert>}</div>;
}
export function ProxyView({
  catalog,
  status,
  notify,
  onSaved,
  onConfigure,
}: {
  catalog: Catalog;
  status: Status;
  notify: (message: string, error?: boolean) => void;
  onSaved: () => void;
  onConfigure: (s: Stack) => void;
}) {
  const [port, setPort] = useState(catalog.proxy?.port || 80),
    [preview, setPreview] = useState<ProxyPreview | null>(null),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const proxy = status.proxy,
    active = catalog.operations.some((o) => o.state === "running"),
    stacks = catalog.stacks.filter((s) => s.routes?.length);
  const titles: Record<string, string> = {
    "not-created": "Aún no creado",
    unknown: "Estado sin comprobar",
    running: "Traefik activo",
    starting: "Iniciando o sin salud confirmada",
    unhealthy: "No saludable",
    stopped: "Traefik detenido",
    conflict: "Conflicto de identidad",
  };
  async function task(f: () => Promise<void>) {
    setBusy(true);
    setError("");
    try {
      await f();
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="runtime-view">
      <div className="section-heading">
        <div>
          <h1>Accesos locales</h1>
          <p>
            Una URL por servicio web. Un solo Traefik compartido dentro de {" "}
            {catalog.host.runtime.displayName}.
          </p>
        </div>
        <button
          disabled={busy}
          onClick={() =>
            void task(async () => {
              await api("/status?refresh=1");
              onSaved();
            })
          }
        >
          <Icon name="refresh" />
          Actualizar estado
        </button>
      </div>
      <section className="panel">
        <div className="panel-heading">
          <Icon name="link" />
          <h2>
            {proxy ? titles[proxy.state] || proxy.state : "Comprobando proxy"}
          </h2>
        </div>
        {!proxy ? (
          <Skeleton label="Consultando Traefik" rows={3} />
        ) : (
          <>
            <p>
              NearProd administra <code>{proxy.image}</code> como contenedor. El
              panel se ejecuta en {catalog.host.displayName}; el proxy no crea ni
              administra otra máquina virtual.
            </p>
            <div className="form-grid">
              <Field
                label="Puerto de acceso local"
                help="80 permite URLs sin puerto. Si otro proxy lo ocupa o no tienes permisos, usa 8080: los enlaces incluirán :8080. No se detienen servicios ajenos."
              >
                <input
                  type="number"
                  min={1}
                  max={65535}
                  value={port}
                  onChange={(e) => setPort(Number(e.target.value))}
                />
              </Field>
              <div className="proxy-explainer">
                <strong>Dominios recomendados</strong>
                <code>proyecto.localhost</code>
                <code>api-proyecto.localhost</code>
                <span>
                  Accesos locales cortos: proyecto.localhost y
                  api-proyecto.localhost.
                </span>
              </div>
            </div>
            <div className="button-row">
              <button
                className="primary"
                disabled={busy || active || !status.connected || !catalog.host.proxy.supported}
                onClick={() =>
                  void task(async () =>
                    setPreview(await api("/proxy/preview", { port })),
                  )
                }
              >
                {proxy.state === "running"
                  ? "Revisar cambio / reparar proxy"
                  : "Revisar activación de Traefik"}
              </button>
              <button
                disabled={
                  busy ||
                  active ||
                  !status.connected ||
                  !catalog.host.proxy.supported ||
                  !["running", "starting", "unhealthy"].includes(proxy.state)
                }
                onClick={() => {
                  if (
                    window.confirm(
                      "¿Detener Traefik? Todas sus URLs dejarán de responder. Tus aplicaciones y datos se conservan.",
                    )
                  )
                    void task(async () => {
                      await api("/proxy/actions", {
                        action: "stop",
                        confirm: true,
                      });
                      notify(
                        "Detención del proxy solicitada. Las aplicaciones no se detienen.",
                      );
                    });
                }}
              >
                Detener proxy
              </button>
            </div>
          </>
        )}
        {!status.connected && (
          <Alert>
            Inicia el runtime desde Runtime y recursos. Registrar URLs no
            requiere Engine, pero activar Traefik sí.
          </Alert>
        )}
        {!catalog.host.proxy.supported && (
          <Alert error>
            El proxy local no está soportado en {catalog.host.displayName}.
          </Alert>
        )}
        <p className="hint">
          HTTP local, enlace de escucha 127.0.0.1. Sin dashboard expuesto, sin
          socket Docker dentro del proxy, sin cambios en /etc/hosts, DNS o
          certificados. La red compartida solo se añade a los servicios HTTP
          elegidos; esos servicios pueden comunicarse entre sí.
        </p>
        {error && <Alert error>{error}</Alert>}
      </section>
      <section className="panel">
        <div className="panel-heading">
          <Icon name="grid" />
          <h2>URLs de tus aplicaciones</h2>
        </div>
        <p>
          Define la URL desde <strong>Configurar → URL de acceso</strong>.
          Después revisa, aprueba e inicia esa aplicación para aplicar la
          conexión. Guardar una URL no recrea contenedores silenciosamente.
        </p>
        {stacks.length ? (
          stacks.map((s) => (
            <div className="proxy-stack" key={s.id}>
              <div className="panel-heading">
                <h3>{s.name}</h3>
                <code>{s.id}</code>
                <button onClick={() => onConfigure(s)}>Configurar URLs</button>
              </div>
              <RouteLinks
                stack={s}
                observed={status.stacks.find((v) => v.id === s.id)}
                proxyPort={catalog.proxy?.port}
              />
            </div>
          ))
        ) : (
          <Alert>
            Aún no hay URLs administradas. Configura una aplicación web;
            DB/Redis/workers sin HTTP no necesitan una.
          </Alert>
        )}
        {catalog.stacks
          .filter((s) => !s.routes?.length)
          .map((s) => (
            <div className="proxy-pending" key={s.id}>
              <span>
                {s.name}
                <small>{s.id}</small>
              </span>
              <button onClick={() => onConfigure(s)}>Configurar acceso</button>
            </div>
          ))}
      </section>
      {preview && (
        <Modal
          title="Activar o actualizar Traefik"
          subtitle="Cambio global del punto de acceso local, no del runtime."
          onClose={() => setPreview(null)}
        >
          <p>{preview.note}</p>
          <p>
            <strong>{preview.image}</strong> · 127.0.0.1:{preview.port}
          </p>
          <p>
            {preview.impacted.length} aplicaciones con URL:{" "}
            {preview.impacted.join(", ") || "ninguna todavía"}.
          </p>
          <Alert>
            La primera activación descarga la imagen. Activar/reparar recrea el
            proxy y puede interrumpir brevemente todas las URLs; cambiar el
            puerto también cambia los enlaces y obliga a actualizar URLs de
            API/CORS en tus aplicaciones; NearProd no modifica esas variables.
          </Alert>
          {preview.blockers.map((b) => (
            <Alert error key={b}>
              {b}
            </Alert>
          ))}
          {error && <Alert error>{error}</Alert>}
          <div className="modal-actions">
            <button onClick={() => setPreview(null)}>Cancelar</button>
            <button
              className="primary"
              disabled={busy || active || Boolean(preview.blockers.length)}
              onClick={() =>
                void task(async () => {
                  await api("/proxy/actions", {
                    ...preview,
                    action: "start",
                    confirm: true,
                  });
                  setPreview(null);
                  notify(
                    "Traefik solicitado. Consulta Actividad y comprueba el estado antes de abrir las URLs.",
                  );
                  onSaved();
                })
              }
            >
              Confirmar y activar Traefik
            </button>
          </div>
        </Modal>
      )}
    </div>
  );
}
