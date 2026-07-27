export type ProjectStatusValue = 'running' | 'partial' | 'stopped' | 'not_created' | 'unknown' | 'manual' | 'external'
export type Runner = 'docker-compose' | 'dockerfile' | 'manual' | 'external'
export type ActionName = 'start' | 'stop' | 'down' | 'restart'

export interface ContainerInfo {
  readonly id: string
  readonly name: string
  readonly image: string
  readonly state: string
  readonly status: string
  readonly health?: string
  readonly ports?: string
}

export interface ProjectStatus {
  readonly id: string
  readonly label: string
  readonly role: string
  readonly runner: Runner | string
  readonly type: string
  readonly path: string
  readonly resolvedPath: string
  readonly project_name: string
  readonly urls: Readonly<Record<string, string>>
  readonly status: ProjectStatusValue
  readonly exists: boolean
  readonly running: number
  readonly total: number
  readonly containers: readonly ContainerInfo[]
  readonly error?: string
}

export interface ProductGroup {
  readonly id: string
  readonly label: string
  readonly description?: string
  readonly status: ProjectStatusValue
  readonly quick_links: Readonly<Record<string, string>>
  readonly components: readonly ProjectStatus[]
  readonly running: number
  readonly total: number
}

export interface DockerSummary {
  readonly context: string
  readonly clientVersion?: string
  readonly serverVersion?: string
  readonly dockerOk: boolean
  readonly error?: string | null
}

export interface DashboardResponse {
  readonly generatedAt: string
  readonly root: string
  readonly projectsFile: string
  readonly docker: DockerSummary
  readonly projects: readonly ProjectStatus[]
  readonly groups: readonly ProductGroup[]
}

export interface JobInfo {
  readonly id: string
  readonly targetId: string
  readonly targetType: 'project' | 'group'
  readonly action: ActionName
  readonly command: string
  readonly status: 'running' | 'succeeded' | 'failed'
  readonly exitCode: number | null
  readonly output: string
  readonly startedAt: string
  readonly finishedAt: string | null
}

export interface DetectResult {
  readonly inputPath: string
  readonly resolvedPath: string
  readonly exists: boolean
  readonly isDirectory: boolean
  readonly runnerSuggestion: Runner | string
  readonly typeSuggestion: string
  readonly roleSuggestion: string
  readonly files: Readonly<Record<string, readonly string[]>>
  readonly warnings: readonly string[]
}

export interface NewComponentPayload {
  readonly id: string
  readonly label: string
  readonly role: string
  readonly runner: Runner | string
  readonly source_type: string
  readonly path: string
  readonly project_name: string
  readonly type: string
  readonly urls: Readonly<Record<string, string>>
  readonly git_url?: string
  readonly trusted?: boolean
}

export interface NewProductPayload {
  readonly slug: string
  readonly label: string
  readonly description: string
  readonly quick_links: Readonly<Record<string, string>>
  readonly components: readonly NewComponentPayload[]
}
