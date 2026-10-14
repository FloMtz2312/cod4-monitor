// Shared bits of the moderation and administration screens: dates, B3 penalty labels, ban evasion clues.
import { t, locale } from './i18n.svelte.js';

/** Short date and time of a unix timestamp ("12/10/2026 21:04"). */
export const dt = ts => (ts ? new Date(ts * 1000).toLocaleString(locale(), { dateStyle: 'short', timeStyle: 'short' }) : '—');
/** Short date only. */
export const day = ts => (ts ? new Date(ts * 1000).toLocaleDateString(locale(), { day: '2-digit', month: '2-digit', year: '2-digit' }) : '—');

/** Staff tags of the player notes, as shown next to a name ("vip" → "★ VIP"). */
export const TAG_LABEL = { suspect: '⚠ Suspect', surveiller: '👁 À surveiller', vip: '★ VIP', ami: '♥ Ami' };
export const tagLabel = tag => t(TAG_LABEL[tag] ?? tag);

export const PEN_LABEL = { Ban: 'Ban', TempBan: 'Ban temporaire', Kick: 'Kick', Warning: 'Avertissement', Notice: 'Note' };
export const PEN_COLOR = { Ban: '#FF6B5E', TempBan: '#FFA94D', Kick: '#B18CFF', Warning: '#F0C56B', Notice: '#8DBBFF' };

/** End of a B3 penalty: "—", "levé", "jamais", "expiré" or its date. */
export function penEnd(x) {
  if (x.type === 'Kick' || x.type === 'Notice') return '—';
  if (x.inactive) return t('levé');
  if (x.time_expire === -1) return t('jamais');
  if (x.time_expire * 1000 < Date.now()) return t('expiré');
  return dt(x.time_expire);
}

/** Who gave a B3 penalty, and how. */
export const penBy = x => (x.origin === 'auto' ? `B3 · ${t('auto')}` : x.origin === 'site' ? x.admin : `${x.admin} · ${t('en jeu')}`);

export const CLUES = {
  ip: 'Même IP', steam: 'Même Steam ID', profile: 'Même profil de carrière', name: 'Même pseudo', subnet: 'Même réseau (IP proche)',
  veteran: 'Nouveau GUID, profil avancé', settings: 'Mêmes réglages réseau', name_near: 'Pseudo proche', style: 'Style de jeu proche', new: 'GUID apparu après le ban'
};
