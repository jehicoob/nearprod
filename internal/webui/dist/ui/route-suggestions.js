/** A DNS label is at most 63 ASCII characters. Keep suggestions valid for long group names. */
export function dnsLabel(value) {
    return value.normalize('NFKD').replace(/[\u0300-\u036f]/g, '').toLowerCase()
        .replace(/[^a-z0-9-]+/g, '-').replace(/-+/g, '-').replace(/^-|-$/g, '')
        .slice(0, 63).replace(/-+$/g, '') || 'proyecto';
}
export function suggestedHost(draft, service, index) {
    const group = dnsLabel(draft.product);
    const role = /api|backend/i.test(service) ? 'api' : /frontend|client|website|^web$/i.test(service) ? 'web'
        : /api|backend/i.test(draft.slug) ? 'api' : 'web';
    const stem = role === 'api' ? `api-${group}` : index ? `${dnsLabel(service)}-${group}` : group;
    return `${dnsLabel(stem)}.localhost`;
}
