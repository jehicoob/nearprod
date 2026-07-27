import { describe, expect, it } from 'vitest'
import { buildDefaultComponent, isValidSlug, runnerForSource, slugify, validateProductPayload } from '../wizard'

describe('wizard helpers', () => {
  it('slugifies product names', () => {
    expect(slugify('Máximo Puntaje Local')).toBe('maximo_puntaje_local')
  })

  it('validates slug format', () => {
    expect(isValidSlug('pos_warehouses')).toBe(true)
    expect(isValidSlug('POS Warehouses')).toBe(false)
  })

  it('maps source types to safe runners', () => {
    expect(runnerForSource('external-url')).toBe('external')
    expect(runnerForSource('no-docker')).toBe('manual')
    expect(runnerForSource('docker-compose')).toBe('docker-compose')
  })

  it('builds default component payloads', () => {
    expect(buildDefaultComponent('demo', 'backend', 'docker-compose')).toMatchObject({
      id: 'demo_backend',
      runner: 'docker-compose',
      role: 'backend',
    })
  })

  it('requires path for non-external runners', () => {
    const errors = validateProductPayload({
      slug: 'demo',
      label: 'Demo',
      description: '',
      quick_links: {},
      components: [buildDefaultComponent('demo', 'backend', 'docker-compose')],
    })
    expect(errors.join(' ')).toContain('necesita una ruta local')
  })
})
