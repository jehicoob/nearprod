export interface Mode { files: string[]; envFiles: string[]; profiles: string[] }
export interface Group { id: string; name: string }
export interface LocalLink { label: string; url: string }
export interface WebRoute { host: string; service: string; port: number; verify?: {service: string; port: number} }
export interface ProxyInfo { enabled: boolean; port: number; image: string; state: string; health: string; project: string; network: string; checkedAt: string | null }
export interface RouteObserved { host: string; url: string; service: string; port: number; state: string; mode: string | null }
export interface ProxyPreview { port: number; image: string; project: string; network: string; impacted: string[]; blockers: string[]; running: boolean; fingerprint: string; note: string }
export interface RouteCheck { host: string; url: string; state: string; checkedAt: string; dns: {state: string; addresses: string[]}; response: {status?: number; error?: string; marker?: string}; note: string; hostsEntry: string }
export interface Stack { routes?: WebRoute[]; links?: LocalLink[]; id: string; uid: string; product: string; slug: string; name: string; path: string; projectName: string; modes: { dev: Mode; verify?: Mode }; activeMode: 'dev' | 'verify' | null; trust: Record<string, {fingerprint: string; allowUnsafe: boolean}>; binding?: {engineId: string; endpoint: string} }
export interface Operation { id: string; action: string; targets: string[]; state: string; startedAt: string; endedAt?: string; lines: {time: string; text: string; stream: string}[]; error?: {code: string; message: string}; results?: {id: string; state: string; error?: {code: string; message: string}}[] }
export type RuntimeKind = 'colima' | 'native';
export interface Runtime { kind: RuntimeKind; context: string; profile: string }
export interface HostCapabilities {
  os: string;
  architecture: string;
  environment: string;
  displayName: string;
  runtime: {
    kind: RuntimeKind;
    displayName: string;
    supportedKinds: RuntimeKind[];
    supported: boolean;
    managedVirtualMachine: boolean;
    canStart: boolean;
    canConfigureResources: boolean;
  };
  packageManagement: { provider: string; displayName: string; managed: boolean; canInstall: boolean; canCheckUpdates: boolean };
  startup: { supported: boolean; manager: string };
  proxy: { supported: boolean; location: 'host' | 'virtual-machine' };
}
export interface Catalog { proxy?: { enabled: boolean; port: number; image: string }; version: string; host: HostCapabilities; roots: string[]; runtime: Runtime; groups: Group[]; stacks: Stack[]; operations: Operation[] }
export interface Container { id: string; name: string; service: string; state: string; running: boolean; health: string; exitCode: number | null; oom: boolean; image: string; imageId: string; platform: string | null; owned: boolean; ports: {host: string; port: number; url: string | null; container: string}[] }
export interface Observed { routes?: RouteObserved[]; id: string; execution: string; health: string; containers: Container[]; watch: {state: string; lines: string[]; error?: {message: string}} }
export interface Status { proxy?: ProxyInfo; connected: boolean; checkedAt: string | null; info?: {version: string; memoryBytes: number; cpus: number; architecture: string}; error?: {code: string; message: string}; stacks: Observed[] }
export interface Candidate { path: string; relative: string; product: string; slug: string; projectName: string; bases: string[]; files: string[]; suggested: string[]; ambiguous: boolean }
export interface Preview { routes?: {host: string; service: string; port: number}[]; stack: string; mode: string; fingerprint: string; approved: boolean; files: string[]; envFiles: string[]; profiles: string[]; command: string[]; risks: string[]; warnings: string[]; blockers: string[]; modeChange: boolean; note: string; services: {name: string; image: string | null; platform: string | null; build: boolean; hasHealthcheck: boolean; watch: boolean; mounts: {type: string; source?: string; target: string}[]}[] }
export interface Doctor { nearprod: string; runtimeLanguage: string; goVersion: string; binary: string; platform: string; tools: {id: string; name: string; supported: boolean; available: boolean; version: string | null; hint: string; required: boolean; purpose: string; status: string; standaloneVersion?: string}[]; colima: {state: 'running' | 'stopped' | 'unknown' | 'missing' | 'not-applicable' | 'unsupported'; profile: string | null; message: string}; allocation: {memoryGiB: number; cpus: number; vmType: string; architecture: string; mountType: string} | null; engine: {version: string; architecture: string; id: string; endpoint: string; memoryBytes?: number} | null; error?: {message: string}; hostMemoryBytes: number; agentRssBytes: number }
export interface ResourcePreview { fingerprint: string; memory: number; cpus: number; running: boolean; affected: {id: string; name: string; project: string}[]; warning: string }
export interface LogLine { time: string; text: string; service: string; stream: string; container: string }
export interface Metrics { host: {totalBytes: number; freeBytes: number; note: string}; agent: {rssBytes: number}; guest: {totalBytes: number; availableBytes: number; swapTotalBytes: number; swapFreeBytes: number} | null; allocation: {memoryGiB: number; cpus: number} | null; engine: {memoryBytes?: number} | null; containers: {id: string; name: string; cpu: string; memory: string; memoryPercent: string}[]; note: string }

export interface ProjectOptions {
  path: string;
  composeFiles: {name: string; path: string; base: boolean}[];
  envFiles: {name: string; path: string; template: boolean; automatic: boolean}[];
  profiles: {name: string; services: string[]}[];
  httpHints?: {service: string; ports: number[]; suggestedPort: number | null; note: string}[];
  services: string[]; defaultServices: string[]; serviceEnvFiles: string[];
  suggestedFiles: string[]; projectName: string; projectNameSource: string;
  existingProjects: string[]; warnings: string[]; note: string;
}
export interface StackDraft {
  product: string; groupName?: string; slug: string; name: string; path: string;
  projectName: string; modes: {dev: Mode; verify?: Mode}; links: LocalLink[]; routes?: WebRoute[];
}
