// This file is copied over the goctl-generated transport after every contract
// generation. goctl currently makes the request argument mandatory even for
// parameterless GET endpoints, while generated endpoint functions omit it.
export type Method = 'get' | 'GET' | 'delete' | 'DELETE' | 'head' | 'HEAD' | 'options' | 'OPTIONS' | 'post' | 'POST' | 'put' | 'PUT' | 'patch' | 'PATCH';

const reg = /:[a-z|A-Z]+/g;

export function parseParams(url: string): string[] {
    return (url.match(reg) ?? []).map((key) => key.replace(/:/, ''));
}

export function genUrl(url: string, params?: Record<string, unknown>): string {
    if (!params) return url;
    const pathParams = parseParams(url);
    for (const key of pathParams) url = url.replace(new RegExp(`:${key}`), String(params[key]));
    const query = Object.entries(params)
        .filter(([key]) => !pathParams.includes(key))
        .map(([key, value]) => `${encodeURIComponent(key)}=${encodeURIComponent(String(value))}`);
    return query.length ? `${url}?${query.join('&')}` : url;
}

async function request<T>({ method, url, data }: { method: Method; url: string; data?: unknown }): Promise<T> {
    const token = typeof window === 'undefined' ? '' : localStorage.getItem('im_token') ?? '';
    const response = await fetch(url, {
        method: method.toUpperCase(),
        credentials: 'include',
        headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
        body: data && !/get|delete/i.test(method) ? JSON.stringify(data) : undefined
    });
    if (!response.ok) throw new Error(`API request failed: ${response.status}`);
    return response.json() as Promise<T>;
}

function api<T>(method: Method, url: string, req?: unknown, body?: unknown): Promise<T> {
    const candidate = req as { params?: Record<string, unknown>; forms?: Record<string, unknown> } | undefined;
    if (url.includes(':')) url = genUrl(url, candidate?.params ?? candidate?.forms);
    return request<T>({ method, url, data: body ?? req });
}

export const webapi = {
    get: <T>(url: string, req?: unknown, body?: unknown) => api<T>('get', url, req, body),
    delete: <T>(url: string, req?: unknown, body?: unknown) => api<T>('delete', url, req, body),
    put: <T>(url: string, req?: unknown, body?: unknown) => api<T>('put', url, req, body),
    post: <T>(url: string, req?: unknown, body?: unknown) => api<T>('post', url, req, body),
    patch: <T>(url: string, req?: unknown, body?: unknown) => api<T>('patch', url, req, body)
};

export default webapi;
