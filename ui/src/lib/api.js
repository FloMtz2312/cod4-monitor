// Calls to the monitor's API, on the same origin (session cookie). A 401 only means "not signed in":
// the caller shows the public data instead.

export class ApiError extends Error {
  constructor(status, message) {
    super(message);
    this.status = status;
  }
}

export async function api(method, path, body) {
  const init = { method, headers: { Accept: 'application/json' } };
  if (body !== undefined) {
    init.headers['Content-Type'] = 'application/json';
    init.body = JSON.stringify(body);
  }
  const r = await fetch(path, init);
  const data = await r.json().catch(() => ({}));
  if (!r.ok) throw new ApiError(r.status, data.error || r.statusText);
  return data;
}

export const get = path => api('GET', path);
export const post = (path, body) => api('POST', path, body ?? {});
