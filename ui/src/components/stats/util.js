// Display helpers of the stats and status screens: durations, dates, shares.
import { t, locale } from '../../lib/i18n.svelte.js';

/** Seconds → "45 s", "12 min", "3 h 05", "2 j 4 h". */
export function dur(s) {
  if (s == null || !isFinite(s)) return '—';
  s = Math.max(0, Math.round(s));
  if (s < 60) return `${s} s`;
  if (s < 3600) return `${Math.round(s / 60)} min`;
  if (s < 86400) return `${Math.floor(s / 3600)} h ${String(Math.floor((s % 3600) / 60)).padStart(2, '0')}`;
  return `${Math.floor(s / 86400)} ${t('j')} ${Math.floor((s % 86400) / 3600)} h`;
}

/** Unix seconds → "07/10/2026 21:04" (local time). */
export const dt = ts => (ts ? new Date(ts * 1000).toLocaleString(locale(), { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' }) : '—');

/** Unix seconds → "07/10 21:04". */
export const short = ts => (ts ? new Date(ts * 1000).toLocaleString(locale(), { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' }) : '—');

/** Unix seconds → "07/10". */
export const dayLabel = ts => new Date(ts * 1000).toLocaleDateString(locale(), { day: '2-digit', month: '2-digit' });

/** Share a / b as "42 %", or "—". */
export const pct = (a, b) => (b ? `${Math.round((100 * a) / b)} %` : '—');

/** 1.25 → "1,3" (one decimal, local). */
export const dec = (v, n = 1) => (v == null ? '—' : Number(v).toLocaleString(locale(), { maximumFractionDigits: n, minimumFractionDigits: n }));

/** Player key of a list row: GUID, else the name. */
export const playerKey = (guid, name) => guid || 'n:' + name;
