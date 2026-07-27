import { Box, Link2 } from 'lucide-react'
import { ActionButtons } from './ActionButtons'
import { ComponentRow } from './ComponentRow'
import { QuickLinks } from './QuickLinks'
import { StatusPill } from './StatusPill'
import type { ActionName, ProductGroup } from '../types'

export interface ProductCardProps {
  readonly group: ProductGroup
  readonly busyTargets: ReadonlySet<string>
  readonly onGroupAction: (groupId: string, action: ActionName) => void
  readonly onProjectAction: (projectId: string, action: ActionName) => void
  readonly onLogs: (projectId: string) => void
}

export function ProductCard({ group, busyTargets, onGroupAction, onProjectAction, onLogs }: ProductCardProps) {
  const groupBusy = busyTargets.has(`group:${group.id}`)
  return (
    <section className="product-card" aria-label={group.label}>
      <header className="product-card__header">
        <div className="product-card__title">
          <span className="product-card__icon"><Box size={18} /></span>
          <div className="product-card__copy">
            <div className="product-card__heading">
              <h3>{group.label}</h3>
              <StatusPill status={group.status} busy={groupBusy} />
            </div>
            <p>{group.description || 'Producto local gestionado por Docker Compose.'}</p>
          </div>
        </div>
      </header>

      <div className="product-card__quick">
        <div className="section-label"><Link2 size={14} /> Accesos rápidos</div>
        <QuickLinks group={group} />
      </div>

      <div className="product-card__actions">
        <div>
          <span className="section-label section-label--compact">Control del producto</span>
          <ActionButtons disabled={groupBusy} onAction={(action) => onGroupAction(group.id, action)} />
        </div>
        <div className="product-card__counter">
          <strong>{group.running}/{group.total}</strong>
          <span>contenedores</span>
        </div>
      </div>

      <div className="component-list" aria-label={`Componentes de ${group.label}`}>
        {group.components.map((component) => (
          <ComponentRow
            key={component.id}
            component={component}
            busy={busyTargets.has(`project:${component.id}`)}
            onAction={onProjectAction}
            onLogs={onLogs}
          />
        ))}
      </div>
    </section>
  )
}
