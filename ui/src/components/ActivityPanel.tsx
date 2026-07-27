import { Terminal, X } from 'lucide-react'

export interface ActivityPanelProps {
  readonly title: string
  readonly output: string
  readonly onClear: () => void
}

export function ActivityPanel({ title, output, onClear }: ActivityPanelProps) {
  return (
    <section className="activity-panel" aria-label="Salida de comandos">
      <header className="activity-panel__header">
        <div>
          <p className="eyebrow">Activity</p>
          <h2><Terminal size={18} /> {title}</h2>
        </div>
        <button className="button button--ghost button--xs" onClick={onClear}><X size={14} /> Limpiar</button>
      </header>
      <pre className="activity-panel__output">{output}</pre>
    </section>
  )
}
