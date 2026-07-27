import type { DashboardResponse, ProductGroup, ProjectStatus, ProjectStatusValue } from '../types'

export const statusLabels: Record<ProjectStatusValue, string> = {
  running: 'Activo',
  partial: 'Parcial',
  stopped: 'Detenido',
  not_created: 'Sin crear',
  unknown: 'Desconocido',
  manual: 'Manual',
  external: 'Externo',
}

export function statusTone(status: ProjectStatusValue): 'success' | 'warning' | 'danger' | 'neutral' | 'info' {
  if (status === 'running') return 'success'
  if (status === 'partial') return 'warning'
  if (status === 'unknown') return 'danger'
  if (status === 'manual' || status === 'external') return 'info'
  return 'neutral'
}

export function summarizeDashboard(data: DashboardResponse | null) {
  const groups = data?.groups ?? []
  const projects = data?.projects ?? []
  const runningProducts = groups.filter((group) => group.status === 'running').length
  const runningContainers = projects.reduce((sum, project) => sum + (project.running || 0), 0)
  const totalContainers = projects.reduce((sum, project) => sum + (project.total || 0), 0)
  const warnings = groups.filter((group) => ['partial', 'unknown'].includes(group.status)).length
  const stopped = groups.filter((group) => ['stopped', 'not_created'].includes(group.status)).length
  return { groups: groups.length, projects: projects.length, runningProducts, runningContainers, totalContainers, warnings, stopped }
}

export function quickLinkEntries(group: ProductGroup): Array<{ readonly label: string; readonly href: string; readonly kind: string }> {
  const labels: Record<string, string> = {
    frontend: 'Frontend',
    backend_docs: 'Backend /docs',
    docs: 'Docs',
    api: 'API',
    app: 'App',
  }
  return Object.entries(group.quick_links ?? {}).map(([key, href]) => ({ label: labels[key] ?? key, href, kind: key }))
}

export function projectPrimaryUrl(project: ProjectStatus): string | null {
  if (project.urls.frontend) return project.urls.frontend
  if (project.urls.app) return project.urls.app
  if (project.urls.docs) return project.urls.docs
  if (project.urls.api) return project.urls.api
  return null
}
