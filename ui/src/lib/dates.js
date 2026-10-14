// Dates and durations of the profile and live screens.
import { i18n, locale } from './i18n.svelte.js';

/** Unix seconds → "12/10/2026 21:04" (local time). */
export function dt(ts) {
  if (!ts) return '—';
  return new Date(ts * 1000).toLocaleString(locale(), { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' });
}

/** Unix seconds → "12/10/2026". */
export function day(ts) {
  if (!ts) return '—';
  return new Date(ts * 1000).toLocaleDateString(locale(), { day: '2-digit', month: '2-digit', year: 'numeric' });
}

/** Seconds → "45 s", "12 min", "3 h 05", "4 j 2 h". */
export function dur(s) {
  s = Math.max(0, Math.round(s ?? 0));
  if (s < 60) return `${s} s`;
  if (s < 3600) return `${Math.floor(s / 60)} min`;
  const h = Math.floor(s / 3600);
  if (h < 48) return `${h} h ${String(Math.floor((s % 3600) / 60)).padStart(2, '0')}`;
  return `${Math.floor(h / 24)} ${({ en: 'd', de: 'T' })[i18n.lang] ?? 'j'} ${h % 24} h`;
}
