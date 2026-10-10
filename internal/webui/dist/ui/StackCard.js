import { Badge, DisclosureButton, HealthBadge, Icon } from './components.js';
const { useState } = React;
export function StackCard({ stack, observed, archived = false, children }) {
    const [expanded, setExpanded] = useState(true);
    const detailsId = `stack-details-${stack.uid}`;
    return React.createElement("article", { className: `stack${archived ? ' archived-stack' : ''}` },
        React.createElement("div", { className: "stack-main" },
            React.createElement("div", { className: "stack-symbol" },
                React.createElement(Icon, { name: "cube" })),
            React.createElement("div", { className: "stack-description" },
                React.createElement("h3", null,
                    stack.name,
                    React.createElement("span", { className: "mono" }, stack.slug)),
                React.createElement("p", { className: "mono path", title: stack.path }, stack.path),
                React.createElement("div", { className: "stack-meta" },
                    React.createElement("span", { className: "mono" }, stack.projectName),
                    React.createElement(Badge, { value: archived ? 'archived' : observed?.execution || 'unknown' }),
                    !archived && observed?.execution === 'running' && React.createElement(HealthBadge, { value: observed.health, containers: observed.containers }),
                    stack.purgePending && React.createElement("span", { title: "Eliminaci\u00F3n incompleta; expande la tarjeta para reintentar" },
                        React.createElement(Badge, { value: "partial" })))),
            React.createElement(DisclosureButton, { expanded: expanded, controls: detailsId, label: stack.name, onToggle: () => setExpanded(value => !value) })),
        React.createElement("div", { className: "stack-details", id: detailsId, role: "region", "aria-label": `Detalles de ${stack.name}`, hidden: !expanded }, children));
}
