// The four account levels of the monitor, reduced to what the interface shows.
//   visitor: no account, public statistics only
//   moderator: invited on a server, private data and actions according to its permissions
//   admin: server owner (or the super admin of the .env), everything on its servers

/** @param {any} me the answer of /api/me (null when it failed) */
export function roleOf(me) {
  if (!me || me.role === 'visitor') return 'visitor';
  if (me.noauth || me.admin || me.role === 'superadmin' || me.role === 'admin') return 'admin';
  return 'moderator';
}

export const roleLabel = { visitor: 'Visiteur', moderator: 'Modérateur', admin: 'Admin' };

/** Tabs of the bottom bar, in order; `chat`: the account is in the admins' chat of the server (its tab after Modération). */
export function tabsFor(role, chat = false) {
  if (role === 'visitor') return ['direct', 'classements', 'joueurs', 'matchs', 'plus'];
  return chat ? ['direct', 'moderation', 'chat-admins', 'joueurs', 'classements', 'plus'] : ['direct', 'moderation', 'joueurs', 'classements', 'plus'];
}

export const tabInfo = {
  direct: { label: 'Direct', icon: 'live' },
  classements: { label: 'Classements', icon: 'trophy' },
  joueurs: { label: 'Joueurs', icon: 'users' },
  matchs: { label: 'Matchs', icon: 'matches' },
  moderation: { label: 'Modération', icon: 'shield' },
  'chat-admins': { label: 'Chat', icon: 'message' },
  plus: { label: 'Plus', icon: 'more' }
};
