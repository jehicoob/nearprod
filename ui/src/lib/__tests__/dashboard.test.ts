import { describe, expect, it } from 'vitest'
import { quickLinkEntries, summarizeDashboard } from '../dashboard'
import type { DashboardResponse, ProductGroup } from '../../types'

const baseDashboard: DashboardResponse = {
  generatedAt: new Date().toISOString(),
  root: '/tmp',
  projectsFile: '/tmp/projects.yml',
  docker: { context: 'default', dockerOk: true },
  projects: [
    { id: 'api', label: 'API', role: 'backend', runner: 'docker-compose', type: 'fastapi', path: '.', resolvedPath: '.', project_name: 'api', urls: {}, status: 'running', exists: true, running: 2, total: 2, containers: [] },
    { id: 'web', label: 'Web', role: 'frontend', runner: 'docker-compose', type: 'vite', path: '.', resolvedPath: '.', project_name: 'web', urls: {}, status: 'partial', exists: true, running: 1, total: 2, containers: [] },
  ],
  groups: [
    { id: 'product', label: 'Product', status: 'partial', quick_links: {}, components: [], running: 3, total: 4 },
  ],
}

describe('summarizeDashboard', () => {
  it('summarizes product and container counts', () => {
    expect(summarizeDashboard(baseDashboard)).toMatchObject({
      groups: 1,
      projects: 2,
      runningProducts: 0,
      runningContainers: 3,
      totalContainers: 4,
      warnings: 1,
    })
  })
})

describe('quickLinkEntries', () => {
  it('maps backend docs and frontend labels', () => {
    const group: ProductGroup = {
      id: 'x',
      label: 'X',
      status: 'running',
      running: 0,
      total: 0,
      components: [],
      quick_links: {
        frontend: 'http://app.localhost',
        backend_docs: 'http://api.localhost/docs',
      },
    }
    expect(quickLinkEntries(group).map((item) => item.label)).toEqual(['Frontend', 'Backend /docs'])
  })
})
