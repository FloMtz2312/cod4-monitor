// The PNG card of a player (or a match) to post on Discord: shared with the phone's share sheet, else downloaded.
import { say } from './toast.svelte.js';
import { t } from './i18n.svelte.js';

export async function shareCard(serverId, type, value) {
  try {
    const params = new URLSearchParams({ type, [type === 'player' ? 'key' : 'match']: value });
    const r = await fetch(`/api/servers/${serverId}/card.png?${params}`);
    if (!r.ok) throw new Error(t('Carte indisponible : aucune donnée enregistrée.'));
    const blob = await r.blob();
    const file = new File([blob], `cod4-${type}.png`, { type: 'image/png' });
    if (navigator.canShare?.({ files: [file] })) {
      try {
        await navigator.share({ files: [file], title: 'CoD4' });
        return;
      } catch (e) {
        if (e.name === 'AbortError') return;
      }
    }
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = file.name;
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 10000);
    say(t('Carte PNG téléchargée · prête à joindre dans Discord'));
  } catch (e) {
    say(e.message);
  }
}
