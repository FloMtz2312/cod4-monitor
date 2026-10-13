<script>
  // Last events of the match, newest first: kills with their highlights, bomb, clutches, round ends.
  import CodName from './CodName.svelte';
  import { weaponName, killTags, teamName, ago } from '../lib/format.js';
  import { t } from '../lib/i18n.svelte.js';

  /** @type {{ events: any[], now: number }} */
  let { events, now } = $props();
  const teamColor = team => team === 'allies' ? 'var(--allies)' : team === 'axis' ? 'var(--axis)' : 'var(--text)';
</script>

<section class="card feed" aria-label={t('Fil du match')}>
  {#each events as e, i (i)}
    <div class="ev">
      <span class="when">{ago(e.ts, now)}</span>
      <span class="what">
        {#if e.type === 'kill'}
          <span class="line">
            {#if e.actor && e.actor !== e.target}
              <span class="who" style:--team={teamColor(e.actor_team)}><CodName raw={e.actor} /></span><span class="arrow">›</span>
            {/if}
            <span class="who" style:--team={teamColor(e.target_team)}><CodName raw={e.target} /></span>
          </span>
          <span class="sub">
            {#if !e.actor}{t('mort seul')}{:else if e.actor === e.target}{t('suicide')}{:else}{weaponName(e.weapon)}{#if e.distance}{' · '}{Math.round(e.distance)} m{/if}{#if e.teamkill}{' · '}{t('tir allié')}{/if}{/if}
          </span>
        {:else if e.type === 'round'}
          <span class="line">{e.winner === 'allies' || e.winner === 'axis' ? t('Round gagné par : {team}', { team: teamName(e.winner) }) : t('Round terminé')}</span>
          {#if e.score_allies != null}<span class="sub">{teamName('allies')} {e.score_allies} – {e.score_axis} {teamName('axis')}</span>{/if}
        {:else if e.type === 'plant'}
          <span class="line">{t('Bombe posée')}</span><span class="sub"><CodName raw={e.actor} /></span>
        {:else if e.type === 'defuse'}
          <span class="line">{t('Bombe désamorcée')}</span><span class="sub"><CodName raw={e.actor} /></span>
        {:else if e.type === 'clutch'}
          <span class="line"><CodName raw={e.actor} />&nbsp;{t('gagne un 1 contre {n}', { n: e.value })}</span>
        {/if}
      </span>
      {#if e.type === 'kill'}
        {@const tags = killTags(e)}
        {#if tags.length}<span class="tag">{tags[0]}</span>{/if}
      {/if}
    </div>
  {:else}
    <p class="empty">{t('Aucun événement pour le moment : le fil se remplit avec les logs du serveur.')}</p>
  {/each}
</section>

<style>
  .feed { overflow: hidden; }
  .ev { display: flex; gap: 12px; align-items: center; padding: 11px 14px; border-top: 1px solid var(--line-soft); }
  .ev:first-child { border-top: 0; }
  .when { width: 54px; flex: none; color: var(--muted); font-size: 12px; font-weight: 600; }
  .what { flex: 1; min-width: 0; display: flex; flex-direction: column; }
  .line { display: flex; align-items: center; min-width: 0; font-weight: 700; font-size: 14px; }
  .who { display: flex; min-width: 0; border-bottom: 2px solid var(--team); }
  .arrow { color: var(--muted); padding: 0 6px; }
  .sub { font-size: 12px; color: var(--muted); display: flex; min-width: 0; }
  .tag { flex: none; font-size: 10px; font-weight: 800; padding: 3px 7px; border-radius: 7px; background: var(--accent-soft); color: var(--accent); letter-spacing: .03em; }
  .empty { margin: 0; padding: 18px 16px; font-size: 14px; color: var(--muted); }
</style>
