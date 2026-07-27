import { useCallback, useEffect, useMemo, useState } from 'react'
import { createProduct, detectProject, getDashboard, getJob, getProjectLogs, runGroupAction, runProjectAction } from '../api/client'
import type { ActionName, DashboardResponse, DetectResult, JobInfo, NewProductPayload } from '../types'

export interface LogViewerState {
  readonly open: boolean
  readonly projectId: string
  readonly title: string
  readonly output: string
  readonly isLoading: boolean
  readonly isRefreshing: boolean
  readonly error: string | null
  readonly lastUpdated: string | null
}

export interface UseDashboardResult {
  readonly data: DashboardResponse | null
  readonly isLoading: boolean
  readonly error: string | null
  readonly filter: string
  readonly setFilter: (value: string) => void
  readonly activeJob: JobInfo | null
  readonly outputTitle: string
  readonly output: string
  readonly busyTargets: ReadonlySet<string>
  readonly refresh: () => Promise<void>
  readonly runAction: (targetType: 'group' | 'project', targetId: string, action: ActionName) => Promise<void>
  readonly showLogs: (projectId: string) => Promise<void>
  readonly closeLogs: () => void
  readonly refreshLogs: (silent?: boolean) => Promise<void>
  readonly logViewer: LogViewerState
  readonly clearOutput: () => void
  readonly detect: (path: string) => Promise<DetectResult>
  readonly addProduct: (payload: NewProductPayload) => Promise<void>
}

export function useDashboard(): UseDashboardResult {
  const [data, setData] = useState<DashboardResponse | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [filter, setFilter] = useState('')
  const [activeJob, setActiveJob] = useState<JobInfo | null>(null)
  const [outputTitle, setOutputTitle] = useState('Sin comandos activos')
  const [output, setOutput] = useState('Selecciona una acción para ver aquí el progreso.')
  const [busyTargets, setBusyTargets] = useState<Set<string>>(new Set())
  const [logViewer, setLogViewer] = useState<LogViewerState>({ open: false, projectId: '', title: '', output: '', isLoading: false, isRefreshing: false, error: null, lastUpdated: null })

  const refresh = useCallback(async () => {
    try {
      const response = await getDashboard()
      setData(response)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'No se pudo cargar el dashboard.')
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    void refresh()
    const id = window.setInterval(() => void refresh(), 7000)
    return () => window.clearInterval(id)
  }, [refresh])

  const pollJob = useCallback(async (jobId: string, busyKey: string) => {
    const payload = await getJob(jobId)
    setActiveJob(payload.job)
    setOutputTitle(`${payload.job.action} · ${payload.job.targetId} · ${payload.job.status}`)
    setOutput(payload.job.output || 'Esperando salida...')
    if (payload.job.status === 'running') {
      window.setTimeout(() => void pollJob(jobId, busyKey), 1200)
      return
    }
    setBusyTargets((current) => {
      const copy = new Set(current)
      copy.delete(busyKey)
      return copy
    })
    await refresh()
  }, [refresh])

  const runAction = useCallback(async (targetType: 'group' | 'project', targetId: string, action: ActionName) => {
    const busyKey = `${targetType}:${targetId}`
    setBusyTargets((current) => new Set(current).add(busyKey))
    setOutputTitle(`${action} · ${targetId}`)
    setOutput('Ejecutando comando...')
    try {
      const payload = targetType === 'group' ? await runGroupAction(targetId, action) : await runProjectAction(targetId, action)
      await pollJob(payload.job.id, busyKey)
    } catch (err) {
      setBusyTargets((current) => {
        const copy = new Set(current)
        copy.delete(busyKey)
        return copy
      })
      setOutputTitle(`Error · ${targetId}`)
      setOutput(err instanceof Error ? err.message : 'No se pudo ejecutar la acción.')
    }
  }, [pollJob])

  const loadLogs = useCallback(async (projectId: string, title: string, silent = false) => {
    setLogViewer((current) => ({
      ...current,
      open: true,
      projectId,
      title,
      output: silent ? current.output : current.projectId === projectId ? current.output : '',
      isLoading: !silent,
      isRefreshing: silent,
      error: null,
    }))
    try {
      const payload = await getProjectLogs(projectId, 500)
      setLogViewer((current) => ({
        ...current,
        open: true,
        projectId,
        title,
        output: payload.output || payload.stderr || 'Sin logs.',
        isLoading: false,
        isRefreshing: false,
        error: null,
        lastUpdated: new Date().toISOString(),
      }))
    } catch (err) {
      setLogViewer((current) => ({
        ...current,
        open: true,
        projectId,
        title,
        isLoading: false,
        isRefreshing: false,
        error: err instanceof Error ? err.message : 'No se pudieron cargar logs.',
      }))
    }
  }, [])

  const showLogs = useCallback(async (projectId: string) => {
    const project = data?.projects.find((item) => item.id === projectId)
    const title = project?.label ? `${project.label} · ${project.role}` : projectId
    await loadLogs(projectId, title, false)
  }, [data?.projects, loadLogs])

  const refreshLogs = useCallback(async (silent = false) => {
    if (!logViewer.projectId) return
    await loadLogs(logViewer.projectId, logViewer.title || logViewer.projectId, silent)
  }, [loadLogs, logViewer.projectId, logViewer.title])

  const closeLogs = useCallback(() => {
    setLogViewer((current) => ({ ...current, open: false, isLoading: false, isRefreshing: false }))
  }, [])

  const clearOutput = useCallback(() => {
    setActiveJob(null)
    setOutputTitle('Sin comandos activos')
    setOutput('Selecciona una acción para ver aquí el progreso.')
  }, [])

  const detect = useCallback((path: string) => detectProject(path), [])

  const addProduct = useCallback(async (payload: NewProductPayload) => {
    await createProduct(payload)
    setOutputTitle(`Producto creado · ${payload.slug}`)
    setOutput(`Se agregó ${payload.label} a projects.yml. Se creó respaldo automático antes de escribir.`)
    await refresh()
  }, [refresh])

  return useMemo(() => ({
    data,
    isLoading,
    error,
    filter,
    setFilter,
    activeJob,
    outputTitle,
    output,
    busyTargets,
    refresh,
    runAction,
    showLogs,
    closeLogs,
    refreshLogs,
    logViewer,
    clearOutput,
    detect,
    addProduct,
  }), [data, isLoading, error, filter, activeJob, outputTitle, output, busyTargets, logViewer, refresh, runAction, showLogs, closeLogs, refreshLogs, clearOutput, detect, addProduct])
}
