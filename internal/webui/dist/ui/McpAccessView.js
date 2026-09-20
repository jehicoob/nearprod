import { api, message } from './api.js';
import { Alert, Icon, LiveStatus, Skeleton } from './components.js';
const { useEffect, useState } = React;
export function McpAccessView() {
    const [info, setInfo] = useState(null);
    const [error, setError] = useState('');
    const [isCopied, setIsCopied] = useState(false);
    const [isLoading, setIsLoading] = useState(false);
    function loadMcpInfo() {
        setError('');
        setInfo(null);
        setIsCopied(false);
        setIsLoading(true);
        void api('/mcp').then(setInfo).catch((reason) => setError(message(reason))).finally(() => setIsLoading(false));
    }
    useEffect(() => {
        loadMcpInfo();
    }, []);
    async function handleCopyConfig() {
        if (!info)
            return;
        try {
            await navigator.clipboard.writeText(JSON.stringify(info.config, null, 2));
            setIsCopied(true);
            window.setTimeout(() => setIsCopied(false), 2500);
        }
        catch (reason) {
            setError(message(reason));
        }
    }
    return React.createElement(React.Fragment, null,
        React.createElement("div", { className: "section-heading hero" },
            React.createElement("div", null,
                React.createElement("span", { className: "eyebrow" }, "CONTROL LOCAL PARA AGENTES"),
                React.createElement("h1", null,
                    "Acceso MCP",
                    React.createElement("span", { className: "brand-dot" }, ".")),
                React.createElement("p", null, "Conecta un cliente MCP directamente con NearProd mediante stdio, sin cuentas ni servicios en la nube.")),
            React.createElement("button", { disabled: isLoading, onClick: loadMcpInfo },
                React.createElement(Icon, { name: "refresh" }),
                isLoading ? 'Actualizando…' : 'Actualizar estado')),
        error && React.createElement(Alert, { error: true }, error),
        !info && !error && React.createElement(Skeleton, { label: "Consultando acceso MCP", rows: 6 }),
        info && React.createElement(React.Fragment, null,
            React.createElement(Alert, { warning: true },
                React.createElement("strong", null, "Acceso local total: no requiere token ni credenciales MCP."),
                " Cualquier agente que use esta configuraci\u00F3n podr\u00E1 consultar, configurar y operar todos los recursos de este cat\u00E1logo. Las acciones destructivas conservan las confirmaciones y comprobaciones de NearProd."),
            React.createElement("div", { className: "mcp-grid" },
                React.createElement("section", { className: "panel" },
                    React.createElement("div", { className: "panel-heading" },
                        React.createElement("div", null,
                            React.createElement("span", { className: "eyebrow" }, "TRANSPORTE"),
                            React.createElement("h2", null, "Servidor listo por stdio")),
                        React.createElement("span", { className: "badge badge-healthy" },
                            React.createElement("span", { className: "dot" }),
                            "Disponible")),
                    React.createElement("p", null,
                        "El cliente inicia un proceso ",
                        React.createElement("code", null, "nearprod mcp serve"),
                        ". No se abre otro puerto y el proceso reutiliza el agente local como \u00FAnico escritor."),
                    React.createElement("dl", { className: "mcp-facts" },
                        React.createElement("div", null,
                            React.createElement("dt", null, "Binario"),
                            React.createElement("dd", null,
                                React.createElement("code", null, info.command))),
                        React.createElement("div", null,
                            React.createElement("dt", null, "Cat\u00E1logo"),
                            React.createElement("dd", null,
                                React.createElement("code", null, info.home))),
                        React.createElement("div", null,
                            React.createElement("dt", null, "Transporte"),
                            React.createElement("dd", null,
                                React.createElement("code", null, info.transport))))),
                React.createElement("section", { className: "panel" },
                    React.createElement("div", { className: "panel-heading" },
                        React.createElement("div", null,
                            React.createElement("span", { className: "eyebrow" }, "CONFIGURACI\u00D3N"),
                            React.createElement("h2", null, "A\u00F1ade NearProd a tu cliente"))),
                    React.createElement("p", null, "Usa este bloque en la secci\u00F3n de servidores MCP del cliente. No contiene secretos."),
                    React.createElement("pre", { className: "console mcp-config" }, JSON.stringify(info.config, null, 2)),
                    React.createElement("div", { className: "button-row" },
                        React.createElement("button", { className: "primary", disabled: isLoading, onClick: () => void handleCopyConfig() },
                            React.createElement(Icon, { name: "terminal" }),
                            isCopied ? 'Configuración copiada' : 'Copiar configuración')),
                    React.createElement(LiveStatus, { message: isCopied ? 'Configuración MCP copiada al portapapeles.' : '' }))),
            React.createElement("section", { className: "panel" },
                React.createElement("div", { className: "panel-heading" },
                    React.createElement("div", null,
                        React.createElement("span", { className: "eyebrow" }, "ALCANCE"),
                        React.createElement("h2", null, "Capacidades disponibles"))),
                React.createElement("p", { className: "mobile-table-hint" }, "Desliza horizontalmente para consultar todos los detalles."),
                React.createElement("div", { className: "table-wrap" },
                    React.createElement("table", null,
                        React.createElement("caption", { className: "sr-only" }, "Capacidades disponibles mediante MCP"),
                        React.createElement("thead", null,
                            React.createElement("tr", null,
                                React.createElement("th", { scope: "col" }, "\u00C1rea"),
                                React.createElement("th", { scope: "col" }, "Acceso"),
                                React.createElement("th", { scope: "col" }, "Detalle"))),
                        React.createElement("tbody", null, info.capabilities.map((capability) => React.createElement("tr", { key: capability.area },
                            React.createElement("td", null,
                                React.createElement("strong", null, capability.area)),
                            React.createElement("td", null,
                                React.createElement("span", { className: "badge badge-running" },
                                    React.createElement("span", { className: "dot" }),
                                    capability.access)),
                            React.createElement("td", null, capability.detail))))))),
            React.createElement("div", { className: "mcp-grid" },
                React.createElement("section", { className: "panel" },
                    React.createElement("h2", null, "Flujo recomendado"),
                    React.createElement("ol", { className: "mcp-steps" },
                        React.createElement("li", null, "Configura el servidor en tu cliente MCP."),
                        React.createElement("li", null,
                            "Pide al agente que consulte ",
                            React.createElement("code", null, "nearprod_overview"),
                            "."),
                        React.createElement("li", null, "Revisa previews antes de aprobar Compose o confirmar cambios destructivos."),
                        React.createElement("li", null, "Conserva el ID de las operaciones as\u00EDncronas y consulta su resultado."))),
                React.createElement("section", { className: "panel" },
                    React.createElement("h2", null, "L\u00EDmites que se conservan"),
                    React.createElement("ul", { className: "hints" }, info.notes.map((note) => React.createElement("li", { key: note }, note)))))));
}
