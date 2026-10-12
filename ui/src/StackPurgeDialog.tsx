import {api, message} from './api.js';
import {Alert, Busy, Field, Modal} from './components.js';
import type {Operation} from './types.js';

const {useEffect, useState} = React;
export interface StackPurgePreview {
  stack: {id: string; name: string; path: string};
  resources: {kind: string; id: string; name: string}[];
  preserved: {kind: string; name: string; reason: string}[];
  blockers: string[];
  fingerprint: string;
  retry: boolean;
  note: string;
}
const resourceLabels: Record<string, string> = {container:'Contenedor', volume:'Volumen y datos', network:'Red', image:'Imagen construida', database:'Base, cuenta y credencial', file:'Archivo generado'};

export function StackPurgeDialog({preview: initial, onClose, onApplied}: {preview: StackPurgePreview; onClose: () => void; onApplied: () => void}) {
  const [preview, setPreview] = useState(initial), [typedId, setTypedId] = useState(''), [acknowledged, setAcknowledged] = useState(false);
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [operation, setOperation] = useState<Operation | null>(null);
  const running = operation?.state === 'running' || operation?.state === 'queued';
  const succeeded = operation?.state === 'succeeded';
  const failed = operation && !running && !succeeded;

  useEffect(() => {
    if (!operation?.id || !running) return;
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout>;
    async function poll() {
      try {
        const next = await api<Operation>(`/operations/${operation!.id}`, undefined, controller.signal);
        if (!controller.signal.aborted) {setOperation(next);setError('');}
        if (!controller.signal.aborted && ['running','queued'].includes(next.state)) timer = setTimeout(() => void poll(), 800);
      } catch(e) {
        if (!controller.signal.aborted) {setError(`No se pudo consultar el progreso: ${message(e)}. La eliminación puede seguir en curso.`);timer = setTimeout(() => void poll(), 2000);}
      }
    }
    void poll();
    return () => {controller.abort();clearTimeout(timer);};
  }, [operation?.id, running]);

  async function submit() {
    setBusy(true);setError('');
    try {
      setOperation(await api<Operation>('/stacks/purge', {target:preview.stack.id, fingerprint:preview.fingerprint, confirm:true, typedId, acknowledgeDataLoss:acknowledged}));
    } catch(e) {setError(message(e));}
    finally {setBusy(false);}
  }
  async function reviewAgain() {
    setBusy(true);setError('');
    try {setPreview(await api<StackPurgePreview>('/stacks/purge-preview', {target:preview.stack.id}));setOperation(null);setTypedId('');setAcknowledged(false);}
    catch(e) {setError(message(e));}
    finally {setBusy(false);}
  }
  return <Modal title="Eliminar aplicación y recursos" subtitle={`${preview.stack.name} · ${preview.stack.id}`} wide onClose={() => {if (!busy) onClose();}}>
    {succeeded ? <Alert>Eliminación completada y verificada. Se retiraron los recursos y datos de la lista y el registro de la aplicación. El código se conservó.</Alert> : <>
      <Alert warning>{preview.note}</Alert>
      <p><strong>Se conserva el código:</strong> <span className="mono path">{preview.stack.path}</span></p>
      {preview.retry && <Alert warning>Hay una eliminación incompleta. Se comprobará cada recurso del plan original y se reintentará únicamente lo pendiente.</Alert>}
      <section className="panel"><h3>Recursos que se eliminarán ({preview.resources.length})</h3>
        {preview.resources.length ? <div className="table-wrap"><table><thead><tr><th>Tipo</th><th>Recurso</th></tr></thead><tbody>{preview.resources.map(r => <tr key={`${r.kind}:${r.id}`}><td>{resourceLabels[r.kind] || r.kind}</td><td className="mono">{r.name}</td></tr>)}</tbody></table></div> : <p>No se encontraron recursos físicos propios. Se retirarán las rutas, vinculaciones y metadata de la aplicación.</p>}
      </section>
      {!!preview.preserved.length && <details><summary>Recursos externos o independientes que se conservan</summary><ul>{preview.preserved.map((r,i) => <li key={i}><strong>{r.name}</strong>: {r.reason}</li>)}</ul></details>}
      {!!preview.blockers.length && <Alert error><strong>No se puede completar la eliminación:</strong><ul>{preview.blockers.map((b,i) => <li key={i}>{b}</li>)}</ul></Alert>}
      {!operation && <>
        <Field label="Escribe exactamente el ID de la aplicación" help={preview.stack.id}><input autoComplete="off" spellCheck={false} value={typedId} disabled={busy} onChange={e => setTypedId(e.target.value)}/></Field>
        <label className="check-label"><input type="checkbox" checked={acknowledged} disabled={busy} onChange={e => setAcknowledged(e.target.checked)}/> Entiendo que se eliminarán los datos y recursos listados y que esta operación no se puede deshacer.</label>
      </>}
    </>}
    {operation && <section aria-live="polite" className="panel"><h3>{running ? 'Eliminando recursos…' : succeeded ? 'Limpieza verificada' : 'Eliminación incompleta'}</h3>{running && <Busy/>}{operation.error && <Alert error>{operation.error.message}</Alert>}<pre className="console">{operation.lines.map(l => l.text).join('\n') || 'Comprobando el plan de eliminación…'}</pre>{running && <p className="hint">Puedes cerrar este diálogo y seguir el progreso en Actividad.</p>}</section>}
    {error && <Alert error>{error}</Alert>}
    <div className="modal-actions">
      <button disabled={busy} onClick={succeeded ? onApplied : onClose}>{succeeded ? 'Cerrar' : operation ? 'Cerrar y ver actividad' : 'Cancelar'}</button>
      {!operation && <><button disabled={busy} onClick={() => void reviewAgain()}>Actualizar revisión</button><button className="danger" disabled={busy || !!preview.blockers.length || typedId !== preview.stack.id || !acknowledged} onClick={() => void submit()}>{busy ? 'Comprobando…' : preview.retry ? 'Reintentar recursos pendientes' : 'Eliminar definitivamente'}</button></>}
      {failed && <button className="danger" disabled={busy} onClick={() => void reviewAgain()}>Revisar y reintentar recursos pendientes</button>}
    </div>
  </Modal>;
}
