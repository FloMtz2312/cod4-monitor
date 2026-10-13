# CoD4 Monitor

🇬🇧 **English** · 🇫🇷 [Français](README.fr.md)

A web dashboard to follow a **Call of Duty 4 / CoD4X** server — who plays, when, on which map, with which stats — and to run it from a browser or a phone.

- **Live**: server status, online players, map, console and chat.
- **Stats**: activity, rankings, badges, records, match history, a profile for every player with their progression.
- **Admin**: map changes, kick and ban, scheduled commands, action log, ban evasion detection.
- **Access**: three account levels (admin, semi-admin, visitor), English and French, usable as an app on a phone.

---

## Contents

1. [Before you start](#1-before-you-start)
2. [Install on a VPS](#2-install-on-a-vps-recommended) (recommended)
3. [Install on your PC](#3-install-on-your-pc-to-try-it) (to try it)
4. [Update](#4-update)
5. [Configuration (`.env`)](#5-configuration-env)
6. [Accounts and roles](#6-accounts-and-roles)
7. [What's in the dashboard](#7-whats-in-the-dashboard)
8. [FTP logs and RCON commands](#8-ftp-logs-and-rcon-commands)
9. [Backups](#9-backups)
10. [Troubleshooting](#10-troubleshooting)
11. [Command cheat sheet](#11-command-cheat-sheet)
12. [For developers](#12-for-developers)
13. [Game server on the same machine and advanced settings](#13-game-server-on-the-same-machine-and-advanced-settings)
14. [B3: penalties, admins and grades from the dashboard](#14-b3-penalties-admins-and-grades-from-the-dashboard)
- [License](#license)

---

## 1. Before you start

Gather this information. Only the first two items are required.

| What | Example | What it's for |
|---|---|---|
| **IP and port of the CoD4 server** | `203.0.113.10:55555` | Querying the server (required) |
| **A strong password** for the dashboard | — | Protecting the site (required on a VPS) |
| The server's `rcon_password` | in its `server.cfg` | Sending commands from the site (map, kick, ban…) |
| FTP access to the mod folder | `ftp://user:pass@ip:21/mods/my_mod/` | Console, chat, kills, connections with IPs |

To put it online, you also need:

- **a VPS** running Ubuntu 24.04 or Debian 12, with 1 to 2 GB of RAM (OVH, Hetzner, Contabo…);
- **a web address**. You can get one for free from [DuckDNS](https://www.duckdns.org), for example `my-server.duckdns.org`.

> 💡 You can do everything from a phone: use the **Termius** app (iPhone / Android) to connect to the VPS, and the browser for GitHub and DuckDNS.

---

## 2. Install on a VPS (recommended)

The site runs in Docker behind **Caddy**, which gets and renews the HTTPS certificate by itself.

### Step 1: point your address to the VPS

On [duckdns.org](https://www.duckdns.org), sign in, create a subdomain (e.g. `my-server`) and paste your **VPS IP** into the *current ip* field. Confirm with *update ip*.

### Step 2: connect to the VPS

From a terminal (or Termius, or your host's web console):

```sh
ssh ubuntu@VPS_IP
```

Replace `ubuntu` with the user your host gave you (sometimes `root`).

### Step 3: get the project

Install git, then clone the project into `/opt/cod4-monitor`:

```sh
sudo apt update && sudo apt install -y git
sudo mkdir -p /opt/cod4-monitor && sudo chown $USER /opt/cod4-monitor
git clone https://github.com/FlorianMartinez01/cod4-monitor.git /opt/cod4-monitor
cd /opt/cod4-monitor
```

### Step 4: prepare the VPS

One command installs Docker, turns on the firewall (only SSH, HTTP and HTTPS stay open) and enables automatic security updates:

```sh
sudo sh deploy/setup-vps.sh
sudo usermod -aG docker $USER
```

**Log out and log back in** to the VPS so you can use `docker` without `sudo`.

> ⚠️ If your host also has a firewall in its control panel, open ports **80** and **443** there too.

### Step 5: configure

```sh
cd /opt/cod4-monitor
cp .env.example .env
nano .env
```

Fill in at least these lines:

```ini
SERVERS=My Server|203.0.113.10:55555
DASHBOARD_USER=admin
DASHBOARD_PASSWORD=a-long-and-unique-password
DOMAIN=my-server.duckdns.org
```

Save with **Ctrl+O**, then **Enter**, and quit with **Ctrl+X**.

For the console, chat, commands and every option, see [Configuration](#5-configuration-env) and [FTP logs and RCON commands](#8-ftp-logs-and-rcon-commands).

### Step 6: start

```sh
mkdir -p backups && sudo chown 1000 backups && chmod 600 .env
docker compose -p cod4-monitor -f docker-compose.prod.yml up -d --build
```

The first build takes a few minutes. Then check that everything is running:

```sh
docker compose -p cod4-monitor -f docker-compose.prod.yml ps
```

Both lines (`monitor` and `caddy`) should say **Up**. If `monitor` says *Restarting*, see [Troubleshooting](#10-troubleshooting).

### Step 7: open the site

Go to `https://my-server.duckdns.org`. The certificate can take a minute to arrive the first time.

- **Without logging in**, you see the site as a visitor would: the public stats.
- **Log in** (top right) with `DASHBOARD_USER` and `DASHBOARD_PASSWORD` to get full admin access.
- To give someone else access: **More → Staff and invitations** (an invitation link) (see [Accounts and roles](#6-accounts-and-roles)).

📱 **On a phone**: open the site, then tap *Share* → **Add to Home Screen** (iPhone) or *menu* → **Add to Home screen** (Android). The site then opens like an app.

> ℹ️ **Already running a monitor on your PC?** Stop it (`docker compose down`). Two monitors on the same server would both send the scheduled commands.

---

## 3. Install on your PC (to try it)

You need [Docker Desktop](https://www.docker.com/products/docker-desktop/). In the project folder:

```sh
cp .env.example .env      # then edit at least SERVERS
docker compose up -d
```

Then open http://localhost:8080 (or the port set in `HTTP_PORT`).

- Without `DASHBOARD_PASSWORD`, no login is asked, but commands only work from the PC itself (`localhost`).
- To open the site from a phone **on the same Wi-Fi**, set a password, then go to `http://PC_IP:8080`. On Windows, allow the port in the firewall once (PowerShell as administrator):
  ```powershell
  New-NetFirewallRule -DisplayName "CoD4 Monitor (LAN)" -Direction Inbound -Protocol TCP -LocalPort 8080 -Action Allow -Profile Private
  ```
  The traffic isn't encrypted, so keep it on your local network. For access from the internet, use the VPS install.

To start the monitor with Windows, turn on *Start Docker Desktop when you sign in* in Docker Desktop's settings.

---

## 4. Update

On the VPS:

```sh
cd /opt/cod4-monitor && git pull && docker compose -p cod4-monitor -f docker-compose.prod.yml up -d --build
```

Your `.env`, the database and the backups are kept. The automated tests run during the build. If they fail, the build stops and **the previous version stays online**.

> ⏰ GitHub tokens expire. When `git pull` refuses to connect, create a new token (see [step 3](#step-3-get-the-project)), then run `rm ~/.git-credentials && git pull` and paste the new token.

**Option: deploy from your PC.** `sh deploy/push.sh ubuntu@VPS_IP` uploads your local folder (`.env` included) and your local database (only if the VPS doesn't have one yet), then starts the site. ⚠️ This script **replaces the VPS `.env`** with the one from your PC.

---

## 5. Configuration (`.env`)

After any change to `.env`, restart with the start command from [step 6](#step-6-start). The comments inside `.env.example` are in French. This section describes every variable.

### The essentials

| Variable | Example | Purpose |
|---|---|---|
| `SERVERS` | `My Server\|203.0.113.10:55555` | The server(s) to follow at the **first start** (details below). After that, they are managed from the site |
| `DASHBOARD_USER` | `admin` | **Admin** username. **Required**: without it, nobody can log in as admin |
| `DASHBOARD_PASSWORD` | — | Admin password. **Empty = no login at all**: only for use on your own PC |
| `DOMAIN` | `my-server.duckdns.org` | Public address (VPS only) |
| `TZ` | `Europe/Paris` | Time zone used for display |

### The `SERVERS` format

> **`SERVERS` is only read at the first start**: the servers are then copied into the database. After that, the admin adds, edits, tests, stops or resumes them from the **Admin → Monitored servers** tab (address, FTP, rcon), without restarting the monitor. Once the servers are in the database, changing `SERVERS` has no effect. You can even leave it empty and add your first server from the site.

Each server is written as up to 4 fields separated by `|`. Only the first two are required:

```
Name|ip:port|ftp_url_of_the_mod_folder|rcon_password
```

Full example:

```ini
SERVERS=My Server|203.0.113.10:55555|ftp://user:password@203.0.113.10:21/mods/my_mod/|myrcon
```

- **Several servers**: separate them with `;`. A drop-down list then appears on the site.
- **Special characters** in the FTP password must be replaced: `@` → `%40`, `:` → `%3A`, `/` → `%2F`, `#` → `%23`, `%` → `%25`.

### Other settings (the defaults are fine)

| Variable | Default | Purpose |
|---|---|---|
| `POLL_INTERVAL` | `5s` | How often the server is queried (5 s minimum) |
| `QUERY_TIMEOUT` | `2s` | How long to wait for a reply |
| `OFFLINE_AFTER` | `45s` | Time without a reply before showing "offline" (a map change doesn't reply for 20–40 s) |
| `LOG_POLL_INTERVAL` | `2s` | How often the logs are read over FTP |
| `SERVER_LOG_TZ` | `UTC` | Time zone of the game server's clock |
| `HTTP_PORT` | `8080` | Port of the site (PC install) |
| `RETENTION_DAYS` | `90` | How many days polls are kept (`0` = forever) |
| `BACKUP_KEEP` | `14` | Number of daily backups kept |
| `DISCORD_WEBHOOK_URL` | — | Discord alerts when the server goes down or comes back |
| `DISCORD_PLAYER_THRESHOLD` | `0` | Discord alert when the player count reaches this number (`0` = off) |

### Emails (optional)

The monitor can email you (the admin). Fill in an email account it sends from, and the address that receives:

| Variable | Example | Purpose |
|---|---|---|
| `SMTP_HOST` | `smtp.gmail.com` | Mail server of the sending account. **Empty = no emails.** |
| `SMTP_PORT` | `587` | `587` (STARTTLS) or `465` (TLS) |
| `SMTP_USER` / `SMTP_PASSWORD` | `my.monitor@gmail.com` | Login of the sending account |
| `MAIL_FROM` | — | Sender address (default: `SMTP_USER`) |
| `MAIL_TO` | `me@outlook.com` | Who receives the emails (several: separate with `,`) |
| `MAIL_LANG` | `fr` | `fr` or `en` |
| `WEEKLY_MAIL` | — | `0` = no weekly summary |
| `VPS_DISK_ALERT` / `VPS_MEM_ALERT` | `90` | Disk / memory alert threshold, in % |
| `VPS_LOAD_ALERT` | `2` | CPU alert: 15-minute load per core |

**With Gmail** (the simplest): turn on 2-step verification on the account, create an *app password* (Google account → Security → App passwords) and put it in `SMTP_PASSWORD`. A free [Brevo](https://www.brevo.com) account also works (`smtp-relay.brevo.com`, port 587).

#### With Resend (an address on your own domain)

[Resend](https://resend.com) sends the emails from an address on your domain (e.g. `monitor@my-domain.com`), and they rarely land in spam. The free plan is plenty: 3,000 emails a month, 100 a day.

You need **a domain of your own** (a few euros a year at OVH, Gandi, Cloudflare…): Resend asks you to add DNS records, which a DuckDNS address can't do.

1. Create an account on [resend.com](https://resend.com).
2. **Domains → Add Domain**: enter your domain. A subdomain (e.g. `mail.my-domain.com`) keeps your main domain out of it.
3. Resend shows DNS records (an `MX` and a `TXT` for SPF, a `TXT` for DKIM). Add them as they are in your domain's DNS zone, at your registrar, then click **Verify DNS Records**. The domain turns *Verified* within minutes, sometimes a few hours.
4. **API Keys → Create API Key**: name `cod4-monitor`, permission **Sending access**, limited to your domain. Copy the key (it starts with `re_`): it is shown only once.
5. In the monitor's `.env`:
   ```sh
   SMTP_HOST=smtp.resend.com
   SMTP_PORT=465
   SMTP_USER=resend
   SMTP_PASSWORD=re_your_api_key
   MAIL_FROM=monitor@my-domain.com
   MAIL_TO=me@example.com
   ```
   `SMTP_USER` is always the word `resend`. `MAIL_FROM` is required: an address on your verified domain (it doesn't need to exist as a mailbox). Port `587` works too.
6. Restart the monitor (`docker compose … up -d`). A test email is sent at startup; you also see it under **Emails** on Resend's site. If it doesn't arrive: `docker logs cod4-monitor-monitor-1 | grep mail`.

For a quick test without a domain, Resend accepts `MAIL_FROM=onboarding@resend.dev`, but only to your Resend account's address.

What you receive:

- **A test email** when the monitor starts with new email settings: if it doesn't arrive, look at `docker logs cod4-monitor-monitor-1 | grep mail`.
- **VPS alerts**: disk almost full, memory or CPU saturated for more than 5 minutes, monitor restarting in a loop. Then a reminder every 24 h while it lasts, and an email when it's over.
- **Weekly summary**, on Monday from 9:00: players, play time, peak, matches, player of the week, records, top 5, most attended map.
- **Server crash**: when the game server stops answering, restarts on its own (crash + automatic restart) or comes back, with the probable cause (last error of its console, e.g. `ERROR: Exceeded limit of 1000 'xmodel' assets`) and the last console lines. After a restart, the end of the previous run is read in `qconsole.log.old` (needs the FTP logs). A normal shutdown (quit, restart by the host) is only recorded, not mailed. The cause also shows under each outage on the home page.
- **Player alerts**: when a player connects who has an admin note **⚠ Suspect** or **👁 To watch**, or a `!report` not handled yet (last 30 days). At most one email per player every 6 hours.

### Push notifications (phone)

Each account of the staff can also get **notifications on its phone, even with the app closed**: **More → Notifications** → *Turn on notifications*, then one switch per alert:

- **Server down or back** (crash, unexpected restart, back online, with the probable cause): any account with rights on the server.
- **New report** (`!report` in game): accounts allowed to handle reports or see private data.
- **Suspect or banned player back** (abnormal stats, player tagged suspect / to watch in game, possible ban evasion) and **B3 ban**: accounts that see private data.

Nothing to configure: no third-party account, no `.env` variable. The monitor creates its own key (VAPID) on the first start and keeps it in its database; the phone's push service (Google, Apple, Mozilla, Microsoft) delivers the notifications. Permissions are checked at each sending, so removing a right also stops its notifications. **iPhone**: iOS 16.4 or later, and only from the app added to the home screen (Safari → Share → *Add to Home Screen*, then open it from there). Each phone or browser has its own choices, and can be muted per server. The emails above don't change.

---

## 6. Accounts and roles

| Role | Who | Sees | Can do |
|---|---|---|---|
| 👑 **Admin** | the `.env` account | everything | everything: commands, kick, ban, map, accounts, notes |
| 🛡 **Semi-admin** | accounts created by the admin | everything: console, chat, IPs, stats, ban evasion | only change their own password |
| 👀 **Visitor** | anyone, no account | the public stats: status, players, activity, rankings, matches | nothing |

- **Give access to someone**: **More → Staff and invitations** → an invitation link valid 48 h, with the permissions of their choice. The `.env` account manages every account in **More → Accounts** (new password, delete).
- **Visitors never see** IPs, the console, the chat or admin notes. The server itself blocks this data; it isn't just hidden on the page.
- **Security**: passwords are hashed (PBKDF2-SHA256), a login lasts 30 days, and an IP is locked out for 15 minutes after 10 failed attempts.

---

## 7. What's in the dashboard

The site is a phone-first app (bottom tabs on a phone, side menu on a computer). It can be installed on the home screen (**More → Install the app**).

| Screen | Content |
|---|---|
| **Live** | Server card (status, map, live match score, join / share), latency, ping and 30-day availability, server details, online players, today's activity, the last 7 days (player of the week, top 5, records). For the staff with the rights: change map (search, images, game mode), restart, messages, kick / ban, end-of-match map vote pool, XP boost. |
| **Moderation** (staff) | Inbox: reports, abnormal stats, tagged players in game, possible ban evasions, B3 bans. Tools: chat with search, live game console, server bans and unban, ban evasion clues, names per GUID and accounts linked by IP, B3 (penalties, admins and grades, lookup, statistics). |
| **Players** | Every player of the period (filters: online, new, noted), search by current or old name (and IP for the staff). On a computer, a table with playtime, sessions, ping, best score, messages… |
| **Player profile** | Summary (form vs previous period, compare with a player, progression K/D / headshots / accuracy), combat (weapons, hit zones, nemeses), matches, moderation (note, welcome message, sessions, IPs, linked accounts, chat, B3 grade and penalties). **Copy link** and **Duel** buttons. |
| **Rankings** | Badges, records (player of the day and of the week), ranking by metric (full sortable table on a computer), community challenges (the owner sets the goals), duel. |
| **More** | Matches and rounds (scoreboards, highlights), advanced stats (attendance, maps with verdicts keep / replace, retention, combat), server status (availability, outages and their cause, latency, players' ping, console errors, server variables, log collection and backups), notifications, and the administration: customization, staff and invitations, server settings, accounts, RCON console, scheduled commands, action log. |

Also:

- **On a computer**: side menu, search with **Ctrl K** or **/** (players by current or old name, screens).
- **Old links** keep working: `/v2/…`, `/login.html`, `/invite.html#…`, `#accueil`, `#joueur/<key>`, `#admin`…
- **Language**: 🇬🇧 / 🇫🇷 button at the top right. By default, the site uses the browser's language.
- **Badges**: the best player of the period in 7 categories. The icon shows next to their name everywhere on the site.

---

## 8. FTP logs and RCON commands

### FTP logs (console, chat, kills, connections)

With the FTP URL of the mod folder as the 3rd field of `SERVERS`, the monitor reads three files:

| File | What it brings |
|---|---|
| `qconsole.log` | Live console (nothing is stored) |
| `games_mp.log` | Chat, kills, rounds, accuracy, name ↔ GUID links |
| `enterleave.log` | Joins and leaves with IP and GUID. The whole history is imported on first start. |

Without FTP, the site still works, but only with the server's public info: players, scores, map.

### RCON commands

With the `rcon_password` as the 4th field of `SERVERS`, the admin gets:

- **map and mode changes** from a gallery of the server's maps (stock maps and `usermaps/`, read over FTP);
- **next map, restart, message to all players, free command**;
- **kick and ban** from the player list or a player's profile;
- **end-of-match vote maps and XP boost**;
- **scheduled commands**, every N minutes or every day at a set time.

Every action is recorded in the **action log** (More → Action log), together with the account that did it.

> The mod can't be changed on the fly (CoD4X protects `fs_game`). It's set in the server's startup parameters (OGP).

### Advanced stats with the `pm_sniper` mod

The `pm_sniper` mod (separate repository) writes extra lines to `games_mp.log`. The monitor reads them automatically, no setting needed:

| Line | What it brings to the dashboard |
|---|---|
| `KE` | Distance of every kill, noscope, quickscope, wallbang, points |
| `RE` / `CL` | Winning team and score of every round, clutches (last one alive who wins) |
| `P_P` / `P_D` | Bomb planted / defused (already in Promod) |
| `MS` / `ME` | Official end-of-match score, level and prestige of every player, match winner |
| `MV` / `MVW` | Results of the end-of-match map vote |
| `PI` | Player profile on arrival (level, prestige, career stats): ban evasion detection |

They show up in the ranking (noscopes, quickscopes, wallbangs, longest kill, clutches, first blood, win %), records and badges, matches (winner, score, highlights of every round), the player profile and the Maps tab (map votes). Without this mod, those columns and sections are hidden and everything else works.

### In-game chat commands

With the `rcon_password` in `SERVERS`, the monitor answers players with a private message (`tell`):

| Command | Reply |
|---|---|
| `!stats` or `!stats name` | Kills, deaths, K/D, headshot %, noscopes, longest kill |
| `!top` | Top 5 killers of the last 7 days |
| `!report name reason` | Report sent to the admins: it shows up in **Moderation** (and as a push notification) |
| `!me` | A short link to type on a phone (`your-domain/me/K7QPX`, or a code without `DOMAIN`): it opens the player's profile in the new interface and pins it (**It's me**). Valid 24 hours |
| `!help` | List of commands (on a server with B3, B3 answers: its list includes these commands) |

One command every 3 seconds per player, one report per minute.

### Ban evasion (GUID spoofer)

**Moderation → Ban evasions**: every player active since a ban is compared with the banned player, and each clue adds points. A live alert shows from **50**. Nothing is banned automatically: it's up to you to check.

| Clue | Points |
|---|---|
| Same IP, or same Steam ID | 60 |
| Same career profile: same prestige, career kills just above the banned player's (the profile stays on the player's PC, even with a new GUID) | 40 |
| Same name (without colours or l33t) | 25 |
| Same network (IP differing only by its last number) | 20 |
| New GUID arriving with a big profile already (level 10+ or a prestige) | 20 |
| Same network settings, when not the defaults (`rate`, `snaps`, `cl_maxpackets`, read with rcon `dumpuser`) | 15 |
| Name 1 or 2 letters away | 15 |
| Similar play style (headshot %, noscope %, kill distance) | 10 |
| GUID first seen after the ban | 10 |

Weak clues only count on top of a clue linking the player to the banned one, and a player already known before the ban is only flagged on a strong clue (IP, Steam ID, profile, name). The profile comes from the `PI` line of the `pm_sniper` mod, the network settings need the rcon password. A cheater who changes GUID, IP (VPN), CoD4 profile, name and play style all at once stays undetectable.

**Welcome message**: in a player's profile, the **admin** (not a semi-admin) can write a personal message, sent privately 30 seconds after each time that player connects. `{name}` is replaced by their name.

**Custom map thumbnails**: the thumbnails are in `ui/public/maps/`. After adding maps to your game, regenerate them from your PC:

```sh
docker run --rm -v "$PWD":/src -v "<cod4 folder>":/game:ro -w /src golang:1.26-alpine go run ./cmd/mapimages /game ui/public/maps
```

---

## 9. Backups

Every day, a copy of the database is written to `backups/monitor-YYYY-MM-DD.db`. The last 14 are kept (`BACKUP_KEEP`).

> ⚠️ These backups are **on the VPS itself**. Copy one somewhere else from time to time. From your PC: `scp ubuntu@VPS_IP:/opt/cod4-monitor/backups/monitor-*.db .`

**Restore a backup** (on the VPS):

```sh
cd /opt/cod4-monitor
docker compose -p cod4-monitor -f docker-compose.prod.yml stop monitor
docker run --rm -v cod4-monitor_data:/data -v "$PWD/backups":/b alpine \
  sh -c 'cp /b/monitor-2026-10-06.db /data/monitor.db && rm -f /data/monitor.db-wal /data/monitor.db-shm && chown -R 1000 /data'
docker compose -p cod4-monitor -f docker-compose.prod.yml start monitor
```

Replace the date with the one of the backup you want.

Kept forever: chat, names, kills, rounds, connections, bans, action log. Polls and sessions are purged after `RETENTION_DAYS` days.

---

## 10. Troubleshooting

| Problem | Fix |
|---|---|
| `monitor` shows **Restarting** | Find the cause with `docker logs --tail 20 cod4-monitor-monitor-1`, then see the lines below. |
| Logs: `attempt to write a readonly database` | The data folder has the wrong owner. Run `docker run --rm -v cod4-monitor_data:/data alpine chown -R 1000:1000 /data`, then `docker restart cod4-monitor-monitor-1`. |
| Logs: `invalid SERVERS entry` | The `SERVERS` line is malformed. The format is `Name\|ip:port`, with no spaces around the `\|`. |
| Logs: `invalid log URL` | The FTP URL must start with `ftp://`, and special characters in the password must be encoded (see [here](#the-servers-format)). |
| The site doesn't open / certificate error | Check that DuckDNS points to the VPS IP, and that ports 80 and 443 are open (VPS firewall **and** your host's). See `docker logs cod4-monitor-caddy-1`. |
| Can't log in as admin | Is `DASHBOARD_USER` set in `.env`? After changing it, restart the site. After 10 failed attempts, wait 15 minutes. |
| "Logs not configured" in the console | Add the FTP URL as the 3rd field of `SERVERS`. |
| No "Server commands" section | Add the `rcon_password` as the 4th field of `SERVERS`, and log in as **admin**. |
| `git pull`: *Invalid username or token* | The GitHub token has expired or is wrong: run `rm ~/.git-credentials`, then `git pull` and paste a new token. |
| `fatal: not a git repository` | You're not in the right folder: `cd /opt/cod4-monitor`. |
| `permission denied` with `docker` | Run `sudo usermod -aG docker $USER`, then log out and back in. |
| iPhone: the top of the page is hidden | Update the site, then fully close the app and reopen it. |

---

## 11. Command cheat sheet

Run these on the VPS, in `/opt/cod4-monitor`. To type less, create an alias once:

```sh
echo "alias cod4='docker compose -p cod4-monitor -f /opt/cod4-monitor/docker-compose.prod.yml'" >> ~/.bashrc && source ~/.bashrc
```

| Action | Command |
|---|---|
| Container status | `cod4 ps` |
| Live logs (Ctrl+C to quit) | `cod4 logs -f monitor` |
| Restart | `cod4 restart` |
| Stop | `cod4 down` (the database is kept) |
| Start / apply an edited `.env` | `cod4 up -d` |
| Update | `git pull && cod4 up -d --build` |

---

## 12. For developers

The monitor is written in Go (Gin, SQLite without CGO). The interface (`ui/`, Svelte) is built by the `Dockerfile` and embedded in the binary.

```sh
go test ./...                              # tests
go run ./cmd/probe 203.0.113.10:55555     # query a server once and print the reply
DB_PATH=monitor.db go run ./cmd/monitor    # run the monitor locally
```

- **Interface**: a Svelte app in `ui/`, served at the root of the site. To work on it, run the monitor (port 8080), then:

  ```sh
  cd ui && npm ci
  npm run dev     # http://localhost:5173/ , the /api calls go to the monitor on port 8080
  npm run check   # Svelte check (also in CI)
  npm run build   # ui/dist, embedded in the binary by go build
  ```

  Without `npm run build`, the site answers "Interface non compilée". Texts are written in French; the English translations are in `ui/src/lib/i18n.svelte.js` and `ui/src/lib/en/*.js`.
- **Roles**: the list of routes each role may call is in `internal/api/auth.go` (`allowed`).
- **Database**: `docker compose cp monitor:/data/monitor.db .`, then open it with `sqlite3`.

<details>
<summary><b>API</b> (click to expand)</summary>

Every `/api/servers/:id/…` route that covers a period accepts `?range=24h|7d|30d|90d` or `?from=<unix>&to=<unix>`.

| Route | Minimum role | Content |
|---|---|---|
| `GET /api/health` | public | Monitor health (Docker healthcheck) |
| `POST /api/login` · `POST /api/logout` · `GET /api/me` | public | Login, current account and role |
| `GET /api/servers` | visitor | Server list and latest status |
| `GET …/status` · `/history` · `/uptime` · `/maps` | visitor | Status, player chart, uptime, maps |
| `GET …/players` · `/player?key=` · `/progress?key=` | visitor | Player stats (no IPs for visitors), profile, progression |
| `GET …/combat` · `/rounds` · `/records` · `/badges` · `/duel` | visitor | Combat, rounds, records, badges, duel |
| `GET …/matches` · `/match?id=` · `/mapstats` · `/heatmap` | visitor | Matches, scoreboard, retention per map, busiest hours |
| `GET /api/events` | visitor | Live stream (SSE). Visitors receive neither console nor chat. |
| `GET …/mapvotes` | visitor | Map votes (mod) |
| `GET …/maprank` · `/week` | visitor | Maps compared (Maps tab), last 7 days (home) |
| `GET …/maprounds` · `/retention` | visitor | Rounds per map (length, attack / defense), player retention |
| `GET …/pings` | account | Players' ping |
| `GET …/logs` · `/chat` · `/aliases` · `/alts` · `/notes` · `/errors` · `/cvars` · `/evasion` · `/reports` | semi-admin | Console, chat, names, IPs, notes, errors, variables, ban evasion |
| `POST …/rcon` · `/kick` · `/ban` · `/unban` · `PUT …/notes` · `PUT …/welcome` · `…/schedules` · `POST …/reports/:id/done` | admin | Actions (recorded in the action log) |
| `GET …/bans` · `/dvar` · `/catalog` · `/rcon/log` · `/api/audit` · `/api/users` | admin | Bans, rcon variables, server maps, history, accounts |
| `GET /metrics` | semi-admin | Prometheus format |

</details>

---

## 13. Game server on the same machine and advanced settings

By default the monitor reads a server's logs over **FTP**. If the game server runs on the **same machine** as the monitor, the monitor can read them straight from the disk: it is faster, and there is no FTP to set up. It does not matter how the server is installed (Docker, a service, by hand).

### Logs on the same machine

1. In `.env`, set the game server's folder on the machine: the whole server, or only the mod folder.
   ```ini
   LOCAL_LOGS_DIR=/opt/cod4-server/cod4
   ```
2. Add `-f docker-compose.local-logs.yml` to all your `docker compose` commands, including in `deploy.sh` if you use automatic deploys:
   ```sh
   docker compose -p cod4-monitor -f docker-compose.prod.yml -f docker-compose.local-logs.yml up -d --build
   ```
   This file mounts `LOCAL_LOGS_DIR` **read-only** on `/gamelogs/` in the monitor.
3. In the dashboard: **More → All servers** → the server's settings → **Reading the logs: On this machine**, with the folder as the monitor sees it. For example `/gamelogs/mods/my_mod/` if you mounted the whole server.
4. Click **Test the connection**. If the folder is wrong, the test suggests the ones that exist.

The monitor cannot read anything outside `/gamelogs/`.

*Without Docker*: `LOCAL_LOGS_ROOT` sets the folder the monitor may read from (default `/gamelogs/`).

### Advanced settings (per server)

In a server's form, the **Advanced settings** block is for servers that don't use the usual names:

| Setting | Default | What it is for |
|---|---|---|
| Game log | `games_mp.log` | Chat, kills, rounds (the server's `g_log`) |
| Console | `qconsole.log` | Live console, cause of crashes (the monitor also reads `<console>.old`) |
| Connections | `enterleave.log` | Joins and leaves with IP and GUID |
| Time zone | `SERVER_LOG_TZ` from `.env` | The game server's clock, if it differs from the other servers |

A **name** is looked up in the log folder. It can include a sub-folder, for example `logs/games.log`. A **path starting with `/`** is used as is: a full path on the FTP server, or inside `/gamelogs/` for a server on the same machine.

---

## 14. B3: penalties, admins and grades from the dashboard

If the game server has a **B3** admin bot, the dashboard reads and changes its database: B3's **SQLite** file (`b3.db`, on the monitor's machine, for example with the `cod4-server` project) or its **MySQL / MariaDB** database (B3's `storage: mysql://…`, anywhere the monitor can reach):

- **Moderation → B3 → Penalties**: bans, temporary bans, kicks and warnings, with the player, the reason, who gave it (an in-game admin, `B3` or `site: <account>`) and when it ends. Filter, search, lift a ban or remove a warning.
- **Moderation → B3 → Admins**: who is admin and at which grade (20 moderator, 40 admin, 60 full admin, 80 senior admin, 100 super admin), with the penalties each gave. The grade can be changed in the list.
- **Moderation → B3 → Stats**: penalties per week, most frequent reasons, most penalized players, most active admins (90 days).
- **Player profile → Moderation → B3**: their grade, their penalties, and the **Warn**, **Kick**, **Temp ban**, **Ban**, **Lift the ban** and **B3 grade** controls.
- **Moderation → B3 → Search**: search by name (current or past), IP (or its start, e.g. `82.64.`) or GUID, like `!lookup`: grade, connections, first and last visit, IP, banned or not.
- **Moderation → Ban evasions**: players not banned who came, after a ban in force, from an IP of the banned player (often the same person with another GUID), with a button to ban them too.
- **Origin of the penalties**: automatic (B3's plugins: insults, spam, too many warnings), by an admin in game, or from the site, with B3's keyword (`#cuss`, `#spam`…).
- **Player profile**: also the names and IPs B3 knows, the accounts sharing one of their IPs (banned ones flagged), their masked grade (`!mask`) and their B3 welcome message.
- **Alerts**: each new ban or temporary ban (in game or from the site) shows up in the moderation inbox, as a push notification, and as a mail if mails are set up.

How the actions apply:

| Action | Effect |
|---|---|
| Warn | Stored in B3 (counted by B3 for its automatic kicks / bans) + private message to the player if in game |
| Kick | Kicked at once (rcon) + kept in B3's history |
| Ban / temporary ban | Stored in B3: the player is kicked at once if in game, and B3 refuses them at every connection |
| Lift the ban | The player's B3 bans are lifted, and the server ban too (`unban`) |
| Grade | Written in B3; applies at the player's **next connection** |

Only the owner of the server (and the `.env` account) can act. Moderators with the private data see the penalties but cannot change them.

### Setup with MySQL / MariaDB (any server owner)

1. Find B3's database in its `b3.ini`: `storage: mysql://user:password@host/database`.
2. Make sure the monitor can reach it: MySQL must listen on an address the monitor's machine can reach (not only `127.0.0.1`), and its firewall must let the monitor in. A dedicated MySQL account for the monitor is best, for example:
   ```sql
   CREATE USER 'monitor'@'<monitor IP>' IDENTIFIED BY '<password>';
   GRANT SELECT, INSERT, UPDATE ON b3.* TO 'monitor'@'<monitor IP>';
   ```
   (`SELECT` alone is enough to see everything; `INSERT` and `UPDATE` are needed for the actions: penalties, lifting a ban, grades.)
3. In the dashboard: **More → All servers → the server's settings → B3 → MySQL / MariaDB**: host, port, database, user, password, encrypted connection if the server supports it. **Test** shows how many admins and penalties were found.

*MariaDB next to B3 on the same machine* (the `cod4-server` project's `docker-compose.mariadb.yml`): with `docker-compose.b3.yml`, the monitor sees B3's data folder, where MariaDB puts its socket. In the host field, put the socket's path, `/b3/mysqld/mysqld.sock`: no network port is opened (only the `.env` account can set it).

The password is stored encrypted, like the rcon password, and is never sent back to the browser. MySQL 5.7+, MySQL 8 and MariaDB 10.2+ are supported. For security, an owner who is not the `.env` account cannot point the monitor to a local or private address (`127.0.0.1`, `10.x`, `192.168.x`…): only the `.env` account can, for a B3 on the monitor's own machine or network.

### Setup with B3's SQLite file (the `.env` account)

1. In the dashboard's `.env`, set B3's folder that holds `b3.db`:
   ```ini
   B3_DATA_DIR=/opt/cod4-server/b3data
   ```
2. Add `-f docker-compose.b3.yml` to all your `docker compose` commands and to `deploy.sh` (next to `docker-compose.local-logs.yml` if you use it):
   ```sh
   sed -i 's|-f docker-compose.local-logs.yml|-f docker-compose.local-logs.yml -f docker-compose.b3.yml|g' deploy.sh
   ./deploy.sh
   ```
   This file mounts B3's folder read-write on `/b3/`. The monitor can only open a B3 database inside that folder.
3. In the dashboard: **More → All servers → the server's settings → B3 → SQLite file on this machine**: `/b3/b3.db`. **Test** shows how many admins and penalties were found.

*Without Docker*: `B3_ROOT` sets the folder the monitor may open B3 databases from (default `/b3/`).

*Developers*: `B3_TEST_MYSQL=mysql://user:pass@127.0.0.1:3306/db go test ./internal/b3/` also runs the B3 tests on MySQL (the CI does it with MySQL 8). The database is emptied.


## Implemented roadmap, security and restore guide (French)

Voir [docs/ROADMAP.md](docs/ROADMAP.md) pour les fonctionnalités, les tests et les instructions de sauvegarde de la clé de chiffrement.

### Server ownership and invitations

Administrators and moderators join by invitation. See [roles, invitations and per-server permissions](docs/ACCOUNTS.md).

## License

Copyright © 2026 Florian Martinez.

CoD4 Monitor is free software, under the [GNU Affero General Public License v3.0](LICENSE) (AGPL-3.0): you may use, change and share it, as long as you keep the author's copyright notice and publish your changes under the same license. That includes a changed version you run as a website for others: its users must be able to get its source code.

The map pictures (`ui/public/maps/`) belong to Activision and to the map authors; they are not covered by this license.
