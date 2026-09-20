import { api, message } from './api.js';
import { Alert, Icon, LiveStatus, Skeleton } from './components.js';
import type { MCPInfo } from './types.js';

const { useEffect, useState } = React;

export function McpAccessView() {
  const [info, setInfo] = useState<MCPInfo | null>(null);
  const [error, setError] = useState('');
  const [isCopied, setIsCopied] = useState(false);
  const [isLoading, setIsLoading] = useState(false);

  function loadMcpInfo() {
    setError('');
    setInfo(null);
    setIsCopied(false);
    setIsLoading(true);
    void api<MCPInfo>('/mcp').then(setInfo).catch((reason) => setError(message(reason))).finally(() => setIsLoading(false));
  }

  useEffect(() => {
    loadMcpInfo();
  }, []);

  async function handleCopyConfig() {
    if (!info) return;
    try {
      await navigator.clipboard.writeText(JSON.stringify(info.config, null, 2));
      setIsCopied(true);
      window.setTimeout(() => setIsCopied(false), 2500);
    } catch (reason) {
      setError(message(reason));
    }
  }

  return <>
    <div className="section-heading hero">
      <div>
        <span className="eyebrow">CONTROL LOCAL PARA AGENTES</span>
        <h1>Acceso MCP<span className="brand-dot">.</span></h1>
        <p>Conecta un cliente MCP directamente con NearProd mediante stdio, sin cuentas ni servicios en la nube.</p>
      </div>
      <button disabled={isLoading} onClick={loadMcpInfo}><Icon name="refresh"/>{isLoading ? 'Actualizando…' : 'Actualizar estado'}</button>
    </div>
    {error && <Alert error>{error}</Alert>}
    {!info && !error && <Skeleton label="Consultando acceso MCP" rows={6}/>}
    {info && <>
      <Alert warning><strong>Acceso local total: no requiere token ni credenciales MCP.</strong> Cualquier agente que use esta configuración podrá consultar, configurar y operar todos los recursos de este catálogo. Las acciones destructivas conservan las confirmaciones y comprobaciones de NearProd.</Alert>
      <div className="mcp-grid">
        <section className="panel">
          <div className="panel-heading"><div><span className="eyebrow">TRANSPORTE</span><h2>Servidor listo por stdio</h2></div><span className="badge badge-healthy"><span className="dot"/>Disponible</span></div>
          <p>El cliente inicia un proceso <code>nearprod mcp serve</code>. No se abre otro puerto y el proceso reutiliza el agente local como único escritor.</p>
          <dl className="mcp-facts">
            <div><dt>Binario</dt><dd><code>{info.command}</code></dd></div>
            <div><dt>Catálogo</dt><dd><code>{info.home}</code></dd></div>
            <div><dt>Transporte</dt><dd><code>{info.transport}</code></dd></div>
          </dl>
        </section>
        <section className="panel">
          <div className="panel-heading"><div><span className="eyebrow">CONFIGURACIÓN</span><h2>Añade NearProd a tu cliente</h2></div></div>
          <p>Usa este bloque en la sección de servidores MCP del cliente. No contiene secretos.</p>
          <pre className="console mcp-config">{JSON.stringify(info.config, null, 2)}</pre>
          <div className="button-row"><button className="primary" disabled={isLoading} onClick={() => void handleCopyConfig()}><Icon name="terminal"/>{isCopied ? 'Configuración copiada' : 'Copiar configuración'}</button></div>
          <LiveStatus message={isCopied ? 'Configuración MCP copiada al portapapeles.' : ''}/>
        </section>
      </div>
      <section className="panel">
        <div className="panel-heading"><div><span className="eyebrow">ALCANCE</span><h2>Capacidades disponibles</h2></div></div>
        <p className="mobile-table-hint">Desliza horizontalmente para consultar todos los detalles.</p>
        <div className="table-wrap"><table><caption className="sr-only">Capacidades disponibles mediante MCP</caption><thead><tr><th scope="col">Área</th><th scope="col">Acceso</th><th scope="col">Detalle</th></tr></thead><tbody>{info.capabilities.map((capability) => <tr key={capability.area}><td><strong>{capability.area}</strong></td><td><span className="badge badge-running"><span className="dot"/>{capability.access}</span></td><td>{capability.detail}</td></tr>)}</tbody></table></div>
      </section>
      <div className="mcp-grid">
        <section className="panel"><h2>Flujo recomendado</h2><ol className="mcp-steps"><li>Configura el servidor en tu cliente MCP.</li><li>Pide al agente que consulte <code>nearprod_overview</code>.</li><li>Revisa previews antes de aprobar Compose o confirmar cambios destructivos.</li><li>Conserva el ID de las operaciones asíncronas y consulta su resultado.</li></ol></section>
        <section className="panel"><h2>Límites que se conservan</h2><ul className="hints">{info.notes.map((note) => <li key={note}>{note}</li>)}</ul></section>
      </div>
    </>}
  </>;
}
