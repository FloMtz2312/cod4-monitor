// Server-side demos: sizes, durations and the link that opens one in the cod4-demo viewer.
import { t } from './i18n.svelte.js';

/** Bytes → "820 Mo", "2,4 Go". */
export function size(b) {
  if (!b) return '0';
  const en = document.documentElement.lang === 'en';
  const fmt = (n, d) => n.toLocaleString(en ? 'en-US' : 'fr-FR', { maximumFractionDigits: d });
  if (b < 1e9) return `${fmt(b / 1e6, 0)} ${en ? 'MB' : 'Mo'}`;
  return `${fmt(b / 1e9, 1)} ${en ? 'GB' : 'Go'}`;
}

/** Gigabytes → "2,2 Go". */
export const gb = n => size((n ?? 0) * 1e9);

/** Duration of a demo, "1 h 12" or "14 min". */
export function length(d) {
  const s = Math.max(0, (d.last_write ?? 0) - (d.started_at ?? 0));
  if (s < 3600) return t('{n} min', { n: Math.max(1, Math.round(s / 60)) });
  return `${Math.floor(s / 3600)} h ${String(Math.round((s % 3600) / 60)).padStart(2, '0')}`;
}

/** A demo is still being written (recorded right now). */
export const live = d => Date.now() / 1000 - (d.last_write ?? 0) < 120;

/** Address of the viewer opening this demo (the file is read from the game server by the monitor). */
export const viewerURL = (serverId, d) =>
  `/demo-viewer/index.html?demo=${encodeURIComponent(`/api/servers/${serverId}/demos/${d.id}/file/${d.file}`)}`;

/** Address to download the demo. */
export const fileURL = (serverId, d) => `/api/servers/${serverId}/demos/${d.id}/file/${encodeURIComponent(d.file)}`;

let installed = null;
/** Whether this monitor serves the demo viewer (asked once). */
export function viewerInstalled() {
  installed ??= fetch('/api/demo-viewer', { credentials: 'same-origin' }).then(r => r.json()).then(r => !!r.installed).catch(() => false);
  return installed;
}
