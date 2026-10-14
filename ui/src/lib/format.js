// Small display helpers shared by the screens.
import { t, locale } from './i18n.svelte.js';

/** CoD4 color codes (^0..^9) → readable colors on the dark background. */
const COLORS = { 0: '#9AA6B5', 1: '#FF6B6B', 2: '#5BE38F', 3: '#FFD866', 4: '#6FA8FF', 5: '#5ED8F2', 6: '#F07CF0', 7: '#E8ECF1', 8: '#E8ECF1', 9: '#9AA6B5' };

/** "^1Ba^7bou" → [{text: "Ba", color: "#FF6B6B"}, {text: "bou", color: "#E8ECF1"}] */
export function colorParts(raw) {
  const parts = [];
  let color = null;
  for (const seg of String(raw ?? '').split(/(\^\d)/)) {
    if (/^\^\d$/.test(seg)) color = COLORS[seg[1]];
    else if (seg) parts.push({ text: seg, color });
  }
  return parts;
}

export const clean = raw => String(raw ?? '').replace(/\^\d/g, '');

const WEAPONS = {
  m40a3: 'M40A3', remington700: 'R700', barrett: 'Barrett', dragunov: 'Dragunov', m21: 'M21',
  beretta: 'M9', colt45: 'M1911', usp: 'USP', deserteagle: 'Desert Eagle', deserteaglegold: 'Desert Eagle Gold',
  frag_grenade: 'Grenade', c4: 'C4', claymore: 'Claymore', knife: 'Couteau'
};

/** "m40a3_acog_mp" → "M40A3" */
export function weaponName(w) {
  if (!w) return '';
  const base = String(w).replace(/_mp$/, '');
  if (WEAPONS[base]) return WEAPONS[base];
  const first = base.split('_')[0];
  return WEAPONS[first] ?? first.toUpperCase();
}

const TAGS = { hs: 'HEADSHOT', wb: 'WALLBANG', ns: 'NOSCOPE', qs: 'QUICKSCOPE', ls: 'LONGSHOT', air: 'AIRBORNE', last: 'LAST KILL', collat: 'COLLAT', triple: 'TRIPLE' };

/** The highlights of a kill, best first: tags of the mod (KE line), else the headshot of the log. */
export function killTags(e) {
  const tags = e.tags ? String(e.tags).split(',').filter(x => TAGS[x]).map(x => TAGS[x]) : [];
  if (!tags.length && e.cause === 'MOD_HEAD_SHOT') tags.push('HEADSHOT');
  return tags;
}

export const teamName = team => team === 'allies' ? t('Alliés') : team === 'axis' ? t('Axe') : '';

/** Seconds ago → "à l'instant", "40 s", "3 min", "2 h", "3 j". */
export function ago(ts, now = Date.now() / 1000) {
  const s = Math.max(0, Math.round(now - ts));
  if (s < 10) return t('à l\'instant');
  if (s < 60) return `${s} s`;
  if (s < 3600) return `${Math.floor(s / 60)} min`;
  if (s < 86400) return `${Math.floor(s / 3600)} h`;
  return `${Math.floor(s / 86400)} ${t('j')}`;
}

/** "il y a 3 min" / "3 min ago", or "à l'instant" / "just now" (never "just now ago"). */
export function agoText(ts, now = Date.now() / 1000) {
  const d = ago(ts, now);
  return d === t('à l\'instant') ? d : t('il y a {d}', { d });
}

export const pingColor = p => p == null ? 'var(--muted)' : p < 60 ? 'var(--ok)' : p < 100 ? 'var(--warn)' : 'var(--danger)';

export const fmt = n => n == null ? '—' : Number(n).toLocaleString(locale(true));


/** Unix seconds → "12/10/2026 21:04" (local time). */
export const dateTime = ts => ts ? new Date(ts * 1000).toLocaleString(locale(), { dateStyle: 'short', timeStyle: 'short' }) : '—';

/** Unix seconds → "12 oct. 2026"; utc: the day of a UTC-based value. */
export const day = (ts, utc = false) => ts ? new Date(ts * 1000).toLocaleDateString(locale(), { day: 'numeric', month: 'short', year: 'numeric', ...(utc ? { timeZone: 'UTC' } : {}) }) : '—';

/** Seconds → "4:05", "1 h 12". */
export function dur(s) {
  if (s == null) return '—';
  s = Math.max(0, Math.round(s));
  if (s >= 3600) return `${Math.floor(s / 3600)} h ${String(Math.floor((s % 3600) / 60)).padStart(2, '0')}`;
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`;
}
