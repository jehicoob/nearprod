import { Boxes, Container, TriangleAlert, Zap } from 'lucide-react'
import type { DashboardResponse } from '../types'
import { summarizeDashboard } from '../lib/dashboard'

export interface KpiGridProps {
  readonly data: DashboardResponse | null
}

export function KpiGrid({ data }: KpiGridProps) {
  const summary = summarizeDashboard(data)
  const cards = [
    { label: 'Running products', value: `${summary.runningProducts}/${summary.groups}`, helper: 'Grupos completamente activos', icon: Zap },
    { label: 'Containers', value: `${summary.runningContainers}/${summary.totalContainers}`, helper: 'Contenedores activos/totales', icon: Container },
    { label: 'Warnings', value: `${summary.warnings}`, helper: 'Parciales o desconocidos', icon: TriangleAlert },
    { label: 'Components', value: `${summary.projects}`, helper: 'Entradas en projects.yml', icon: Boxes },
  ]
  return (
    <section className="kpi-grid" aria-label="Resumen">
      {cards.map((card) => {
        const Icon = card.icon
        return (
          <article className="kpi-card" key={card.label}>
            <div className="kpi-card__label"><Icon size={15} /> {card.label}</div>
            <strong>{card.value}</strong>
            <span>{card.helper}</span>
          </article>
        )
      })}
    </section>
  )
}
