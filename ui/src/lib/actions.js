// Actions on the game server (rcon). Every call is checked and logged by the monitor.
import { post } from './api.js';
import { say } from './toast.svelte.js';
import { t } from './i18n.svelte.js';
import { clean } from './format.js';

/** Removes what could end the rcon command (quotes, ;, line breaks, backslash). */
export const safeText = s => String(s ?? '').replace(/["';\\\r\n\x00]/g, '').trim();

async function run(fn, done) {
  try {
    await fn();
    say(done);
    return true;
  } catch (e) {
    say(t('Échec : {err}', { err: e.message }));
    return false;
  }
}

export const changeMap = (id, map, title) =>
  run(() => post(`/api/servers/${id}/rcon`, { command: `map ${map}` }), t('Changement de map vers {map}…', { map: title }));
export const restart = id => run(() => post(`/api/servers/${id}/rcon`, { command: 'fast_restart' }), t('Partie relancée'));
export const nextMap = id => run(() => post(`/api/servers/${id}/rcon`, { command: 'map_rotate' }), t('Passage à la map suivante…'));
export const sayAll = (id, msg) => run(() => post(`/api/servers/${id}/rcon`, { command: `say ${safeText(msg)}` }), t('Message envoyé à tous'));
export const tell = (id, name, msg) =>
  run(() => post(`/api/servers/${id}/tell`, { name, message: safeText(msg) }), t('Message envoyé à {name}', { name: clean(name) }));
export const kick = (id, name) => run(() => post(`/api/servers/${id}/kick`, { name }), t('{name} a été expulsé', { name: clean(name) }));
/** Any console command (needs the rcon permission, checked by the monitor). */
export const rcon = (id, command, done) => run(() => post(`/api/servers/${id}/rcon`, { command }), done);
export const mapRestart = id => rcon(id, 'map_restart', t('Redémarrage de la map…'));
/** Map change in another game mode: g_gametype first (read when the map loads), then the map. */
export const changeMapMode = (id, map, title, gametype) =>
  run(async () => {
    await post(`/api/servers/${id}/rcon`, { command: `g_gametype ${gametype}` });
    await post(`/api/servers/${id}/rcon`, { command: `map ${map}` });
  }, t('Changement de map vers {map}…', { map: title }));
export const ban = (id, name, duration, reason) =>
  run(() => post(`/api/servers/${id}/ban`, { name, duration, reason: safeText(reason) }),
    duration ? t('{name} banni ({d})', { name: clean(name), d: duration }) : t('{name} banni définitivement', { name: clean(name) }));
