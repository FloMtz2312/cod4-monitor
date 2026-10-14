# CoD4 Monitor — a tour

**CoD4 Monitor** is a free, open-source web dashboard for **Call of Duty 4 / CoD4X** servers. It follows who plays, when, on which map and how well, and lets the server staff run the server from a browser or a phone.

- Self-hosted: one Go binary + SQLite, a Docker Compose file, HTTPS by Caddy on a small VPS.
- Works with any CoD4X server: it queries the server (UDP), reads its logs (FTP or local disk) and talks rcon.
- English and French, installable as an app on a phone, push notifications.
- License: [AGPL-3.0](../../LICENSE). Source: [github.com/FlorianMartinez01/cod4-monitor](https://github.com/FlorianMartinez01/cod4-monitor).

> All the data in these screenshots is made up: servers, players, chat and IP addresses (documentation ranges).

---

## Live

The home screen shows the selected server as it is right now: map, score of the round, players with their kills, score and ping, the last 24 hours, the player of the week and the week's records. Staff get one-tap actions: change map, restart, message to all, map vote, XP boost.

![Live](desktop-02-live.png)

The **match feed** tells the round as it happens (kills with distance, headshots, noscopes, wallbangs, collaterals, bomb plants, clutches), and the **chat** tab shows the in-game chat live.

| Match feed | Chat |
|---|---|
| ![Match feed](desktop-03-live-match-feed.png) | ![Chat](desktop-04-live-chat.png) |

## Players

Every player who ever joined, searchable by current or old names, with playtime, K/D and last visit.

![Players](desktop-05-players.png)

### Player profile

One page per player: K/D, kills, headshot rate, longest kill, playtime, old names, recent form, last rounds, combat details (weapons, hit locations, kill types), match history. A player can link the profile to themselves ("It's me", confirmed in game with `!me`), share a **Discord card** (PNG) or start a **duel**.

![Player profile](desktop-06-player-profile.png)

| Combat | Matches |
|---|---|
| ![Combat](desktop-07-player-combat.png) | ![Matches](desktop-08-player-matches.png) |

Staff also see a **Moderation** tab: notes and tags (suspect, to watch, VIP), sessions with IPs, chat history, linked accounts and B3 penalties, with kick / ban / B3 actions.

![Player moderation](desktop-09-player-moderation.png)

## Rankings and matches

Rankings by period (week, month, all time) and by metric, with badges.

![Rankings](desktop-10-rankings.png)

Every match is kept with the winner of each round, the MVP and a full scoreboard (points, K/D, headshots, accuracy, damage, noscopes, quickscopes, wallbangs, longest kill), and can be shared as a Discord card.

| Matches | One match |
|---|---|
| ![Matches](desktop-11-matches.png) | ![Match detail](desktop-12-match-detail.png) |

## Advanced stats

Attendance (players per hour and per day, peak times), maps (play time, retention, votes), loyalty (new vs returning players) and combat (weapons, distances, kill types).

| Attendance | Maps |
|---|---|
| ![Attendance](desktop-13-stats-attendance.png) | ![Maps](desktop-14-stats-maps.png) |

| Loyalty | Weapons |
|---|---|
| ![Loyalty](desktop-15-stats-loyalty.png) | ![Weapons](desktop-16-stats-weapons.png) |

### Duel

Who killed whom between two players, all time: kills, headshots, favourite weapon, last kill.

![Duel](desktop-17-duel.png)

## Server health

Availability, server latency, players' ping and outages with their cause: a crash is told apart from a clean stop, with the last error of the game console.

![Server status](desktop-18-server-status.png)

One dashboard can follow **several servers**, each with its own team and settings.

![All servers](desktop-19-all-servers.png)

## Moderation

One inbox for everything that needs a look: in-game `!report`, suspect or watched players coming back, possible **ban evasion**, B3 bans. Each card has the right actions (act in game, view, dismiss).

![Moderation inbox](desktop-20-moderation.png)

| Bans | Chat history |
|---|---|
| ![Bans](desktop-21-bans.png) | ![Chat history](desktop-22-chat-history.png) |

**Ban evasion detection** scores every new player against banned ones (IP range, network settings, names, play style), and **linked accounts** groups the GUIDs that share an IP.

| Ban evasion | Linked accounts |
|---|---|
| ![Ban evasion](desktop-23-ban-evasion.png) | ![Linked accounts](desktop-24-linked-accounts.png) |

### B3

With the B3 admin bot (SQLite or MySQL / MariaDB), its penalties, admins and groups are shown and edited from the dashboard.

![B3](desktop-25-b3.png)

## Server administration

The game console live with rcon commands, and scheduled commands (messages, regular restarts).

| Console | Scheduled commands |
|---|---|
| ![Console](desktop-26-console.png) | ![Scheduled commands](desktop-27-scheduled-commands.png) |

Accounts are created by invitation. The owner of a server gives each moderator exactly the permissions they need (private data, kick, ban, reports, notes, maps, messages, rcon). Every action done from the site is logged.

| Team and invitations | Activity log |
|---|---|
| ![Team](desktop-28-team.png) | ![Activity log](desktop-31-activity-log.png) |

| Server settings | Branding |
|---|---|
| ![Server settings](desktop-29-server-settings.png) | ![Branding](desktop-30-branding.png) |

| More | Search (Ctrl K) |
|---|---|
| ![More](desktop-32-more.png) | ![Search](desktop-33-search.png) |

![Login](desktop-01-login.png)

## On a phone

Built for phones first: a bottom navigation bar, sheets for quick actions, and it installs on the home screen like an app (with push notifications for a server down, a report, a suspect player back or a B3 ban).

| Live | Players | Profile |
|---|---|---|
| ![](mobile-01-live.png) | ![](mobile-02-players.png) | ![](mobile-03-player-profile.png) |

| Rankings | Matches | Match |
|---|---|---|
| ![](mobile-04-rankings.png) | ![](mobile-05-matches.png) | ![](mobile-06-match-detail.png) |

| Moderation | More | Notifications |
|---|---|---|
| ![](mobile-07-moderation.png) | ![](mobile-08-more.png) | ![](mobile-09-notifications.png) |

| Server picker | Player actions |
|---|---|
| ![](mobile-10-server-picker-sheet.png) | ![](mobile-11-player-sheet.png) |

---

**Try it:** the [README](../../README.md) explains how to install it on a VPS or a PC. **Found a bug or have an idea?** [Open an issue](https://github.com/FlorianMartinez01/cod4-monitor/issues/new/choose).
