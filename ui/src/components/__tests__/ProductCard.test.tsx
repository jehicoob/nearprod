import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ProductCard } from '../ProductCard'
import type { ProductGroup } from '../../types'

const group: ProductGroup = {
  id: 'pos',
  label: 'POS Warehouses',
  description: 'POS local',
  status: 'running',
  running: 3,
  total: 3,
  quick_links: {
    frontend: 'http://pos.localhost',
    backend_docs: 'http://api.pos.localhost/docs',
  },
  components: [
    { id: 'pos_backend', label: 'Backend API', role: 'backend', runner: 'docker-compose', type: 'fastapi', path: '~/Trabajo/backend', resolvedPath: '/Users/me/backend', project_name: 'pos_backend', urls: { docs: 'http://api.pos.localhost/docs' }, status: 'running', exists: true, running: 2, total: 2, containers: [] },
  ],
}

describe('ProductCard', () => {
  it('renders quick links and component actions', () => {
    render(<ProductCard group={group} busyTargets={new Set()} onGroupAction={vi.fn()} onProjectAction={vi.fn()} onLogs={vi.fn()} />)
    expect(screen.getByText('POS Warehouses')).toBeInTheDocument()
    expect(screen.getByText('Backend /docs')).toHaveAttribute('href', 'http://api.pos.localhost/docs')
    expect(screen.getByText('Backend API')).toBeInTheDocument()
    expect(screen.getAllByText('Iniciar').length).toBeGreaterThan(0)
  })
})
