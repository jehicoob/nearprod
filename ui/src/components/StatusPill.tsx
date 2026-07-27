import { statusLabels, statusTone } from '../lib/dashboard'
import type { ProjectStatusValue } from '../types'

export interface StatusPillProps {
  readonly status: ProjectStatusValue
  readonly busy?: boolean
}

export function StatusPill({ status, busy = false }: StatusPillProps) {
  const tone = busy ? 'info' : statusTone(status)
  return <span className={`status-pill status-pill--${tone}`}>{busy ? 'Procesando' : statusLabels[status] ?? status}</span>
}
