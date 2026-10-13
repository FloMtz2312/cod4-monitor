// "C'est moi": the profile this phone pins, per server, without an account (kept in the browser only).
// The code of !me in game does the same from a link: /me/<code> → #/moi/<code>.
import { get } from './api.js';

const STORE = 'v2.me';

function load() {
  try {
    const v = JSON.parse(localStorage.getItem(STORE) || '{}');
    return v && typeof v === 'object' ? v : {};
  } catch {
    return {};
  }
}

/** @type {Record<string, {key: string, name_raw: string}>} */
export const mine = $state(load());

function save() {
  try { localStorage.setItem(STORE, JSON.stringify(mine)); } catch {}
}

/** The pinned profile of a server, or null. */
export const meOn = serverId => (serverId == null ? null : mine[String(serverId)] ?? null);

export function setMe(serverId, key, name_raw) {
  mine[String(serverId)] = { key, name_raw: name_raw || '' };
  save();
}

export function clearMe(serverId) {
  delete mine[String(serverId)];
  save();
}

/** Resolves a !me code and pins its profile. @returns {Promise<{server_id: number, key: string, name_raw: string}>} */
export async function claim(code) {
  const r = await get('/api/claim/' + encodeURIComponent(code.replace(/\s+/g, '').toUpperCase()));
  setMe(r.server_id, r.key, r.name_raw);
  return r;
}
