import type { ActionName, DashboardResponse, DetectResult, JobInfo, NewProductPayload } from '../types'

async function requestJson<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await fetch(url, {
    headers: { 'content-type': 'application/json', ...(options?.headers ?? {}) },
    ...options,
  })
  const payload = await response.json().catch(() => ({}))
  if (!response.ok) {
    throw new Error(payload.error || `Request failed: ${response.status}`)
  }
  return payload as T
}

export function getDashboard(): Promise<DashboardResponse> {
  return requestJson<DashboardResponse>('/api/projects')
}

export function runProjectAction(projectId: string, action: ActionName): Promise<{ readonly job: JobInfo }> {
  return requestJson(`/api/projects/${projectId}/actions/${action}`, { method: 'POST' })
}

export function runGroupAction(groupId: string, action: ActionName): Promise<{ readonly job: JobInfo }> {
  return requestJson(`/api/groups/${groupId}/actions/${action}`, { method: 'POST' })
}

export function getJob(jobId: string): Promise<{ readonly job: JobInfo }> {
  return requestJson(`/api/jobs/${jobId}`)
}

export function getProjectLogs(projectId: string, tail = 180): Promise<{ readonly code: number; readonly output: string; readonly stderr: string }> {
  return requestJson(`/api/projects/${projectId}/logs?tail=${tail}`)
}

export function detectProject(path: string): Promise<DetectResult> {
  return requestJson('/api/detect', { method: 'POST', body: JSON.stringify({ path }) })
}

export function createProduct(payload: NewProductPayload): Promise<{ readonly group: unknown; readonly backup: string }> {
  return requestJson('/api/products', { method: 'POST', body: JSON.stringify(payload) })
}
