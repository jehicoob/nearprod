import type {ReactNode} from 'react';
import {Badge, DisclosureButton, HealthBadge, Icon} from './components.js';
import type {ArchivedStack, Observed, Stack} from './types.js';

const {useState} = React;

export function StackCard({stack, observed, archived = false, children}: {stack: Stack | ArchivedStack; observed?: Observed; archived?: boolean; children: ReactNode}) {
  const [expanded, setExpanded] = useState(true);
  const detailsId = `stack-details-${stack.uid}`;
  return <article className={`stack${archived ? ' archived-stack' : ''}`}>
    <div className="stack-main">
      <div className="stack-symbol"><Icon name="cube"/></div>
      <div className="stack-description">
        <h3>{stack.name}<span className="mono">{stack.slug}</span></h3>
        <p className="mono path" title={stack.path}>{stack.path}</p>
        <div className="stack-meta">
          <span className="mono">{stack.projectName}</span>
          <Badge value={archived ? 'archived' : observed?.execution || 'unknown'}/>
          {!archived && observed?.execution === 'running' && <HealthBadge value={observed.health} containers={observed.containers}/>}
          {stack.purgePending && <span title="Eliminación incompleta; expande la tarjeta para reintentar"><Badge value="partial"/></span>}
        </div>
      </div>
      <DisclosureButton expanded={expanded} controls={detailsId} label={stack.name} onToggle={() => setExpanded(value => !value)}/>
    </div>
    <div className="stack-details" id={detailsId} role="region" aria-label={`Detalles de ${stack.name}`} hidden={!expanded}>{children}</div>
  </article>;
}
