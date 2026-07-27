import { ChevronDown, ChevronUp, Clipboard, RefreshCw, Search, X, Zap } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'

export interface LogsModalProps {
  readonly open: boolean
  readonly title: string
  readonly projectId: string
  readonly output: string
  readonly isLoading: boolean
  readonly isRefreshing: boolean
  readonly error: string | null
  readonly lastUpdated: string | null
  readonly onClose: () => void
  readonly onRefresh: () => void
  readonly onLiveRefresh: () => void
}

interface RenderedLinePart {
  readonly text: string
  readonly matchIndex: number | null
}

interface RenderedLine {
  readonly line: string
  readonly originalLineNumber: number
  readonly parts: readonly RenderedLinePart[]
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function buildRenderedLines(lines: readonly string[], query: string, onlyMatches: boolean): { readonly lines: readonly RenderedLine[]; readonly matchCount: number } {
  const q = query.trim()
  const lowerQ = q.toLowerCase()
  let matchCount = 0

  if (!q) {
    return {
      lines: lines.map((line, index) => ({ line, originalLineNumber: index + 1, parts: [{ text: line || ' ', matchIndex: null }] })),
      matchCount,
    }
  }

  const regex = new RegExp(`(${escapeRegExp(q)})`, 'gi')
  const renderedLines = lines.flatMap((line, index) => {
    if (onlyMatches && !line.toLowerCase().includes(lowerQ)) return []
    const parts = line.split(regex).map((part): RenderedLinePart => {
      if (part.toLowerCase() !== lowerQ) return { text: part, matchIndex: null }
      const currentMatchIndex = matchCount
      matchCount += 1
      return { text: part, matchIndex: currentMatchIndex }
    })
    return [{ line, originalLineNumber: index + 1, parts: parts.length ? parts : [{ text: ' ', matchIndex: null }] }]
  })

  return { lines: renderedLines, matchCount }
}

function formatUpdatedAt(value: string | null): string {
  if (!value) return 'Sin actualizar'
  return new Intl.DateTimeFormat('es-CO', { hour: '2-digit', minute: '2-digit', second: '2-digit' }).format(new Date(value))
}

export function LogsModal({ open, title, projectId, output, isLoading, isRefreshing, error, lastUpdated, onClose, onRefresh, onLiveRefresh }: LogsModalProps) {
  const [query, setQuery] = useState('')
  const [onlyMatches, setOnlyMatches] = useState(false)
  const [activeMatchIndex, setActiveMatchIndex] = useState(0)
  const [live, setLive] = useState(false)
  const outputRef = useRef<HTMLPreElement | null>(null)
  const endRef = useRef<HTMLSpanElement | null>(null)
  const safeOutput = output || 'Sin logs.'
  const lines = useMemo(() => safeOutput.split('\n'), [safeOutput])
  const rendered = useMemo(() => buildRenderedLines(lines, query, onlyMatches), [lines, onlyMatches, query])
  const hasMatches = rendered.matchCount > 0
  const searching = Boolean(query.trim())

  useEffect(() => {
    if (!open) setLive(false)
  }, [open])

  useEffect(() => {
    if (!open || !live) return undefined
    const id = window.setInterval(() => {
      if (!isLoading && !isRefreshing) onLiveRefresh()
    }, 2000)
    return () => window.clearInterval(id)
  }, [isLoading, isRefreshing, live, onLiveRefresh, open])

  useEffect(() => {
    setActiveMatchIndex(0)
  }, [query])

  useEffect(() => {
    if (!hasMatches) return
    if (activeMatchIndex > rendered.matchCount - 1) setActiveMatchIndex(0)
  }, [activeMatchIndex, hasMatches, rendered.matchCount])

  useEffect(() => {
    if (!hasMatches) return
    const element = outputRef.current?.querySelector(`[data-match-index="${activeMatchIndex}"]`)
    element?.scrollIntoView?.({ block: 'center', inline: 'nearest' })
  }, [activeMatchIndex, hasMatches, onlyMatches])

  useEffect(() => {
    if (!open || !live || searching) return
    endRef.current?.scrollIntoView?.({ block: 'end', inline: 'nearest' })
  }, [live, open, safeOutput, searching])

  const goToPreviousMatch = () => {
    if (!hasMatches) return
    setActiveMatchIndex((current) => (current - 1 + rendered.matchCount) % rendered.matchCount)
  }

  const goToNextMatch = () => {
    if (!hasMatches) return
    setActiveMatchIndex((current) => (current + 1) % rendered.matchCount)
  }

  if (!open) return null

  return (
    <div className="modal-backdrop" role="presentation">
      <section className="logs-modal" role="dialog" aria-modal="true" aria-label={`Logs de ${title}`}>
        <header className="logs-modal__header">
          <div>
            <p className="eyebrow">Logs</p>
            <h2>{title}</h2>
            <div className="logs-modal__meta">
              <span className="code-chip">{projectId}</span>
              <span className="logs-modal__updated">Última lectura: {formatUpdatedAt(lastUpdated)}</span>
              {isRefreshing ? <span className="logs-modal__refreshing">Actualizando...</span> : null}
            </div>
          </div>
          <div className="logs-modal__header-actions">
            <label className={live ? 'live-toggle live-toggle--active' : 'live-toggle'}>
              <input type="checkbox" checked={live} onChange={(event) => setLive(event.target.checked)} />
              <Zap size={14} /> Live
            </label>
            <button className="button button--secondary button--sm" type="button" onClick={onRefresh} disabled={isLoading}>
              <RefreshCw size={14} /> Recargar
            </button>
            <button className="icon-button" type="button" onClick={onClose} aria-label="Cerrar logs"><X size={18} /></button>
          </div>
        </header>

        <div className="logs-modal__toolbar">
          <label className="search-box logs-modal__search">
            <Search size={16} />
            <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Buscar dentro de los logs..." autoFocus />
          </label>
          <div className="logs-modal__nav" aria-label="Navegación de coincidencias">
            <button className="icon-button" type="button" onClick={goToPreviousMatch} disabled={!hasMatches} aria-label="Coincidencia anterior"><ChevronUp size={16} /></button>
            <button className="icon-button" type="button" onClick={goToNextMatch} disabled={!hasMatches} aria-label="Siguiente coincidencia"><ChevronDown size={16} /></button>
          </div>
          <label className="check-pill">
            <input type="checkbox" checked={onlyMatches} onChange={(event) => setOnlyMatches(event.target.checked)} disabled={!query.trim()} />
            Solo coincidencias
          </label>
          <button className="button button--ghost button--sm" type="button" onClick={() => void navigator.clipboard?.writeText(safeOutput)}>
            <Clipboard size={14} /> Copiar
          </button>
          <span className="logs-modal__counter">
            {query.trim() ? `${hasMatches ? activeMatchIndex + 1 : 0}/${rendered.matchCount} coincidencia${rendered.matchCount === 1 ? '' : 's'}` : `${lines.length} líneas`}
          </span>
        </div>

        {error ? <div className="error-box">{error}</div> : null}
        <div className="logs-modal__body" aria-busy={isLoading}>
          {isLoading ? <div className="logs-modal__loading">Cargando logs...</div> : null}
          {!isLoading && rendered.lines.length === 0 ? <div className="empty-hint">No hay líneas que coincidan con la búsqueda.</div> : null}
          {!isLoading && rendered.lines.length > 0 ? (
            <pre className="logs-modal__output" ref={outputRef}>
              {rendered.lines.map((line) => (
                <div className="log-line" key={`${line.originalLineNumber}-${line.line.slice(0, 20)}`}>
                  <span className="log-line__number">{line.originalLineNumber}</span>
                  <code>
                    {line.parts.map((part, index) => part.matchIndex === null
                      ? <span key={`${part.text}-${index}`}>{part.text}</span>
                      : (
                        <mark
                          key={`${part.text}-${part.matchIndex}`}
                          className={part.matchIndex === activeMatchIndex ? 'log-match log-match--active' : 'log-match'}
                          data-match-index={part.matchIndex}
                        >
                          {part.text}
                        </mark>
                      ))}
                  </code>
                </div>
              ))}
              <span ref={endRef} />
            </pre>
          ) : null}
        </div>
      </section>
    </div>
  )
}
