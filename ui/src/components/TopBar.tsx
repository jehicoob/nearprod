import { Plus, RefreshCw, Search } from 'lucide-react'
import type { DockerSummary } from '../types'

export interface TopBarProps {
  readonly docker: DockerSummary | null
  readonly filter: string
  readonly lastUpdated?: string
  readonly onFilterChange: (value: string) => void
  readonly onRefresh: () => void
  readonly onOpenWizard: () => void
}

export function TopBar({ docker, filter, lastUpdated, onFilterChange, onRefresh, onOpenWizard }: TopBarProps) {
  return (
    <header className="topbar">
      <div className="topbar__title">
        <p className="eyebrow">Colima · Docker Compose · Traefik</p>
        <h1>Local Infra Control</h1>
      </div>
      <label className="search-box">
        <Search size={16} />
        <input value={filter} onChange={(event) => onFilterChange(event.target.value)} placeholder="Filtrar por producto, ruta, rol o dominio..." />
      </label>
      <div className="topbar__right">
        <div className="topbar__status" title={docker?.error ?? undefined}>
          <span className={docker?.dockerOk ? 'dot dot--success' : 'dot dot--danger'} />
          <span>{docker?.serverVersion ? `Docker ${docker.serverVersion}` : 'Docker'}</span>
          {lastUpdated ? <span className="muted">{lastUpdated}</span> : null}
        </div>
        <button className="button button--secondary button--sm" onClick={onRefresh} type="button"><RefreshCw size={15} /> Refrescar</button>
        <button className="button button--primary button--sm" onClick={onOpenWizard} type="button"><Plus size={15} /> Agregar</button>
      </div>
    </header>
  )
}
