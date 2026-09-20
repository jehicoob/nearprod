import type { ReactNode, ReactElement } from 'react';
import { api, message } from './api.js';
import type { Operation, Stack, Preview, Container } from './types.js';
const { useState, useEffect, useRef } = React;
export function Icon({ name, size = 18 }: {name: string; size?: number}) {
  const paths: Record<string, ReactNode> = {
    cube: <><path d="m12 3 9 5v8l-9 5-9-5V8z"/><path d="m3 8 9 5 9-5M12 13v8M7.5 5.5l9 5"/></>,
    grid: <><rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/></>,
    terminal: <><rect x="3" y="4" width="18" height="16" rx="3"/><path d="m7 9 3 3-3 3m6 0h4"/></>,
    folder: <path d="M3 7V5a2 2 0 0 1 2-2h5l2 3h7a2 2 0 0 1 2 2v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7z"/>,
    settings: <><path d="M4 7h16M4 17h16"/><circle cx="8" cy="7" r="3"/><circle cx="16" cy="17" r="3"/></>,
    activity: <path d="M2 12h5l3-8 4 16 3-8h5"/>,
    refresh: <><path d="M20 7a8 8 0 1 0 0 10M20 3v5h-5"/></>,
    search: <><circle cx="10" cy="10" r="6"/><path d="m15 15 6 6"/></>,
    plus: <path d="M12 4v16M4 12h16"/>, close: <path d="m6 6 12 12M18 6 6 18"/>,
    play: <path d="m7 4 14 8-14 8z"/>, stop: <rect x="6" y="6" width="12" height="12" rx="2"/>,
    check: <path d="m4 12 5 5L20 6"/>, warning: <><path d="m12 3 10 18H2zM12 9v5m0 3v.5"/></>,
    link: <><path d="M14 3h7v7m0-7L10 14"/><path d="M10 4H4v16h16v-6"/></>,
    chevron: <path d="m8 4 8 8-8 8"/>, clock: <><circle cx="12" cy="12" r="9"/><path d="M12 6v6l4 2"/></>,
  };
  return <svg aria-hidden="true" width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round">{paths[name] || paths.cube}</svg>;
}
export function Badge({ value }: {value: string}) {
  const labels: Record<string,string> = { running: 'En ejecución', healthy: 'Saludable', unhealthy: 'No saludable', unchecked: 'Salud sin comprobar', stopped: 'Detenido', 'not-created': 'No creado', archived: 'Archivada', unknown: 'Sin conexión', failed: 'Falló', succeeded: 'Completada', cancelled: 'Cancelada', interrupted: 'Interrumpida', partial: 'Parcial', restarting: 'Reiniciando', starting: 'Iniciando', paused: 'Pausado', off: 'Apagado', dev: 'Desarrollo', verify: 'Prueba de imagen' };
  return <span className={`badge badge-${value}`}><span className="dot"/>{labels[value] || value}</span>;
}
export function Modal({ title, subtitle, children, onClose, wide = false }: {title: string; subtitle?: string; children: ReactNode; onClose: () => void; wide?: boolean}) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => { ref.current?.showModal(); }, []);
  return <dialog ref={ref} className={wide ? 'modal wide' : 'modal'} onCancel={e => {e.preventDefault(); onClose();}} onClick={e => { if (e.target === ref.current) { const r = ref.current.getBoundingClientRect(); if (e.clientX < r.left || e.clientX > r.right || e.clientY < r.top || e.clientY > r.bottom) onClose(); } }} aria-label={title}><div className="modal-heading"><div><h2>{title}</h2>{subtitle && <p>{subtitle}</p>}</div><button className="icon-button" aria-label="Cerrar diálogo" onClick={onClose}><Icon name="close"/></button></div>{children}</dialog>;
}
export function Alert({ children, error = false }: {children: ReactNode; error?: boolean}) { return <div className={`alert ${error ? 'error' : ''}`} role={error ? 'alert' : 'note'}><Icon name={error ? 'warning' : 'terminal'}/><div>{children}</div></div>; }
export function Busy() { return <div className="busy"><span className="spinner"/> Consultando…</div>; }
export function LiveStatus({ message }: {message: string}) {
  return <span className="sr-only" role="status" aria-live="polite" aria-atomic="true">{message}</span>;
}
export function OperationsView({ operations, cancel }: {operations: Operation[]; cancel: (id: string) => void}) {
  if (!operations.length) return <div className="empty small"><Icon name="activity" size={30}/><h3>La actividad aparecerá aquí</h3><p>Operaciones de la interfaz y del CLI, en un mismo lugar.</p></div>;
  return <div className="operation-list">{operations.map(op => <details className="operation" key={op.id}><summary><span className="operation-title"><Icon name="terminal"/><strong>{op.action}</strong><span className="mono muted">{op.targets.join(', ') || 'runtime'}</span></span><span className="operation-meta"><time>{new Date(op.startedAt).toLocaleTimeString()}</time><Badge value={op.state}/></span></summary><div className="operation-details"><p className="mono muted">ID: {op.id}</p>{op.error && <Alert error>{op.error.message}</Alert>}{(Array.isArray(op.results) ? op.results : []).filter(r => r.error).map(r => <Alert key={r.id} error><strong>{r.id}</strong>: {r.error?.message}</Alert>)}<pre className="console">{op.lines.map(l => l.text).join('\n') || 'Sin salida de comandos.'}</pre>{op.state === 'succeeded' && op.results && <details><summary>Resultado y rutas generadas</summary><pre className="console review-json">{JSON.stringify(op.results,null,2).slice(0,65536)}</pre></details>}{op.state === 'running' && <button className="danger" onClick={() => cancel(op.id)}>Solicitar cancelación</button>}</div></details>)}</div>;
}
export function ReviewDialog({ stack, mode, onClose, onApproved }: {stack: Stack; mode: string; onClose: () => void; onApproved: () => void}) {
  const [preview, setPreview] = useState<Preview | null>(null), [error, setError] = useState(''), [unsafe, setUnsafe] = useState(false), [busy, setBusy] = useState(false);
  useEffect(() => {const controller = new AbortController(); void api<Preview>('/preview', {target: stack.id, mode}, controller.signal).then(setPreview).catch(e => {if (!controller.signal.aborted) setError(message(e));}); return () => controller.abort();}, [stack.id, mode]);
  async function approve() { if (!preview) return; setBusy(true); try {await api('/trust', {target: stack.id, mode, fingerprint: preview.fingerprint, allowUnsafe: unsafe}); onApproved();} catch(e) {setError(message(e));} finally {setBusy(false);} }
  return <Modal title="Revisar configuración" subtitle={`${stack.id} · ${mode === 'dev' ? 'Desarrollo' : 'Verificación'}`} onClose={onClose} wide>{!preview && !error && <Skeleton label="Preparando revisión de Compose" rows={6}/>}{error && <Alert error>{error}</Alert>}{preview && <><div className="review-summary"><Badge value={mode}/><span>{preview.services.length} servicios</span><span>{preview.approved ? 'Revisión vigente' : 'Requiere aprobación'}</span></div><h3>Archivos en orden</h3><ol className="file-list">{preview.files.map(f => <li key={f} className="mono">{f}</li>)}</ol><div className="table-wrap"><table><thead><tr><th>Servicio</th><th>Imagen / construcción</th><th>Salud</th><th>Plataforma</th></tr></thead><tbody>{preview.services.map(s => <tr key={s.name}><td>{s.name}</td><td className="mono">{s.image || (s.build ? 'Dockerfile local' : 'Sin imagen')}</td><td>{s.hasHealthcheck ? 'Healthcheck' : 'Sin comprobar'}</td><td>{s.platform || 'No fijada'}</td></tr>)}</tbody></table></div>{preview.blockers.map((v,i) => <Alert error key={i}>{v}</Alert>)}{preview.risks.length > 0 && <div className="risk-box"><h3>Capacidades que debes revisar</h3><ul>{preview.risks.map((v,i) => <li key={i}>{v}</li>)}</ul><label className="check-label"><input type="checkbox" checked={unsafe} onChange={e => setUnsafe(e.target.checked)}/> He revisado y acepto estas capacidades sensibles.</label></div>}<details><summary>Advertencias y comando previsto</summary><ul className="hints">{preview.warnings.map((v,i) => <li key={i}>{v}</li>)}</ul><pre className="console">docker {preview.command.map(a => JSON.stringify(a)).join(' ')}</pre></details>{!!preview.routes?.length && <section className="review-routes"><h3>URLs que NearProd configurará</h3>{preview.routes.map(r => <p key={r.host}><code>{r.host}</code> → {r.service}:{r.port}</p>)}<p>Se añadirá un override administrado para la red de Traefik. Los Compose del repositorio no cambian; los puertos publicados se conservan. Activa el proxy desde Accesos locales.</p></section>}<Alert>{preview.note} Aprobar permite ejecutar código del repositorio con tus permisos Docker; no crea una zona aislada de confianza.</Alert><div className="modal-actions"><button onClick={onClose}>Cerrar</button><button className="primary" disabled={busy || preview.blockers.length > 0 || (preview.risks.length > 0 && !unsafe)} onClick={() => void approve()}>{busy ? 'Aprobando…' : 'Aprobar esta configuración'}</button></div></>}</Modal>;
}
export function ImageDialog({ container, onClose }: {container: Container; onClose: () => void}) {
  const [info, setInfo] = useState<{id: string; platform: string | null; digests: string[]} | null>(null), [error, setError] = useState('');
  useEffect(() => { void api<{id: string; platform: string | null; digests: string[]}>(`/images?id=${encodeURIComponent(container.imageId)}`).then(setInfo).catch(e => setError(message(e))); }, [container.imageId]);
  return <Modal title="Artefacto en ejecución" subtitle={container.name} onClose={onClose}>{error && <Alert error>{error}</Alert>}{!info && !error && <Skeleton label="Consultando imagen" rows={4}/>}{info && <><label>Imagen<input readOnly value={container.image}/></label><label>ID de imagen<input readOnly value={info.id}/></label><label>Plataforma<input readOnly value={info.platform || 'Sin comprobar'}/></label><label>Digests<textarea readOnly rows={3} value={info.digests.join('\n') || 'No disponible (una imagen local puede no tener digest de registro).'}/></label><Alert>El mismo Dockerfile o etiqueta no demuestra que sea el mismo artefacto del despliegue. ARM64 y AMD64 son plataformas diferentes.</Alert></>}</Modal>;
}

/** Stable space, screen-reader status and reduced-motion support in CSS. */
export function Skeleton({ label = 'Cargando datos', rows = 3, compact = false }: {label?: string; rows?: number; compact?: boolean}) {
  return <span className={`skeleton-block ${compact ? 'compact' : ''}`} role="status" aria-label={label} aria-busy="true"><span className="sr-only">{label}…</span>{Array.from({length: rows}, (_,i) => <span aria-hidden="true" key={i} className={`skeleton-line ${i === rows - 1 ? 'short' : ''}`}/>)}</span>;
}
export function Field({ label, help, children }: {label: string; help?: ReactNode; children: ReactElement}) {
  const id = React.useId();
  return <div className="field"><label htmlFor={id}>{label}</label>{React.cloneElement(children, {id, 'aria-describedby': help ? `${id}-help` : undefined})}{help && <p className="field-help" id={`${id}-help`}>{help}</p>}</div>;
}
