import { useMemo, useState } from 'react'
import { ActivityPanel } from './components/ActivityPanel'
import { AddProjectWizard } from './components/AddProjectWizard'
import { KpiGrid } from './components/KpiGrid'
import { LogsModal } from './components/LogsModal'
import { ProductCard } from './components/ProductCard'
import { TopBar } from './components/TopBar'
import { useDashboard } from './hooks/useDashboard'
import type { ProductGroup } from './types'

function groupMatches(group: ProductGroup, filter: string): boolean {
  const q = filter.trim().toLowerCase()
  if (!q) return true
  const haystack = [
    group.id,
    group.label,
    group.description,
    Object.values(group.quick_links).join(' '),
    group.components.map((component) => [component.id, component.label, component.role, component.type, component.path, Object.values(component.urls).join(' ')].join(' ')).join(' '),
  ].join(' ').toLowerCase()
  return haystack.includes(q)
}

export function App() {
  const dashboard = useDashboard()
  const [wizardOpen, setWizardOpen] = useState(false)
  const groups = useMemo(() => (dashboard.data?.groups ?? []).filter((group) => groupMatches(group, dashboard.filter)), [dashboard.data, dashboard.filter])
  const lastUpdated = dashboard.data?.generatedAt ? new Intl.DateTimeFormat('es-CO', { hour: '2-digit', minute: '2-digit', second: '2-digit' }).format(new Date(dashboard.data.generatedAt)) : undefined

  return (
    <div className="app-shell app-shell--single" id="dashboard">
      <main className="workspace">
        <TopBar
          docker={dashboard.data?.docker ?? null}
          filter={dashboard.filter}
          lastUpdated={lastUpdated}
          onFilterChange={dashboard.setFilter}
          onRefresh={() => void dashboard.refresh()}
          onOpenWizard={() => setWizardOpen(true)}
        />
        {dashboard.error ? <div className="error-box">{dashboard.error}</div> : null}
        <KpiGrid data={dashboard.data} />
        <section className="products-layout" aria-label="Productos locales">
          {dashboard.isLoading ? <div className="empty-state">Cargando proyectos...</div> : null}
          {!dashboard.isLoading && groups.length === 0 ? <div className="empty-state">No hay productos que coincidan con el filtro.</div> : null}
          {groups.map((group) => (
            <ProductCard
              key={group.id}
              group={group}
              busyTargets={dashboard.busyTargets}
              onGroupAction={(groupId, action) => void dashboard.runAction('group', groupId, action)}
              onProjectAction={(projectId, action) => void dashboard.runAction('project', projectId, action)}
              onLogs={(projectId) => void dashboard.showLogs(projectId)}
            />
          ))}
        </section>
        <ActivityPanel title={dashboard.outputTitle} output={dashboard.output} onClear={dashboard.clearOutput} />
      </main>
      <LogsModal
        open={dashboard.logViewer.open}
        title={dashboard.logViewer.title}
        projectId={dashboard.logViewer.projectId}
        output={dashboard.logViewer.output}
        isLoading={dashboard.logViewer.isLoading}
        isRefreshing={dashboard.logViewer.isRefreshing}
        error={dashboard.logViewer.error}
        lastUpdated={dashboard.logViewer.lastUpdated}
        onClose={dashboard.closeLogs}
        onRefresh={() => void dashboard.refreshLogs(false)}
        onLiveRefresh={() => void dashboard.refreshLogs(true)}
      />
      <AddProjectWizard open={wizardOpen} onClose={() => setWizardOpen(false)} onDetect={dashboard.detect} onSubmit={dashboard.addProduct} />
    </div>
  )
}
