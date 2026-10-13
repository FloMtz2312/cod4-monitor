// Push notifications of this browser: the service worker (/sw.js), the subscription to the browser's push service and
// the choices kept by the monitor (/api/push). iPhone: only in the app installed on the home screen (iOS 16.4+).
import { api, get } from './api.js';
import { i18n } from './i18n.svelte.js';

const SAVED = 'v2.push.endpoint'; // the last endpoint given to the monitor: a browser can change it on its own

export const ios = () => /iphone|ipad|ipod/i.test(navigator.userAgent) || (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1);
export const standalone = () => matchMedia('(display-mode: standalone)').matches || /** @type {any} */ (navigator).standalone === true;

/** "ok", "install" (iPhone: from the home screen app only) or "unsupported". */
export function support() {
  const has = 'serviceWorker' in navigator && 'PushManager' in window && 'Notification' in window;
  if (ios() && !standalone()) return 'install';
  return has ? 'ok' : 'unsupported';
}

export const permission = () => ('Notification' in window ? Notification.permission : 'default');

async function registration() {
  const reg = await navigator.serviceWorker.register('/sw.js', { scope: '/' });
  await navigator.serviceWorker.ready;
  return reg;
}

/** Subscription of this browser and the choices the monitor has for it ({sub: null} when not subscribed). */
export async function current() {
  const reg = await registration();
  const s = await reg.pushManager.getSubscription();
  const r = await get('/api/push' + (s ? '?endpoint=' + encodeURIComponent(s.endpoint) : ''));
  return { key: r.key, kinds: r.kinds ?? [], json: s ? s.toJSON() : null, sub: s ? r.sub ?? null : null };
}

const keyBytes = b64 => {
  const s = atob(b64.replace(/-/g, '+').replace(/_/g, '/') + '='.repeat((4 - (b64.length % 4)) % 4));
  return Uint8Array.from(s, c => c.charCodeAt(0));
};

/** Asks the permission (needs a tap), subscribes and saves the choices. */
export async function subscribe(types, muted) {
  if ((await Notification.requestPermission()) !== 'granted') throw new Error('denied');
  const reg = await registration();
  const { key } = await get('/api/push');
  if (!key) throw new Error('off');
  // a subscription the monitor doesn't know (expired, other key, other account): start from a fresh one
  const old = await reg.pushManager.getSubscription();
  if (old) await old.unsubscribe().catch(() => {});
  const s = await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: keyBytes(key) });
  return save(s.toJSON(), types, muted);
}

/** Saves the choices of this browser. */
export async function save(sub, types, muted) {
  const r = await api('PUT', '/api/push', { endpoint: sub.endpoint, keys: sub.keys, lang: i18n.lang, types, muted });
  try { localStorage.setItem(SAVED, sub.endpoint); } catch {}
  return r.sub;
}

export async function unsubscribe() {
  const reg = await registration();
  const s = await reg.pushManager.getSubscription();
  if (s) {
    await api('DELETE', '/api/push', { endpoint: s.endpoint }).catch(() => {});
    await s.unsubscribe();
  }
  try { localStorage.removeItem(SAVED); } catch {}
}

export const test = endpoint => api('POST', '/api/push/test', { endpoint });

/** At start: a browser that changed its endpoint (or lost it) keeps its choices; the language follows the app's. */
export async function sync() {
  let saved = null;
  try { saved = localStorage.getItem(SAVED); } catch {}
  if (!saved || support() !== 'ok' || permission() !== 'granted') return;
  const reg = await navigator.serviceWorker.getRegistration('/');
  if (!reg) return;
  const s = await reg.pushManager.getSubscription();
  if (s && s.endpoint === saved) {
    const r = await get('/api/push?endpoint=' + encodeURIComponent(saved));
    if (r.sub && r.sub.lang !== i18n.lang) await save(s.toJSON(), r.sub.types, r.sub.muted);
    return;
  }
  const r = await get('/api/push?endpoint=' + encodeURIComponent(saved));
  if (!r.sub || !r.key) return; // unsubscribed meanwhile, or from another account
  const fresh = s ?? await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: keyBytes(r.key) });
  await save(fresh.toJSON(), r.sub.types, r.sub.muted);
  await api('DELETE', '/api/push', { endpoint: saved }).catch(() => {});
}
