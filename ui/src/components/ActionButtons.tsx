import { Play, PowerOff, RotateCw, Square } from 'lucide-react'
import type { ActionName } from '../types'

export interface ActionButtonsProps {
  readonly disabled?: boolean
  readonly compact?: boolean
  readonly onAction: (action: ActionName) => void
}

const ACTIONS: ReadonlyArray<{ action: ActionName; label: string; className: string; icon: typeof Play }> = [
  { action: 'start', label: 'Iniciar', className: 'button--primary', icon: Play },
  { action: 'stop', label: 'Detener', className: 'button--warning', icon: Square },
  { action: 'restart', label: 'Reiniciar', className: 'button--secondary', icon: RotateCw },
  { action: 'down', label: 'Bajar', className: 'button--danger', icon: PowerOff },
]

export function ActionButtons({ disabled = false, compact = false, onAction }: ActionButtonsProps) {
  const size = compact ? 'button--xs' : 'button--sm'
  return (
    <div className={compact ? 'action-row action-row--compact' : 'action-row'}>
      {ACTIONS.map(({ action, label, className, icon: Icon }) => (
        <button
          key={action}
          className={`button ${className} ${size}`}
          disabled={disabled}
          onClick={() => onAction(action)}
          type="button"
        >
          <Icon size={compact ? 12 : 14} />
          <span>{label}</span>
        </button>
      ))}
    </div>
  )
}
