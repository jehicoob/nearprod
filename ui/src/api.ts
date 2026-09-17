export class ApiError extends Error { constructor(public code: string, message: string, public details?: unknown) { super(message); } }
export async function api<T>(route: string, body?: unknown, signal?: AbortSignal): Promise<T> {
  const res = await fetch(`/api${route}`, { method: body === undefined ? 'GET' : 'POST', credentials: 'same-origin', headers: body === undefined ? undefined : {'Content-Type': 'application/json'}, body: body === undefined ? undefined : JSON.stringify(body), signal });
  const value = await res.json();
  if (!res.ok) throw new ApiError(value.error?.code || 'HTTP_ERROR', value.error?.message || `HTTP ${res.status}`, value.error?.details);
  return value as T;
}
export const humanBytes = (n?: number | null) => n === undefined || n === null ? 'Sin datos' : n >= 2 ** 30 ? `${(n / 2 ** 30).toFixed(2)} GiB` : `${(n / 2 ** 20).toFixed(0)} MiB`;
export const message = (e: unknown) => e instanceof Error ? e.message : 'Error inesperado.';
