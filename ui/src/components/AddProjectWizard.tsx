import { useMemo, useState } from 'react'
import { AlertTriangle, Check, ChevronRight, Folder, Github, Link, Server, ShieldAlert, X } from 'lucide-react'
import type { DetectResult, NewComponentPayload, NewProductPayload } from '../types'
import { buildDefaultComponent, runnerForSource, slugify, sourceOptions, validateProductPayload } from '../lib/wizard'

export interface AddProjectWizardProps {
  readonly open: boolean
  readonly onClose: () => void
  readonly onDetect: (path: string) => Promise<DetectResult>
  readonly onSubmit: (payload: NewProductPayload) => Promise<void>
}

const icons = {
  'local-folder': Folder,
  'github-clone': Github,
  'docker-compose': Server,
  'no-docker': ShieldAlert,
  'external-url': Link,
}

export function AddProjectWizard({ open, onClose, onDetect, onSubmit }: AddProjectWizardProps) {
  const [step, setStep] = useState(0)
  const [sourceType, setSourceType] = useState('local-folder')
  const [label, setLabel] = useState('')
  const [slug, setSlug] = useState('')
  const [description, setDescription] = useState('')
  const [components, setComponents] = useState<NewComponentPayload[]>([])
  const [detectingId, setDetectingId] = useState<string | null>(null)
  const [detections, setDetections] = useState<Record<string, DetectResult>>({})
  const [error, setError] = useState<string | null>(null)

  const effectiveSlug = slug || slugify(label)
  const payload = useMemo<NewProductPayload>(() => ({
    slug: effectiveSlug,
    label,
    description,
    quick_links: {},
    components,
  }), [effectiveSlug, label, description, components])
  const validationErrors = validateProductPayload(payload)

  if (!open) return null

  const addComponent = (role: string) => {
    const productSlug = effectiveSlug || 'new_product'
    setComponents((current) => [...current, buildDefaultComponent(productSlug, role, sourceType)])
  }

  const updateComponent = (index: number, patch: Partial<NewComponentPayload>) => {
    setComponents((current) => current.map((component, itemIndex) => itemIndex === index ? { ...component, ...patch } : component))
  }

  const runDetection = async (index: number) => {
    const component = components[index]
    setDetectingId(component.id)
    setError(null)
    try {
      const detection = await onDetect(component.path)
      setDetections((current) => ({ ...current, [component.id]: detection }))
      updateComponent(index, {
        runner: runnerForSource(component.source_type, detection.runnerSuggestion),
        type: detection.typeSuggestion,
        role: component.role || detection.roleSuggestion,
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No se pudo detectar el proyecto.')
    } finally {
      setDetectingId(null)
    }
  }

  const submit = async () => {
    setError(null)
    if (validationErrors.length) {
      setError(validationErrors.join(' '))
      return
    }
    try {
      await onSubmit(payload)
      onClose()
      setStep(0)
      setLabel('')
      setSlug('')
      setDescription('')
      setComponents([])
      setDetections({})
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No se pudo crear el producto.')
    }
  }

  return (
    <div className="wizard-backdrop" role="dialog" aria-modal="true" aria-label="Agregar proyecto">
      <aside className="wizard-drawer">
        <header className="wizard-header">
          <div>
            <h2>New Project Wizard</h2>
            <p>Configura un producto, sus componentes y accesos rápidos.</p>
          </div>
          <button className="icon-button" onClick={onClose} aria-label="Cerrar"><X /></button>
        </header>

        <div className="wizard-steps">
          {['Product details', 'Components', 'Detection', 'Review'].map((item, index) => (
            <button key={item} className={index === step ? 'wizard-step wizard-step--active' : index < step ? 'wizard-step wizard-step--done' : 'wizard-step'} onClick={() => setStep(index)}>
              <span>{index < step ? <Check size={14} /> : index + 1}</span>
              {item}
            </button>
          ))}
        </div>

        <div className="wizard-body">
          {step === 0 ? (
            <section className="wizard-section">
              <p className="section-label">Select project source</p>
              <div className="source-grid">
                {sourceOptions.map((option) => {
                  const Icon = icons[option.id]
                  return (
                    <button key={option.id} className={sourceType === option.id ? 'source-card source-card--selected' : 'source-card'} onClick={() => setSourceType(option.id)}>
                      <Icon size={22} />
                      <span><strong>{option.label}</strong><small>{option.description}</small></span>
                    </button>
                  )
                })}
              </div>
              <label className="field"><span>Nombre del producto</span><input value={label} onChange={(event) => { setLabel(event.target.value); if (!slug) setSlug(slugify(event.target.value)) }} placeholder="POS Warehouses" /></label>
              <label className="field"><span>Slug</span><input value={effectiveSlug} onChange={(event) => setSlug(event.target.value)} placeholder="pos_warehouses" /></label>
              <label className="field"><span>Descripción</span><textarea value={description} onChange={(event) => setDescription(event.target.value)} placeholder="Backend + frontend local para..." /></label>
              <div className="safety-note"><AlertTriangle size={18} /> Repositorios de terceros se registran primero y requieren revisión manual antes de ejecución.</div>
            </section>
          ) : null}

          {step === 1 ? (
            <section className="wizard-section">
              <div className="wizard-toolbar"><p className="section-label">Components</p><div><button className="button button--secondary button--xs" onClick={() => addComponent('backend')}>+ Backend</button><button className="button button--secondary button--xs" onClick={() => addComponent('frontend')}>+ Frontend</button><button className="button button--ghost button--xs" onClick={() => addComponent('service')}>+ Service</button></div></div>
              {components.length === 0 ? <p className="empty-hint">Agrega backend, frontend o servicio externo.</p> : null}
              {components.map((component, index) => (
                <div className="component-form" key={`${component.id}-${index}`}>
                  <label className="field"><span>ID</span><input value={component.id} onChange={(event) => updateComponent(index, { id: event.target.value, project_name: event.target.value })} /></label>
                  <label className="field"><span>Label</span><input value={component.label} onChange={(event) => updateComponent(index, { label: event.target.value })} /></label>
                  <label className="field"><span>Role</span><select value={component.role} onChange={(event) => updateComponent(index, { role: event.target.value })}><option>backend</option><option>frontend</option><option>worker</option><option>service</option><option>external</option></select></label>
                  <label className="field field--wide"><span>Path</span><input value={component.path} onChange={(event) => updateComponent(index, { path: event.target.value })} placeholder="~/Trabajo/..." /></label>
                  <label className="field field--wide"><span>URL principal</span><input value={component.urls.frontend || component.urls.docs || component.urls.api || ''} onChange={(event) => updateComponent(index, { urls: component.role === 'frontend' ? { frontend: event.target.value } : { docs: event.target.value.endsWith('/docs') ? event.target.value : event.target.value, api: event.target.value.replace(/\/docs$/, '') } })} placeholder="http://...localhost" /></label>
                  <button className="button button--ghost button--xs" onClick={() => setComponents((current) => current.filter((_, itemIndex) => itemIndex !== index))}>Eliminar</button>
                </div>
              ))}
            </section>
          ) : null}

          {step === 2 ? (
            <section className="wizard-section">
              <p className="section-label">Detection</p>
              {components.map((component, index) => {
                const detection = detections[component.id]
                return (
                  <article className="detect-card" key={component.id}>
                    <div><strong>{component.label}</strong><p className="path-text">{component.path || 'Sin ruta'}</p></div>
                    <button className="button button--secondary button--xs" onClick={() => void runDetection(index)} disabled={!component.path || detectingId === component.id}>{detectingId === component.id ? 'Detectando...' : 'Detectar'}</button>
                    {detection ? <div className="detect-result"><span>Runner: {detection.runnerSuggestion}</span><span>Tipo: {detection.typeSuggestion}</span>{detection.warnings.map((warning) => <em key={warning}>{warning}</em>)}</div> : null}
                  </article>
                )
              })}
            </section>
          ) : null}

          {step === 3 ? (
            <section className="wizard-section">
              <p className="section-label">Review</p>
              <pre className="review-box">{JSON.stringify(payload, null, 2)}</pre>
              {validationErrors.length ? <div className="error-box">{validationErrors.join(' ')}</div> : null}
            </section>
          ) : null}

          {error ? <div className="error-box">{error}</div> : null}
        </div>

        <footer className="wizard-footer">
          <button className="button button--ghost" onClick={onClose}>Cancel</button>
          <div>
            <button className="button button--secondary" disabled={step === 0} onClick={() => setStep((current) => Math.max(0, current - 1))}>Back</button>
            {step < 3 ? <button className="button button--primary" onClick={() => setStep((current) => Math.min(3, current + 1))}>Continue <ChevronRight size={16} /></button> : <button className="button button--primary" onClick={() => void submit()}>Save product</button>}
          </div>
        </footer>
      </aside>
    </div>
  )
}
