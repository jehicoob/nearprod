export class ApiError extends Error {
    code;
    details;
    constructor(code, message, details) {
        super(message);
        this.code = code;
        this.details = details;
    }
}
export async function api(route, body, signal) {
    const res = await fetch(`/api${route}`, { method: body === undefined ? 'GET' : 'POST', credentials: 'same-origin', headers: body === undefined ? undefined : { 'Content-Type': 'application/json' }, body: body === undefined ? undefined : JSON.stringify(body), signal });
    const value = await res.json();
    if (!res.ok)
        throw new ApiError(value.error?.code || 'HTTP_ERROR', value.error?.message || `HTTP ${res.status}`, value.error?.details);
    return value;
}
export const humanBytes = (n) => n === undefined || n === null ? 'Sin datos' : n >= 2 ** 30 ? `${(n / 2 ** 30).toFixed(2)} GiB` : `${(n / 2 ** 20).toFixed(0)} MiB`;
export const message = (e) => e instanceof Error ? e.message : 'Error inesperado.';
