// What the signed-in account may do on a server (/api/me "access"). The owner and the super admin may do everything.
const ALL = ['private', 'kick', 'ban', 'reports', 'notes', 'maps', 'say', 'rcon'];

/** @returns {Set<string>} */
export function permsOf(me, serverId) {
  if (!me || me.role === 'visitor' || serverId == null) return new Set();
  if (me.noauth || me.admin) return new Set(ALL);
  const a = me.access?.[String(serverId)];
  if (!a) return new Set();
  if (a.owner) return new Set(ALL);
  // {kick: true, …} from /api/me (a list is accepted too)
  const ps = a.permissions ?? {};
  return new Set(Array.isArray(ps) ? ps : Object.keys(ps).filter(k => ps[k]));
}

export const canAct = p => p.has('kick') || p.has('ban') || p.has('say');

/** Owner of the server (or super admin): B3 penalties, everything else. */
export function isOwner(me, serverId) {
  if (!me || me.role === 'visitor' || serverId == null) return false;
  return !!(me.noauth || me.admin || me.access?.[String(serverId)]?.owner);
}
