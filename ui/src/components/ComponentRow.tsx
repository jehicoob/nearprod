import { ExternalLink, FolderGit2, ScrollText } from 'lucide-react'
import { StatusPill } from './StatusPill'
import { ActionButtons } from './ActionButtons'
import type { ActionName, ProjectStatus } from '../types'
import { projectPrimaryUrl } from '../lib/dashboard'

export interface ComponentRowProps {
  readonly component: ProjectStatus
  readonly busy: boolean
  readonly onAction: (projectId: string, action: ActionName) => void
  readonly onLogs: (projectId: string) => void
}

export function ComponentRow({ component, busy, onAction, onLogs }: ComponentRowProps) {
  const url = projectPrimaryUrl(component)
  const runnable = component.runner === 'docker-compose'
  return (
    <article className="component-row">
      <div className="component-row__top">
        <div className="component-row__main">
          <span className={`component-row__role component-row__role--${component.role}`}>{component.role}</span>
          <div className="component-row__copy">
            <h4>{component.label}</h4>
            <p className="path-text"><FolderGit2 size={12} /> {component.path}</p>
          </div>
        </div>
        <div className="component-row__meta">
          <StatusPill status={component.status} busy={busy} />
          <span className="code-chip">{component.project_name}</span>
        </div>
      </div>

      <div className="component-row__bottom">
        <div className="component-row__links">
          {url ? <a href={url} target="_blank" rel="noreferrer"><ExternalLink size={14} /> Abrir servicio</a> : <span className="muted">Sin URL principal</span>}
          <button className="text-button" type="button" onClick={() => onLogs(component.id)} disabled={!runnable}><ScrollText size={14} /> Logs</button>
          <span className="code-chip code-chip--soft">{component.type}</span>
        </div>
        <ActionButtons compact disabled={busy || !runnable} onAction={(action) => onAction(component.id, action)} />
      </div>
    </article>
  )
}
