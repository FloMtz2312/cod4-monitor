// Hash routing: #/direct, #/joueurs, #/joueur/<key>… so that every screen has a link to share.

// Addresses of the previous interface: its sign-in and invitation pages (the token is after the #), and its tabs.
if (location.pathname === '/login.html' || location.pathname === '/invite.html') {
  const to = location.pathname === '/invite.html' ? '#/invitation/' + location.hash.slice(1) : '#/connexion';
  history.replaceState(null, '', '/' + to);
}
const OLD = { 'tchat-admins': 'chat-admins', accueil: 'direct', activite: 'stats', maps: 'stats', combat: 'classements', admin: 'moderation', serveur: 'etat' };

function parse() {
  const parts = location.hash.replace(/^#\/?/, '').split('/').filter(Boolean).map(decodeURIComponent);
  const name = parts[0] || 'direct';
  if (name === 'maps' && parts.length === 1) return { name: 'stats', params: ['maps'] };
  return { name: OLD[name] ?? name, params: parts.slice(1) };
}

export const route = $state(parse());
let moved = false; // a screen was opened inside the app: "back" can go back in the history

addEventListener('hashchange', () => {
  const r = parse();
  route.name = r.name;
  route.params = r.params;
  moved = true;
  scrollTo(0, 0);
});

/** Previous screen of the app, or `name` when the page was opened on this screen (shared link). */
export function back(name) {
  if (moved && history.length > 1) history.back();
  else go(name);
}

/** Replaces the current screen (no history entry): a redirect. */
export function replace(name, ...params) {
  history.replaceState(null, '', href(name, ...params));
  dispatchEvent(new HashChangeEvent('hashchange'));
}

/** Opens a screen: go('joueur', key). */
export function go(name, ...params) {
  location.hash = '#/' + [name, ...params.map(p => encodeURIComponent(p))].join('/');
}

export function href(name, ...params) {
  return '#/' + [name, ...params.map(p => encodeURIComponent(p))].join('/');
}
