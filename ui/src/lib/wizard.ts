import type { NewComponentPayload, NewProductPayload, Runner } from '../types'

export const sourceOptions = [
  { id: 'local-folder', label: 'Local Folder', description: 'Registrar una carpeta local existente.' },
  { id: 'github-clone', label: 'Clone from GitHub', description: 'Registrar un repositorio clonado o pendiente de revisión.' },
  { id: 'docker-compose', label: 'Existing Docker Compose', description: 'Usar un docker-compose.yml existente.' },
  { id: 'no-docker', label: 'Project without Docker', description: 'Registrar como manual hasta dockerizarlo.' },
  { id: 'external-url', label: 'External URL', description: 'Monitorear un endpoint externo sin runner local.' },
] as const

export function slugify(input: string): string {
  return input
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '')
}

export function isValidSlug(slug: string): boolean {
  return /^[a-z0-9][a-z0-9_-]*$/.test(slug)
}

export function runnerForSource(sourceType: string, detectedRunner?: string): Runner | string {
  if (sourceType === 'external-url') return 'external'
  if (sourceType === 'no-docker') return 'manual'
  if (sourceType === 'docker-compose') return 'docker-compose'
  return detectedRunner || 'manual'
}

export function buildDefaultComponent(productSlug: string, role: string, sourceType: string): NewComponentPayload {
  const id = `${productSlug}_${role || 'service'}`
  return {
    id,
    label: role === 'frontend' ? 'Frontend App' : role === 'backend' ? 'Backend API' : 'Service',
    role: role || 'service',
    runner: runnerForSource(sourceType),
    source_type: sourceType,
    path: '',
    project_name: id,
    type: 'manual',
    urls: {},
    trusted: sourceType !== 'github-clone',
  }
}

export function validateProductPayload(payload: NewProductPayload): string[] {
  const errors: string[] = []
  if (!isValidSlug(payload.slug)) errors.push('El slug del producto no es válido.')
  if (!payload.label.trim()) errors.push('El nombre del producto es obligatorio.')
  if (!payload.components.length) errors.push('Agrega al menos un componente.')
  for (const component of payload.components) {
    if (!isValidSlug(component.id)) errors.push(`El id del componente ${component.id || '(vacío)'} no es válido.`)
    if (component.runner !== 'external' && !component.path.trim()) errors.push(`El componente ${component.id} necesita una ruta local.`)
    if (component.runner === 'external' && !Object.values(component.urls).some(Boolean)) errors.push(`El componente externo ${component.id} necesita una URL.`)
  }
  return errors
}
