// Shared state: the signed-in account and the monitored servers.
import { get } from './api.js';
import { roleOf } from './roles.js';

export const app = $state({
  me: null,
  servers: [],
  serverId: null,
  loaded: false,
  error: ''
});

export function role() {
  return roleOf(app.me);
}

export function currentServer() {
  return app.servers.find(s => s.id === app.serverId) ?? null;
}

export function selectServer(id) {
  app.serverId = id;
  try { localStorage.setItem('v2.server', String(id)); } catch {}
}

export async function loadMe() {
  try {
    app.me = await get('/api/me');
  } catch {
    app.me = { role: 'visitor' };
  }
}

export async function loadServers() {
  try {
    const r = await get('/api/servers');
    app.servers = (r.servers ?? []).filter(s => s.enabled);
    app.error = '';
    if (!app.servers.some(s => s.id === app.serverId)) {
      let saved = null;
      try { saved = Number(localStorage.getItem('v2.server')); } catch {}
      const pick = app.servers.find(s => s.id === saved) ?? app.servers.find(s => s.online) ?? app.servers[0];
      app.serverId = pick ? pick.id : null;
    }
  } catch (e) {
    app.error = e.message || 'error';
  } finally {
    app.loaded = true;
  }
}

/** "mp_backlot" → "Backlot", "mp_bo2paintball" → "Bo2paintball". */
export function mapName(map) {
  if (!map) return '—';
  const m = String(map).replace(/^mp_/, '').replace(/_/g, ' ');
  return m.charAt(0).toUpperCase() + m.slice(1);
}

/** Initials of a server name for its logo: "Sniper Only Promod" → "SO". */
export function initials(name) {
  const w = String(name || '?').replace(/\^\d/g, '').split(/[^A-Za-z0-9]+/).filter(Boolean);
  return ((w[0]?.[0] ?? '?') + (w[1]?.[0] ?? '')).toUpperCase();
}
