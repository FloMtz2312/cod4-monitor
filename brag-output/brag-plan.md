# Brag Plan: CoD4 Monitor

## What is this app?
A free, open-source, self-hosted web dashboard for Call of Duty 4 / CoD4X servers: live scoreboard, player profiles, rankings, stats, and moderation tools for staff, from a browser or a phone.

## The angle
A 2007 game gets a 2026 control room. Let the real screenshots from `docs/brag/` carry it: each scene is one real screen, framed as a floating app window, with a slow punch-in on the detail that matters. Treat it as a calm presentation: no jokes, just the product.

## Hook (first 2-3 seconds)
"Your CoD4X server, live in a browser." It sits on the left. On the right, the real Live screen (map Crash, Allies 4 vs. Axis 3, round 8) tilts into place.

## Key moments (the middle)
- Live: punch in on the scoreboard and player list (TOxicTim 13/5, 2025 pts).
- Player profile: Kestrel's stat cards (K/D 2.06, 2,681 kills, 40 % headshots, 97 m longest kill).
- Moderation: punch in on the ban-evasion card "TOxicTim looks like Toxic.Tim, Score 100".
- Phone: three mobile screens (Live, Profile, Moderation) arrive one by one.

## Outro / punchline
"CoD4 Monitor": free and open source. One Go binary + SQLite. github.com/FlorianMartinez01/cod4-monitor

## User flow worth showing
Open Live (watch the round) → open a player (their whole history) → moderation inbox (act on ban evasion). The same flow then plays on a phone.

## Tone
- Preset: polished (with app-store's one-feature-per-scene structure)
- Creative direction: quiet, premium product presentation for a gaming tool
- Interpretation: slow punch-ins, soft crossfades, mixed-case type, generous holds. Fast entrances (≤0.6s) followed by long reads.

## Format: landscape, 1920x1080
## Duration: 25s

## Visual identity (from the project)
- Background: #0B0E13 (surface #141922, line #263042)
- Accent: #E5484D (brand red sampled from the screenshots; the CSS default `--accent` is amber #F5A524, overridden by branding)
- Text: #E8ECF1, muted #9AA6B5; allies #5EA0FF, ok #3DD68C
- Display font: Space Grotesk. Body font: Manrope (both shipped in `ui/public/fonts`).
- Strongest visual element: the Live hero card (map art, ALLIES 4 / AXIS 3, red "Join the game").

## Share copy (draft)
CoD4 Monitor: a free, open-source dashboard for your CoD4X server. Live scoreboard, player profiles, and ban-evasion detection, from your browser or your phone.

## Audio direction
- Role: warm bed + sparse professional accents
- Music: happy-beats-business-moves-vol-12 (steady, clean; 110 BPM)
- Music treatment: 0.32 bed with a 0.6s fade-in and a fade-out over the last 1.5s
- Music cue guidance: bundled preset `cues/happy-beats-business-moves-vol-12-by-ende-dot-app.music-cues.json`. Strong cues 8.74 (profile), 13.11 (moderation), 22.93 (logo). Phones on every other beat: 17.47 / 18.56 / 19.66.
- Audio-reactive treatment: none. Polished restraint; the screenshots carry the visual.
- SFX posture: sparse. A soft impact on each scene change, card slides for the phones, one bell on the logo.
- Restraint rule: no SFX on the text; nothing louder than the logo bell.

## Storyboard

### Scene 1: Hook (0–3.8s)
Left: "Your CoD4X server," / "live in a browser." Right: desktop-02-live in a window, tilting from 3D into flat.
Sequential/interaction: two headline lines, 0.3s apart, then a hold
Audio intent: music fades in; soft drop when the window lands
Transition: soft crossfade → 2

### Scene 2: Live (3.8–8.74s)
Kicker "LIVE". Headline "Every round, as it happens." Sub "Map, score, players, ping. Change map or restart in one tap." Window: desktop-02-live, punch-in on the scoreboard + player list.
Audio: soft impact at the cut. Transition: crossfade → 3

### Scene 3: Players (8.74–13.11s, beat-locked)
Kicker "PLAYERS". "One page per player." Sub "K/D, headshots, longest kill, old names, match history." Window: desktop-06-player-profile, punch-in on the stat cards.
Transition: crossfade → 4

### Scene 4: Moderation (13.11–17.47s, beat-locked)
Kicker "MODERATION". "Ban evasion, spotted." Sub "Reports, suspects, and B3 bans in one inbox." Window: desktop-20-moderation, punch-in on the "TOxicTim looks like Toxic.Tim" card.
Transition: crossfade → 5

### Scene 5: Phone (17.47–21.84s)
Headline centered at top: "Built for your phone first." Three phones (mobile-01, 03, 07) rise one by one at 17.47 / 18.56 / 19.66, each with a card slide.
Transition: soft fade → 6

### Scene 6: Outro (21.84–25s)
"CoD4 Monitor" lands at 22.93 (bell). Below: "Free & open source · One Go binary + SQLite" and the GitHub URL.

**Music mood:** steady, clean, confident
**Audio summary:** a calm bed that opens with the hook, soft thuds on each feature change, card slides under the phones, and one bell on the name.
