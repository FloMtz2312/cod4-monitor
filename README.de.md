# CoD4 Monitor

🇬🇧 [English](README.md) · 🇫🇷 [Français](README.fr.md) · 🇩🇪 **Deutsch**

Ein Web-Dashboard, mit dem du einen **Call of Duty 4 / CoD4X**-Server im Blick behältst – wer spielt, wann, auf welcher Map, mit welchen Stats – und ihn vom Browser oder Handy aus verwaltest.

- **Live**: Serverstatus, Spieler online, Map, Konsole und Chat.
- **Stats**: Aktivität, Ranglisten, Abzeichen, Rekorde, Match-Verlauf, ein Profil für jeden Spieler mit seiner Entwicklung.
- **Admin**: Map-Wechsel, Kick und Ban, geplante Befehle, Aktivitätsprotokoll, Erkennung von Ban-Umgehungen.
- **Zugriff**: Super-Admin, Serverbesitzer, Moderatoren mit Rechten pro Server und Besucher; Konten nur per Einladung. Auf Englisch, Französisch und Deutsch, auf dem Handy als App nutzbar (Benachrichtigungen und E-Mails auf Englisch oder Französisch).

Einen Fehler gefunden oder eine Idee? [Eröffne ein Issue](https://github.com/FloMtz2312/cod4-monitor/issues/new/choose): Das Formular führt dich durch.

---

## Inhalt

1. [Bevor du loslegst](#1-bevor-du-loslegst)
2. [Installation auf einem VPS](#2-installation-auf-einem-vps-empfohlen) (empfohlen)
3. [Installation auf deinem PC](#3-installation-auf-deinem-pc-zum-ausprobieren) (zum Ausprobieren)
4. [Aktualisieren](#4-aktualisieren)
5. [Konfiguration (`.env`)](#5-konfiguration-env)
6. [Konten und Rollen](#6-konten-und-rollen)
7. [Was das Dashboard bietet](#7-was-das-dashboard-bietet)
8. [FTP-Logs und RCON-Befehle](#8-ftp-logs-und-rcon-befehle)
9. [Backups](#9-backups)
10. [Fehlerbehebung](#10-fehlerbehebung)
11. [Befehlsübersicht](#11-befehlsübersicht)
12. [Für Entwickler](#12-für-entwickler)
13. [Spielserver auf derselben Maschine und erweiterte Einstellungen](#13-spielserver-auf-derselben-maschine-und-erweiterte-einstellungen)
14. [B3: Strafen, Admins und Ränge über das Dashboard](#14-b3-strafen-admins-und-ränge-über-das-dashboard)
15. [Server-Demos](#15-server-demos)
16. [Fehler melden / Mitwirken](#16-fehler-melden--mitwirken)
- [Lizenz](#lizenz)

---

## 1. Bevor du loslegst

Sammle diese Informationen. Nur die ersten beiden Punkte sind Pflicht.

| Was | Beispiel | Wofür |
|---|---|---|
| **IP und Port des CoD4-Servers** | `203.0.113.10:55555` | Abfragen des Servers (Pflicht) |
| **Ein starkes Passwort** für das Dashboard | — | Schutz der Seite (Pflicht auf einem VPS) |
| Das `rcon_password` des Servers | in seiner `server.cfg` | Befehle von der Seite aus senden (Map, Kick, Ban…) |
| FTP-Zugang zum Mod-Ordner | `ftp://user:pass@ip:21/mods/my_mod/` | Konsole, Chat, Kills, Verbindungen mit IPs |

Um die Seite online zu stellen, brauchst du außerdem:

- **einen VPS** mit Ubuntu 24.04 oder Debian 12 und 1 bis 2 GB RAM (OVH, Hetzner, Contabo…);
- **eine Webadresse**. Eine kostenlose bekommst du bei [DuckDNS](https://www.duckdns.org), zum Beispiel `my-server.duckdns.org`.

> 💡 Du kannst alles vom Handy aus erledigen: Nutze die App **Termius** (iPhone / Android), um dich mit dem VPS zu verbinden, und den Browser für GitHub und DuckDNS.

---

## 2. Installation auf einem VPS (empfohlen)

Die Seite läuft in Docker hinter **Caddy**, das das HTTPS-Zertifikat selbst besorgt und erneuert.

### Schritt 1: deine Adresse auf den VPS zeigen lassen

Melde dich auf [duckdns.org](https://www.duckdns.org) an, lege eine Subdomain an (z. B. `my-server`) und trag die **IP deines VPS** in das Feld *current ip* ein. Bestätige mit *update ip*.

### Schritt 2: mit dem VPS verbinden

Aus einem Terminal (oder Termius, oder der Web-Konsole deines Hosters):

```sh
ssh ubuntu@VPS_IP
```

Ersetze `ubuntu` durch den Benutzer, den dir dein Hoster gegeben hat (manchmal `root`).

### Schritt 3: das Projekt holen

Installiere git und klone dann das Projekt nach `/opt/cod4-monitor`:

```sh
sudo apt update && sudo apt install -y git
sudo mkdir -p /opt/cod4-monitor && sudo chown $USER /opt/cod4-monitor
git clone https://github.com/FloMtz2312/cod4-monitor.git /opt/cod4-monitor
cd /opt/cod4-monitor
```

### Schritt 4: den VPS vorbereiten

Ein einziger Befehl installiert Docker, schaltet die Firewall ein (nur SSH, HTTP und HTTPS bleiben offen) und aktiviert automatische Sicherheitsupdates:

```sh
sudo sh deploy/setup-vps.sh
sudo usermod -aG docker $USER
```

**Melde dich vom VPS ab und wieder an**, damit du `docker` ohne `sudo` nutzen kannst.

> ⚠️ Wenn dein Hoster auch eine Firewall in seinem Kundenbereich hat, öffne dort ebenfalls die Ports **80** und **443**.

### Schritt 5: konfigurieren

```sh
cd /opt/cod4-monitor
cp .env.example .env
nano .env
```

Fülle mindestens diese Zeilen aus:

```ini
SERVERS=My Server|203.0.113.10:55555
DASHBOARD_USER=admin
DASHBOARD_PASSWORD=a-long-and-unique-password
DOMAIN=my-server.duckdns.org
```

Speichere mit **Strg+O**, dann **Enter**, und beende mit **Strg+X**.

Für Konsole, Chat, Befehle und alle Optionen siehe [Konfiguration](#5-konfiguration-env) und [FTP-Logs und RCON-Befehle](#8-ftp-logs-und-rcon-befehle).

### Schritt 6: starten

```sh
mkdir -p backups && sudo chown 1000 backups && chmod 600 .env
docker compose -p cod4-monitor -f docker-compose.prod.yml up -d --build
```

Der erste Build dauert ein paar Minuten. Prüfe danach, ob alles läuft:

```sh
docker compose -p cod4-monitor -f docker-compose.prod.yml ps
```

Beide Zeilen (`monitor` und `caddy`) sollten **Up** anzeigen. Wenn bei `monitor` *Restarting* steht, sieh unter [Fehlerbehebung](#10-fehlerbehebung) nach.

### Schritt 7: die Seite öffnen

Ruf `https://my-server.duckdns.org` auf. Beim ersten Mal kann es eine Minute dauern, bis das Zertifikat da ist.

- **Ohne Anmeldung** siehst du die Seite so wie ein Besucher: die öffentlichen Stats.
- **Melde dich an** (oben rechts) mit `DASHBOARD_USER` und `DASHBOARD_PASSWORD`: Das ist das Super-Admin-Konto mit vollem Zugriff.
- Um jemand anderem Zugriff zu geben: **Mehr → Team und Einladungen** (ein Einladungslink, siehe [Konten und Rollen](#6-konten-und-rollen)).

📱 **Auf dem Handy**: Öffne die Seite und tippe dann auf *Teilen* → **Zum Home-Bildschirm** (iPhone) oder *Menü* → **Zum Startbildschirm hinzufügen** (Android). Die Seite öffnet sich dann wie eine App.

> ℹ️ **Läuft schon ein Monitor auf deinem PC?** Stopp ihn (`docker compose down`). Zwei Monitore für denselben Server würden beide die geplanten Befehle senden.

---

## 3. Installation auf deinem PC (zum Ausprobieren)

Du brauchst [Docker Desktop](https://www.docker.com/products/docker-desktop/). Im Projektordner:

```sh
cp .env.example .env      # then edit at least SERVERS, DASHBOARD_USER and DASHBOARD_PASSWORD
docker compose up -d
```

Öffne dann http://localhost:8080 (oder den Port aus `HTTP_PORT`).

- **Mit Docker sind `DASHBOARD_USER` und `DASHBOARD_PASSWORD` Pflicht, auch auf einem PC.** Mit leerem Passwort lauscht der Monitor in seinem Container nur auf `127.0.0.1`, sodass der von Docker veröffentlichte Port ihn nicht erreicht.
- Ein leeres Passwort funktioniert nur, wenn du die Binärdatei direkt startest (`go run ./cmd/monitor`, siehe [Für Entwickler](#12-für-entwickler)): Es wird keine Anmeldung verlangt, und die Seite samt Befehlen funktioniert nur vom PC selbst aus (`localhost`).
- Um die Seite von einem Handy **im selben WLAN** zu öffnen, ruf `http://PC_IP:8080` auf. Unter Windows gibst du den Port einmalig in der Firewall frei (PowerShell als Administrator):
  ```powershell
  New-NetFirewallRule -DisplayName "CoD4 Monitor (LAN)" -Direction Inbound -Protocol TCP -LocalPort 8080 -Action Allow -Profile Private
  ```
  Der Datenverkehr ist nicht verschlüsselt, also bleib damit in deinem lokalen Netzwerk. Für Zugriff aus dem Internet nimm die VPS-Installation.

Damit der Monitor mit Windows startet, aktiviere in den Einstellungen von Docker Desktop *Start Docker Desktop when you sign in*.

---

## 4. Aktualisieren

Auf dem VPS:

```sh
cd /opt/cod4-monitor && git pull && docker compose -p cod4-monitor -f docker-compose.prod.yml up -d --build
```

Deine `.env`, die Datenbank und die Backups bleiben erhalten. Die automatischen Tests laufen während des Builds. Schlagen sie fehl, bricht der Build ab und **die vorherige Version bleibt online**.

Wenn du zusätzliche Compose-Dateien nutzt (`docker-compose.local-logs.yml`, `docker-compose.b3.yml`), häng ihre `-f …` ebenfalls an diesen Befehl an.

**Option: vom PC aus deployen.** `sh deploy/push.sh ubuntu@VPS_IP` lädt deinen lokalen Ordner (inklusive `.env`) und deine lokale Datenbank hoch (nur wenn der VPS noch keine hat) und startet dann die Seite. ⚠️ Dieses Skript **ersetzt die `.env` des VPS** durch die von deinem PC.

### Automatisches Deployment von GitHub

Optional, für alle, die ihren eigenen Fork betreiben: Jeder Push auf `main` aktualisiert den VPS von selbst.

1. **Forke** das Repository auf GitHub. Auf dem VPS muss das Projekt aus deinem Fork stammen: Klone die URL deines Forks in [Schritt 3](#schritt-3-das-projekt-holen) oder führe `git remote set-url origin https://github.com/YOUR_NAME/cod4-monitor.git` aus.
2. Kopiere auf dem VPS das Deployment-Skript:
   ```sh
   cd /opt/cod4-monitor
   cp deploy/deploy.sh /opt/cod4-monitor/deploy.sh && chmod +x /opt/cod4-monitor/deploy.sh
   ```
   Liegt das Projekt nicht in `/opt/cod4-monitor`, ändere die `cd`-Zeile der Kopie und nimm unten deinen Pfad. Wenn du zusätzliche Compose-Dateien nutzt, häng ihre `-f …` an die `docker compose`-Zeilen der Kopie an.
3. Erstelle einen Schlüssel, der nur fürs Deployment dient:
   ```sh
   ssh-keygen -t ed25519 -N "" -C github-deploy -f ~/.ssh/github_deploy
   ```
4. Erlaube diesen Schlüssel, aber nur zum Ausführen von `deploy.sh`:
   ```sh
   echo "command=\"/opt/cod4-monitor/deploy.sh\",no-port-forwarding,no-agent-forwarding,no-X11-forwarding,no-pty $(cat ~/.ssh/github_deploy.pub)" >> ~/.ssh/authorized_keys
   ```
5. In deinem Fork: **Settings → Secrets and variables → Actions → New repository secret**. Lege vier Secrets an:

   | Secret | Wert |
   |---|---|
   | `VPS_HOST` | Die IP des VPS |
   | `VPS_USER` | Dein Benutzer auf dem VPS (die Ausgabe von `whoami`) |
   | `VPS_SSH_KEY` | Der Inhalt von `~/.ssh/github_deploy` (der private Schlüssel: `cat ~/.ssh/github_deploy`) |
   | `VPS_KNOWN_HOST` | Die Ausgabe von `ssh-keyscan -t ed25519 VPS_IP` |

   Bei einem Fork schaltet GitHub die Workflows zunächst ab: Öffne den Tab **Actions** deines Forks und aktiviere sie.

Jeder Push auf `main` führt dann die Prüfungen (`go vet`, Tests) auf GitHub aus, verbindet sich mit dem VPS und startet `deploy.sh`: Es folgt `main` exakt, baut den Monitor neu und wartet bis zu 2 Minuten, bis er gesund ist. Von Hand vorgenommene Änderungen an versionierten Dateien auf dem VPS gehen verloren; `.env`, die Datenbank und die Backups bleiben erhalten. Zum manuellen Deployen: **Actions → „Déploiement VPS“ → Run workflow**. Ohne die `VPS_*`-Secrets tut der Workflow nichts.

---

## 5. Konfiguration (`.env`)

Nach jeder Änderung an der `.env` startest du neu, mit dem Startbefehl aus [Schritt 6](#schritt-6-starten). Die Kommentare in `.env.example` sind auf Englisch. Dieser Abschnitt beschreibt jede Variable.

### Das Wichtigste

| Variable | Beispiel | Zweck |
|---|---|---|
| `SERVERS` | `My Server\|203.0.113.10:55555` | Der oder die zu verfolgenden Server beim **ersten Start** (Details unten). Danach werden sie über die Seite verwaltet |
| `DASHBOARD_USER` | `admin` | Benutzername des **Super-Admins**. Zusammen mit einem Passwort **Pflicht**: Ohne ihn startet der Monitor nicht |
| `DASHBOARD_PASSWORD` | — | Passwort des Super-Admins. **Pflicht mit Docker.** Leer = gar keine Anmeldung, der Monitor lauscht nur auf `127.0.0.1`: nur, wenn du die Binärdatei direkt auf deinem PC startest |
| `DOMAIN` | `my-server.duckdns.org` | Öffentliche Adresse (nur VPS) |
| `TZ` | `Europe/Paris` | Zeitzone für die Anzeige |

### Das `SERVERS`-Format

> **`SERVERS` wird nur beim ersten Start gelesen**: Die Server werden dann in die Datenbank kopiert. Danach fügst du sie unter **Mehr → Alle Server** hinzu, bearbeitest, testest, stoppst oder reaktivierst sie (Einstellungen jedes Servers oder **+ Server hinzufügen**: Adresse, FTP, rcon), ohne den Monitor neu zu starten. Sobald die Server in der Datenbank sind, hat eine Änderung an `SERVERS` keine Wirkung mehr. Du kannst es sogar leer lassen und deinen ersten Server über die Seite hinzufügen.

Jeder Server besteht aus bis zu 4 Feldern, getrennt durch `|`. Nur die ersten beiden sind Pflicht:

```
Name|ip:port|ftp_url_of_the_mod_folder|rcon_password
```

Vollständiges Beispiel:

```ini
SERVERS=My Server|203.0.113.10:55555|ftp://user:password@203.0.113.10:21/mods/my_mod/|myrcon
```

- **Mehrere Server**: Trenne sie mit `;`. Auf der Seite erscheint dann eine Auswahlliste.
- **Sonderzeichen** im FTP-Passwort müssen ersetzt werden: `@` → `%40`, `:` → `%3A`, `/` → `%2F`, `#` → `%23`, `%` → `%25`.

### Weitere Einstellungen (die Standardwerte passen)

| Variable | Standard | Zweck |
|---|---|---|
| `POLL_INTERVAL` | `15s` (`5s` in `.env.example`) | Wie oft der Server abgefragt wird (mindestens 5 s) |
| `QUERY_TIMEOUT` | `2s` | Wie lange auf eine Antwort gewartet wird |
| `OFFLINE_AFTER` | `45s` | Zeit ohne Antwort, bevor „offline“ angezeigt wird (bei einem Map-Wechsel antwortet der Server 20–40 s lang nicht) |
| `LOG_POLL_INTERVAL` | `2s` | Wie oft die Logs per FTP gelesen werden |
| `LOG_FTP_SESSION` | `60s` | Die FTP-Verbindung wird nach dieser Zeit erneuert: Manche FTP-Server zeigen eine Datei so, wie sie beim Öffnen der Verbindung war; der Chat kam dann erst an, wenn der Server die Verbindung trennte. `0` = nie |
| `HISTORY_FTP_MAX_MB` | `50` | Maximale Größe der `games_mp.log`, deren Vergangenheit beim ersten Start per FTP importiert wird (sie wird zweimal heruntergeladen). `0` = nie per FTP |
| `IMPORT_MAX_MB` | `2048` | Maximale Größe einer alten Log-Datei, die über den Import der Server-Einstellungen geschickt wird |
| `SERVER_LOG_TZ` | `UTC` | Zeitzone der Uhr des Spielservers |
| `HTTP_PORT` | `8080` | Port, auf dem der Monitor lauscht (auf einem VPS leitet Caddy dorthin weiter) |
| `HTTP_BIND` | leer (alle Adressen) | Adresse, auf der der Monitor lauscht. Wird auf `127.0.0.1` gesetzt, wenn `DASHBOARD_PASSWORD` leer ist |
| `TRUSTED_PROXIES` | `127.0.0.1/32,::1/128` | Reverse-Proxys, die die IP des Besuchers weitergeben dürfen (`X-Forwarded-For`). `docker-compose.prod.yml` setzt die Adresse von Caddy selbst. `none` = diese Header ignorieren |
| `DB_PATH` | `monitor.db` (`/data/monitor.db` in `.env.example`) | Die Datenbank. Mit Docker liegt sie im Volume `data` |
| `MONITOR_SECRET_KEY` | — | Schlüssel, der die gespeicherten FTP-, rcon- und B3-Passwörter verschlüsselt (Base64 von 32 Bytes, z. B. `openssl rand -base64 32`). Leer = die Datei `DB_PATH.key` wird neben der Datenbank angelegt. Siehe [Backups](#9-backups) |
| `RETENTION_DAYS` | `90` | Wie viele Tage die Abfragen aufbewahrt werden (`0` = für immer) |
| `BACKUP_DIR` | leer (`/backups` in `.env.example`) | Ordner der täglichen Backups (`./backups` des Projekts mit Docker). Leer = keine Backups |
| `BACKUP_KEEP` | `14` | Anzahl der aufbewahrten täglichen Backups |
| `DISCORD_WEBHOOK_URL` | — | Discord-Benachrichtigungen, wenn der Server ausfällt oder wieder da ist |
| `DISCORD_PLAYER_THRESHOLD` | `0` | Discord-Benachrichtigung, wenn die Spielerzahl diesen Wert erreicht (`0` = aus) |

### E-Mails (optional)

Der Monitor kann dir (dem Admin) E-Mails schicken. Trag ein E-Mail-Konto ein, von dem er sendet, und die Empfängeradresse:

| Variable | Beispiel | Zweck |
|---|---|---|
| `SMTP_HOST` | `smtp.gmail.com` | Mailserver des sendenden Kontos. **Leer = keine E-Mails.** |
| `SMTP_PORT` | `587` | `587` (STARTTLS) oder `465` (TLS) |
| `SMTP_USER` / `SMTP_PASSWORD` | `my.monitor@gmail.com` | Zugangsdaten des sendenden Kontos |
| `MAIL_FROM` | — | Absenderadresse (Standard: `SMTP_USER`) |
| `MAIL_TO` | `me@outlook.com` | Wer die E-Mails bekommt (mehrere: mit `,` trennen) |
| `MAIL_LANG` | `fr` | `fr` oder `en` |
| `WEEKLY_MAIL` | — | `0` = keine wöchentliche Zusammenfassung |
| `VPS_DISK_ALERT` / `VPS_MEM_ALERT` | `90` | Alarmschwelle für Festplatte / Arbeitsspeicher, in % |
| `VPS_LOAD_ALERT` | `2` | CPU-Alarm: 15-Minuten-Last pro Kern |

**Mit Gmail** (am einfachsten): Aktiviere die Bestätigung in zwei Schritten für das Konto, erstelle ein *App-Passwort* (Google-Konto → Sicherheit → App-Passwörter) und trag es in `SMTP_PASSWORD` ein. Ein kostenloses [Brevo](https://www.brevo.com)-Konto funktioniert auch (`smtp-relay.brevo.com`, Port 587).

#### Mit Resend (eine Adresse auf deiner eigenen Domain)

[Resend](https://resend.com) verschickt die E-Mails von einer Adresse auf deiner Domain (z. B. `monitor@my-domain.com`), und sie landen selten im Spam. Der kostenlose Tarif reicht locker: 3.000 E-Mails im Monat, 100 am Tag.

Du brauchst **eine eigene Domain** (ein paar Euro im Jahr bei OVH, Gandi, Cloudflare…): Resend verlangt, dass du DNS-Einträge hinzufügst, was mit einer DuckDNS-Adresse nicht geht.

1. Erstelle ein Konto auf [resend.com](https://resend.com).
2. **Domains → Add Domain**: Gib deine Domain ein. Mit einer Subdomain (z. B. `mail.my-domain.com`) bleibt deine Hauptdomain außen vor.
3. Resend zeigt DNS-Einträge an (einen `MX` und einen `TXT` für SPF, einen `TXT` für DKIM). Füge sie unverändert in der DNS-Zone deiner Domain bei deinem Registrar hinzu und klicke dann auf **Verify DNS Records**. Die Domain wird innerhalb von Minuten, manchmal ein paar Stunden, *Verified*.
4. **API Keys → Create API Key**: Name `cod4-monitor`, Berechtigung **Sending access**, auf deine Domain beschränkt. Kopiere den Schlüssel (er beginnt mit `re_`): Er wird nur einmal angezeigt.
5. In der `.env` des Monitors:
   ```sh
   SMTP_HOST=smtp.resend.com
   SMTP_PORT=465
   SMTP_USER=resend
   SMTP_PASSWORD=re_your_api_key
   MAIL_FROM=monitor@my-domain.com
   MAIL_TO=me@example.com
   ```
   `SMTP_USER` ist immer das Wort `resend`. `MAIL_FROM` ist Pflicht: eine Adresse auf deiner verifizierten Domain (es muss kein echtes Postfach dahinter geben). Port `587` funktioniert auch.
6. Starte den Monitor neu (`docker compose … up -d`). Beim Start wird eine Test-E-Mail verschickt; du siehst sie auch unter **Emails** auf der Seite von Resend. Wenn sie nicht ankommt: `docker logs cod4-monitor-monitor-1 | grep mail`.

Für einen schnellen Test ohne Domain akzeptiert Resend `MAIL_FROM=onboarding@resend.dev`, aber nur an die Adresse deines Resend-Kontos.

Was du bekommst:

- **Eine Test-E-Mail**, wenn der Monitor mit neuen E-Mail-Einstellungen startet: Kommt sie nicht an, schau in `docker logs cod4-monitor-monitor-1 | grep mail`.
- **VPS-Alarme**: Festplatte fast voll, Arbeitsspeicher oder CPU länger als 5 Minuten ausgelastet, Monitor in einer Neustart-Schleife. Danach eine Erinnerung alle 24 h, solange es anhält, und eine E-Mail, wenn es vorbei ist.
- **Wöchentliche Zusammenfassung**, montags ab 9:00 Uhr: Spieler, Spielzeit, Spitzenwert, Matches, Spieler der Woche, Rekorde, Top 5, meistbesuchte Map.
- **Serverabsturz**: wenn der Spielserver nicht mehr antwortet, von selbst neu startet (Absturz + automatischer Neustart) oder wieder da ist, mit der wahrscheinlichen Ursache (letzter Fehler seiner Konsole, z. B. `ERROR: Exceeded limit of 1000 'xmodel' assets`) und den letzten Konsolenzeilen. Nach einem Neustart wird das Ende des vorherigen Laufs aus `qconsole.log.old` gelesen (dafür braucht es die FTP-Logs). Ein normales Herunterfahren (Beenden, Neustart durch den Hoster) wird nur protokolliert, nicht gemailt. Die Ursache erscheint auch unter jedem Ausfall auf der Startseite.
- **Spieler-Alarme**: wenn sich ein Spieler verbindet, der eine Admin-Notiz **⚠ Verdächtig** oder **👁 Beobachten** hat oder einen noch nicht bearbeiteten `!report` (letzte 30 Tage). Höchstens eine E-Mail pro Spieler alle 6 Stunden.

### Push-Benachrichtigungen (Handy)

Jedes Konto des Teams kann außerdem **Benachrichtigungen auf dem Handy bekommen, auch wenn die App geschlossen ist**: **Mehr → Benachrichtigungen** → *Benachrichtigungen aktivieren*, dann ein Schalter pro Alarm:

- **Server down oder wieder da** (Absturz, unerwarteter Neustart, wieder online, mit der wahrscheinlichen Ursache): jedes Konto mit Rechten auf dem Server.
- **Neue Meldung** (`!report` im Spiel): Konten, die Meldungen bearbeiten oder private Daten sehen dürfen.
- **Verdächtiger oder gebannter Spieler zurück** (auffällige Stats, im Spiel als verdächtig / zu beobachten markierter Spieler, mögliche Ban-Umgehung) und **B3-Ban**: Konten, die private Daten sehen.

Nichts zu konfigurieren: kein Drittanbieter-Konto, keine `.env`-Variable. Der Monitor erzeugt beim ersten Start seinen eigenen Schlüssel (VAPID) und speichert ihn in seiner Datenbank; der Push-Dienst des Handys (Google, Apple, Mozilla, Microsoft) stellt die Benachrichtigungen zu. Die Rechte werden bei jedem Versand geprüft, wer also ein Recht verliert, bekommt auch die zugehörigen Benachrichtigungen nicht mehr. **iPhone**: iOS 16.4 oder neuer, und nur aus der App, die zum Home-Bildschirm hinzugefügt wurde (Safari → Teilen → *Zum Home-Bildschirm*, dann von dort öffnen). Jedes Handy bzw. jeder Browser hat eigene Einstellungen und kann pro Server stummgeschaltet werden. An den E-Mails oben ändert sich nichts.

---

## 6. Konten und Rollen

| Rolle | Wer | Sieht | Darf |
|---|---|---|---|
| 👑 **Super-Admin** | das `.env`-Konto (`DASHBOARD_USER`) | alles, auf jedem Server | alles: jeder Server, lädt Administratoren ein, verwaltet die Konten, überträgt Server, die Maschine des Monitors (Logs und B3 auf dieser Maschine) |
| 🏠 **Admin** (Serverbesitzer) | vom Super-Admin eingeladen | alles auf den eigenen Servern; bei den anderen nur die öffentlichen Stats | fügt seine Server hinzu und richtet sie ein (Adresse, FTP, rcon, B3), lädt Moderatoren ein, jede Aktion auf seinen Servern |
| 🛡 **Moderator** | vom Besitzer auf einen Server eingeladen | was seine Rechte erlauben, Server für Server | was seine Rechte erlauben (siehe unten) |
| 👀 **Besucher** | jeder, ohne Konto | die öffentlichen Stats: Status, Spieler, Aktivität, Ranglisten, Matches | nichts |

Die Rechte eines Moderators werden pro Server gewählt: **private Daten sehen** (IP, Chat, Logs, Notizen), **kicken**, **bannen** und entbannen, **Meldungen bearbeiten**, **Spielernotizen bearbeiten**, **Map wechseln** / Spiel neu starten, **Nachricht senden**, **volle RCON-Konsole** und geplante Befehle (volle Kontrolle über das Spiel). Standardmäßig hat ein Moderator keines davon und sieht seine Rechte unter **Mehr → Meine Rechte auf … → Meine Berechtigungen**.

- **Konten entstehen nur per Einladung**: **Mehr → Team und Einladungen** → ein Einladungslink, 48 h gültig. Der Besitzer lädt Moderatoren für den Server ein; der Super-Admin kann außerdem Administratoren einladen. Der Super-Admin verwaltet alle Konten unter **Mehr → Dashboard-Konten** (neues Passwort, löschen).
- **Besucher sehen nie** IPs, die Konsole, den Chat oder Admin-Notizen. Der Server selbst sperrt diese Daten; sie sind nicht nur auf der Seite ausgeblendet.
- **Sicherheit**: Passwörter werden gehasht (PBKDF2-SHA256), eine Anmeldung gilt 7 Tage, und eine IP wird nach 10 Fehlversuchen für 15 Minuten gesperrt.

Details (Einladungen, Rechte, Übertragung eines Servers): [docs/ACCOUNTS.de.md](docs/ACCOUNTS.de.md).

---

## 7. Was das Dashboard bietet

Die Seite ist eine App, die fürs Handy gedacht ist (Tabs unten auf dem Handy, Seitenmenü auf dem Computer). Sie lässt sich auf dem Home-Bildschirm installieren (**Mehr → App installieren**). Die Tabs: **Live**, **Ranglisten**, **Spieler**, **Matches** (Besucher) oder **Moderation** (Team), **Mehr**.

| Ansicht | Inhalt |
|---|---|
| **Live** | Serverkarte (Status, Map, Live-Spielstand, beitreten / teilen), Latenz, Ping und Verfügbarkeit über 30 Tage, Serverdetails, Spieler online, heutige Aktivität, die letzten 7 Tage (Spieler der Woche, Top 5, Rekorde). Für das Team mit den passenden Rechten: Map wechseln (Suche, Bilder, Spielmodus), Neustart, Nachrichten, Kick / Ban, Map-Pool für die Abstimmung am Matchende, XP-Boost. |
| **Moderation** (Team) | Posteingang (Offen / Verlauf): Meldungen, verdächtige Stats, im Spiel markierte Spieler, mögliche Ban-Umgehungen, B3-Bans. Werkzeuge, je nach Rechten: Chat mit Suche, Konsole (Live-Spielkonsole), Bans (Server-Bans und Entbannen), Umgehungen (Hinweise auf Ban-Umgehung), Verknüpfte Konten (Namen pro GUID, über die IP verknüpfte Konten), B3 (Strafen, Admins und Ränge, Suche, Stats). |
| **Spieler** | Alle Spieler des Zeitraums (Filter: online, neu, mit Notiz), Suche nach aktuellem oder altem Namen (und IP für das Team). Auf dem Computer eine Tabelle mit Spielzeit, Sitzungen, Ping, bestem Score, Nachrichten… |
| **Spielerprofil** | Übersicht (Form im Vergleich zum vorherigen Zeitraum, Vergleich mit einem Spieler, Entwicklung von K/D / Headshots / Treffsicherheit), Kampf (Waffen, Trefferzonen, Erzfeinde), Matches, Moderation (Notiz, Willkommensnachricht, Sitzungen, IPs, verknüpfte Konten, Chat, B3-Rang und -Strafen). Buttons **Link kopieren**, **Duell** und **Discord-Karte** (ein PNG-Bild der letzten 7 Tage des Spielers, ohne IP oder GUID, zum Posten auf Discord). |
| **Ranglisten** | Abzeichen, Rekorde (Spieler des Tages und der Woche), Rangliste nach Kennzahl (vollständige sortierbare Tabelle auf dem Computer), Herausforderungen der Woche (Kills, Headshots, Ziele der ganzen Community; der Besitzer legt die Ziele fest), **Duell** (wer wen zwischen zwei Spielern getötet hat, seit jeher). |
| **Matches** (Besucher) | Matches und Runden: Scoreboards, Highlights, eine **Discord-Karte** (PNG) pro Match. Team-Konten finden sie unter Mehr → Letzte Matches. |
| **Mehr** | **Erweiterte Stats** (Besucherzahlen, Maps mit Urteil behalten / ersetzen und Map-Abstimmungen, Treue, Kampf), **Letzte Matches**, **Server-Status** (Verfügbarkeit, Ausfälle und ihre Ursache, Latenz, Ping der Spieler, Konsolenfehler, Servervariablen, Log-Erfassung und Backups), **Alle Server** (Einstellungen jedes Servers, **+ Server hinzufügen**). Dann, für den ausgewählten Server und je nach Rechten: Anpassung, Team und Einladungen, Server-Einstellungen, Konsole, Geplante Befehle, Dashboard-Konten, Monitor-Logs (Super-Admin: die letzten Zeilen von `docker logs`, Geheimnisse maskiert), Aktivitätsprotokoll. Dann Benachrichtigungen, App installieren, Passwort und **Über** (Bug melden oder Idee vorschlagen, Quellcode). |

Außerdem:

- **Auf dem Computer**: Seitenmenü, Suche mit **Strg K** oder **/** (Spieler nach aktuellem oder altem Namen, Ansichten).
- **Alte Links** funktionieren weiter: `/v2/…`, `/login.html`, `/invite.html#…`, `#accueil`, `#joueur/<key>`, `#admin`…
- **Sprache**: **Mehr → Sprache** (Französisch, Englisch, Deutsch). Standardmäßig nutzt die Seite die Sprache des Browsers.
- **Abzeichen**: der beste Spieler des Zeitraums in 7 Kategorien. Das Symbol erscheint überall auf der Seite neben seinem Namen.

---

## 8. FTP-Logs und RCON-Befehle

### FTP-Logs (Konsole, Chat, Kills, Verbindungen)

Mit der FTP-URL des Mod-Ordners als 3. Feld von `SERVERS` liest der Monitor drei Dateien:

| Datei | Was sie bringt |
|---|---|
| `qconsole.log` | Live-Konsole (es wird nichts gespeichert) |
| `games_mp.log` | Chat, Kills, Runden, Treffsicherheit, Verknüpfungen Name ↔ GUID. Beim ersten Start wird der Teil, der vor dem Monitor geschrieben wurde, einmalig im Hintergrund importiert (Spieler, Kills, Chat) und mithilfe von `enterleave.log` datiert: Dieses Log enthält nur die Zeit seit dem Serverstart, daher wird ein Serverlauf, den `enterleave.log` nicht abdeckt, übersprungen, und diese alten Kills haben keine Runde. |
| `enterleave.log` | Beitritte und Abgänge mit IP und GUID. Der gesamte Verlauf wird beim ersten Start importiert. |

Ohne FTP funktioniert die Seite trotzdem, aber nur mit den öffentlichen Infos des Servers: Spieler, Scores, Map.

**Alte Logs importieren**: In den Einstellungen eines Servers (sein Besitzer) schickst du eine alte `enterleave.log` oder `games_mp.log` (die Datei, die beiseitegelegt wurde, als der Server eine neue angefangen hat, ein Backup…). Der Monitor importiert sie im Hintergrund, ohne das Live-Lesen anzuhalten, und überspringt, was er schon hat: Eine Datei kann gefahrlos erneut geschickt werden. Schick zuerst die `enterleave.log`: Sie datiert die Zeilen der `games_mp.log` aus demselben Zeitraum. Bis zu `IMPORT_MAX_MB` (2048) pro Datei.

Die Ansicht **Spieler** listet die Verbindungen aus `enterleave.log` (mit Spielzeit), ergänzt um die Spieler, die nur in `games_mp.log` oder von B3 gesehen wurden (markiert mit „Im Spiel-Log gesehen“ / „Von B3 gesehen“, ohne Spielzeit).

### RCON-Befehle

Mit dem `rcon_password` als 4. Feld von `SERVERS` (oder in den Einstellungen des Servers) bekommen der Besitzer und die Moderatoren mit den passenden Rechten:

- **Map- und Moduswechsel** aus einer Galerie der Maps des Servers (Standard-Maps und `usermaps/`, per FTP gelesen);
- **nächste Map, Neustart, Nachricht an alle Spieler, freier Befehl**;
- **Kick und Ban** aus der Spielerliste oder dem Profil eines Spielers;
- **Maps für die Abstimmung am Matchende und XP-Boost**;
- **geplante Befehle**, alle N Minuten oder jeden Tag zu einer festen Uhrzeit.

Jede Aktion wird im **Aktivitätsprotokoll** festgehalten (Mehr → Aktivitätsprotokoll), zusammen mit dem Konto, das sie ausgeführt hat.

> Die Mod lässt sich nicht im laufenden Betrieb wechseln (CoD4X schützt `fs_game`). Sie wird in den Startparametern des Servers festgelegt (OGP).

### Erweiterte Stats mit der Mod `pm_sniper`

Die Mod `pm_sniper` (separates Repository) schreibt zusätzliche Zeilen in `games_mp.log`. Der Monitor liest sie automatisch, ohne Einstellung:

| Zeile | Was sie dem Dashboard bringt |
|---|---|
| `KE` | Distanz jedes Kills, Noscope, Quickscope, Wallbang, Punkte |
| `RE` / `CL` | Siegerteam und Spielstand jeder Runde, Clutches (der letzte Überlebende, der gewinnt) |
| `P_P` / `P_D` | Bombe gelegt / entschärft (gibt es schon in Promod) |
| `MS` / `ME` | Offizieller Endstand, Level und Prestige jedes Spielers, Matchsieger |
| `MV` / `MVW` | Ergebnisse der Map-Abstimmung am Matchende |
| `PI` | Spielerprofil beim Beitritt (Level, Prestige, Karriere-Stats): Erkennung von Ban-Umgehungen |

Sie erscheinen in der Rangliste (Noscopes, Quickscopes, Wallbangs, längster Kill, Clutches, First Blood, Sieg-%), bei Rekorden und Abzeichen, in den Matches (Sieger, Spielstand, Highlights jeder Runde), im Spielerprofil und unter **Mehr → Erweiterte Stats → Maps** (Map-Abstimmungen). Ohne diese Mod werden diese Spalten und Bereiche ausgeblendet, und alles andere funktioniert.

### Chat-Befehle im Spiel

Mit dem `rcon_password` in `SERVERS` antwortet der Monitor den Spielern per privater Nachricht (`tell`):

| Befehl | Antwort |
|---|---|
| `!stats` oder `!stats name` | Kills, Tode, K/D, Headshot-%, Noscopes, längster Kill |
| `!top` | Top 5 der Killer der letzten 7 Tage |
| `!report name reason` | Meldung an die Admins: Sie erscheint unter **Moderation** (und als Push-Benachrichtigung) |
| `!me` | Ein kurzer Link zum Eintippen auf dem Handy (`your-domain/me/K7QPX`, oder ein Code ohne `DOMAIN`): Er öffnet das Profil des Spielers und pinnt es an (**Das bin ich**). 24 Stunden gültig |
| `!help` | Liste der Befehle (auf einem Server mit B3 antwortet B3: Seine Liste enthält diese Befehle) |

Ein Befehl alle 3 Sekunden pro Spieler, eine Meldung pro Minute.

### Ban-Umgehung (GUID-Spoofer)

**Moderation → Umgehungen**: Jeder Spieler, der seit einem Ban aktiv war, wird mit dem gebannten Spieler verglichen, und jedes Indiz bringt Punkte. Ab **50** erscheint ein Live-Alarm. Es wird nichts automatisch gebannt: Die Prüfung liegt bei dir.

| Indiz | Punkte |
|---|---|
| Gleiche IP oder gleiche Steam-ID | 60 |
| Gleiches Karriereprofil: gleiches Prestige, Karriere-Kills knapp über denen des gebannten Spielers (das Profil bleibt auf dem PC des Spielers, auch mit neuer GUID) | 40 |
| Gleicher Name (ohne Farben oder l33t) | 25 |
| Gleiches Netzwerk (IP unterscheidet sich nur in der letzten Zahl) | 20 |
| Neue GUID, die schon mit einem großen Profil ankommt (Level 10+ oder ein Prestige) | 20 |
| Gleiche Netzwerkeinstellungen, wenn nicht die Standardwerte (`rate`, `snaps`, `cl_maxpackets`, per rcon `dumpuser` gelesen) | 15 |
| Name weicht um 1 oder 2 Buchstaben ab | 15 |
| Ähnlicher Spielstil (Headshot-%, Noscope-%, Kill-Distanz) | 10 |
| GUID erstmals nach dem Ban gesehen | 10 |

Schwache Indizien zählen nur zusätzlich zu einem Indiz, das den Spieler mit dem gebannten verbindet, und ein Spieler, der schon vor dem Ban bekannt war, wird nur bei einem starken Indiz markiert (IP, Steam-ID, Profil, Name). Das Profil stammt aus der `PI`-Zeile der Mod `pm_sniper`, für die Netzwerkeinstellungen braucht es das rcon-Passwort. Ein Cheater, der GUID, IP (VPN), CoD4-Profil, Namen und Spielstil alle auf einmal ändert, bleibt unentdeckbar.

**Willkommensnachricht**: Im Profil eines Spielers (Tab Moderation) kann der **Besitzer** des Servers (kein Moderator) eine persönliche Nachricht schreiben, die dem Spieler 30 Sekunden nach jedem Verbinden privat geschickt wird. `{name}` wird durch seinen Namen ersetzt.

**Eigene Map-Vorschaubilder**: Die Vorschaubilder liegen in `ui/public/maps/`. Nachdem du Maps zu deinem Spiel hinzugefügt hast, erzeugst du sie auf deinem PC neu:

```sh
docker run --rm -v "$PWD":/src -v "<cod4 folder>":/game:ro -w /src golang:1.26-alpine go run ./cmd/mapimages /game ui/public/maps
```

---

## 9. Backups

Jeden Tag wird eine Kopie der Datenbank nach `backups/monitor-YYYY-MM-DD.db` (`BACKUP_DIR`) geschrieben und anschließend durch eine Wiederherstellung geprüft. Die letzten 14 werden aufbewahrt (`BACKUP_KEEP`).

> ⚠️ Diese Backups liegen **auf dem VPS selbst**. Kopiere ab und zu eins an einen anderen Ort. Von deinem PC aus: `scp ubuntu@VPS_IP:/opt/cod4-monitor/backups/monitor-*.db .`

**Bewahr auch den Schlüssel auf.** Die gespeicherten FTP-, rcon- und B3-Passwörter sind mit einem Schlüssel verschlüsselt, der **nicht** in den Backups steckt: die Datei `monitor.db.key` neben der Datenbank (`DB_PATH` + `.key`, mit Docker im Volume `data`), oder `MONITOR_SECRET_KEY`, falls du ihn gesetzt hast. Eine ohne ihren Schlüssel wiederhergestellte Datenbank verliert diese Passwörter: Der Monitor verweigert den Start (`cannot decrypt server secrets`), sobald ein Server ein gespeichertes FTP- oder rcon-Passwort hat. Sichere den Schlüssel einmal, an einem anderen Ort als die Backups:

```sh
docker run --rm -v cod4-monitor_data:/data alpine cat /data/monitor.db.key > monitor.db.key && chmod 600 monitor.db.key
```

**Ein Backup wiederherstellen** (auf dem VPS):

```sh
cd /opt/cod4-monitor
docker compose -p cod4-monitor -f docker-compose.prod.yml stop monitor
docker run --rm -v cod4-monitor_data:/data -v "$PWD/backups":/b alpine \
  sh -c 'cp /b/monitor-2026-10-06.db /data/monitor.db && rm -f /data/monitor.db-wal /data/monitor.db-shm && chown -R 1000 /data'
docker compose -p cod4-monitor -f docker-compose.prod.yml start monitor
```

Ersetze das Datum durch das des gewünschten Backups. Auf demselben VPS liegt der Schlüssel noch im Volume. Auf einer neuen Maschine (oder einem neuen Volume) legst du den gesicherten Schlüssel vor `start monitor` zurück: Kopiere `monitor.db.key` nach `backups/`, führe den Befehl unten aus und lösche ihn dann wieder aus `backups/`.

```sh
docker run --rm -v cod4-monitor_data:/data -v "$PWD/backups":/b alpine \
  sh -c 'cp /b/monitor.db.key /data/monitor.db.key && chmod 600 /data/monitor.db.key && chown 1000 /data/monitor.db.key'
```

Für immer aufbewahrt: Chat, Namen, Kills, Runden, Verbindungen, Bans, Aktivitätsprotokoll. Abfragen und Sitzungen werden nach `RETENTION_DAYS` Tagen gelöscht.

---

## 10. Fehlerbehebung

| Problem | Lösung |
|---|---|
| `monitor` zeigt **Restarting** | Finde die Ursache mit `docker logs --tail 20 cod4-monitor-monitor-1` und sieh dann in den Zeilen unten nach. |
| Logs: `attempt to write a readonly database` | Der Datenordner hat den falschen Besitzer. Führe `docker run --rm -v cod4-monitor_data:/data alpine chown -R 1000:1000 /data` aus, dann `docker restart cod4-monitor-monitor-1`. |
| Logs: `invalid SERVERS entry` | Die `SERVERS`-Zeile ist fehlerhaft. Das Format ist `Name\|ip:port`, ohne Leerzeichen um das `\|`. |
| Logs: `invalid log URL` | Die FTP-URL muss mit `ftp://` beginnen, und Sonderzeichen im Passwort müssen kodiert werden (siehe [hier](#das-servers-format)). |
| Die Seite öffnet sich nicht / Zertifikatsfehler | Prüfe, ob DuckDNS auf die IP des VPS zeigt und ob die Ports 80 und 443 offen sind (Firewall des VPS **und** die deines Hosters). Siehe `docker logs cod4-monitor-caddy-1`. |
| Anmeldung mit dem `.env`-Konto klappt nicht | Ist `DASHBOARD_USER` in der `.env` gesetzt? Nach einer Änderung startest du die Seite neu. Nach 10 Fehlversuchen 15 Minuten warten. |
| „Logs nicht eingerichtet“ in der Konsole | Füge die FTP-URL als 3. Feld von `SERVERS` hinzu. |
| Keine Serverbefehle (Map, Nachrichten, Konsole) | Füge das rcon-Passwort hinzu (4. Feld von `SERVERS`, oder Mehr → Alle Server → die Einstellungen des Servers) und melde dich als **Besitzer** des Servers an, oder als Moderator mit dem Recht für Map, Nachrichten oder RCON. |
| Logs: `cannot decrypt server secrets` | Die Datenbank wurde ohne ihren Schlüssel wiederhergestellt. Leg die passende `monitor.db.key` (oder `MONITOR_SECRET_KEY`) zurück, siehe [Backups](#9-backups). |
| `fatal: not a git repository` | Du bist nicht im richtigen Ordner: `cd /opt/cod4-monitor`. |
| `permission denied` bei `docker` | Führe `sudo usermod -aG docker $USER` aus, melde dich dann ab und wieder an. |
| iPhone: Der obere Teil der Seite ist verdeckt | Aktualisiere die Seite, schließ die App dann komplett und öffne sie erneut. |

---

## 11. Befehlsübersicht

Führe diese auf dem VPS in `/opt/cod4-monitor` aus. Damit du weniger tippen musst, leg einmalig einen Alias an:

```sh
echo "alias cod4='docker compose -p cod4-monitor -f /opt/cod4-monitor/docker-compose.prod.yml'" >> ~/.bashrc && source ~/.bashrc
```

| Aktion | Befehl |
|---|---|
| Status der Container | `cod4 ps` |
| Live-Logs (Strg+C zum Beenden) | `cod4 logs -f monitor` |
| Neu starten | `cod4 restart` |
| Stoppen | `cod4 down` (die Datenbank bleibt erhalten) |
| Starten / geänderte `.env` übernehmen | `cod4 up -d` |
| Aktualisieren | `git pull && cod4 up -d --build` |

---

## 12. Für Entwickler

Der Monitor ist in Go geschrieben (Gin, SQLite ohne CGO). Die Oberfläche (`ui/`, Svelte) wird vom `Dockerfile` gebaut und in die Binärdatei eingebettet.

```sh
go test ./...                              # tests
go run ./cmd/probe 203.0.113.10:55555     # query a server once and print the reply
DB_PATH=monitor.db go run ./cmd/monitor    # run the monitor locally
```

- **Oberfläche**: eine Svelte-App in `ui/`, ausgeliefert im Wurzelverzeichnis der Seite. Um daran zu arbeiten, starte den Monitor (Port 8080) und dann:

  ```sh
  cd ui && npm ci
  npm run dev     # http://localhost:5173/ , the /api calls go to the monitor on port 8080
  npm run check   # Svelte check (also in CI)
  npm run build   # ui/dist, embedded in the binary by go build
  ```

  Ohne `npm run build` antwortet die Seite mit „Interface non compilée“. Die Texte sind auf Französisch geschrieben; die englischen Übersetzungen liegen in `ui/src/lib/i18n.svelte.js` und `ui/src/lib/en/*.js`, die deutschen in `ui/src/lib/de/*.js`.
- **Rollen**: Die Routen, die ein Besucher lesen darf, stehen in `internal/api/auth.go` (`visitorRoutes`); die Regeln für Konten und Rechte pro Server in `internal/api/tenancy.go` (`tenantAllowed`).
- **Datenbank**: `docker compose cp monitor:/data/monitor.db .`, dann mit `sqlite3` öffnen.

<details>
<summary><b>API</b> (zum Aufklappen klicken)</summary>

Jede Route `/api/servers/:id/…`, die einen Zeitraum abdeckt, akzeptiert `?range=24h|7d|30d|90d` oder `?from=<unix>&to=<unix>`.

| Route | Wer (Besitzer und Super-Admin: alles auf ihren Servern) | Inhalt |
|---|---|---|
| `GET /api/health` | öffentlich | Zustand des Monitors (Docker-Healthcheck) |
| `POST /api/login` · `POST /api/logout` · `GET /api/me` | öffentlich | Anmeldung, aktuelles Konto und Rolle |
| `GET /api/servers` | Besucher | Serverliste und letzter Status |
| `GET …/status` · `/history` · `/uptime` · `/maps` | Besucher | Status, Spielerdiagramm, Verfügbarkeit, Maps |
| `GET …/players` · `/player?key=` · `/progress?key=` | Besucher | Spieler-Stats (keine IPs für Besucher), Profil, Entwicklung |
| `GET …/combat` · `/rounds` · `/records` · `/badges` · `/duel` | Besucher | Kampf, Runden, Rekorde, Abzeichen, Duell |
| `GET …/matches` · `/match?id=` · `/mapstats` · `/heatmap` | Besucher | Matches, Scoreboard, Bindung pro Map, Stoßzeiten |
| `GET /api/events` | Besucher | Live-Stream (SSE). Besucher bekommen weder Konsole noch Chat. |
| `GET …/mapvotes` | Besucher | Map-Abstimmungen (Mod) |
| `GET …/maprank` · `/week` | Besucher | Maps im Vergleich (Erweiterte Stats → Maps), letzte 7 Tage (Startseite) |
| `GET …/maprounds` · `/retention` | Besucher | Runden pro Map (Dauer, Angriff / Verteidigung), Spielerbindung |
| `GET …/challenges` · `/card.png` | Besucher | Herausforderungen der Woche, Discord-Karte (PNG) |
| `GET …/logs` · `/chat` · `/aliases` · `/alts` · `/errors` · `/pings` · `/cvars` · `/evasion` · `/b3/…` | `private` | Konsole, Chat, Namen, IPs, Fehler, Ping, Variablen, Ban-Umgehung, B3 |
| `GET …/notes` | `private` oder `notes` | Spielernotizen |
| `GET …/reports` · `/inbox` · `POST …/reports/:rid/done` · `/inbox/dismiss` | `reports` (Lesen: oder `private`) | Meldungen, Moderations-Posteingang |
| `POST …/kick` · `/tell` · `/ban` · `/unban` · `PUT …/notes` | `kick` · `say` · `ban` · `ban` · `notes` | Aktionen (im Aktivitätsprotokoll festgehalten) |
| `GET …/bans` | `ban` | Server-Bans |
| `GET …/catalog` | `maps` oder `rcon` | Maps des Servers |
| `POST …/rcon` | `rcon`, oder `maps` / `say` nur für Map- und Nachrichtenbefehle | Befehle |
| `GET …/dvar` · `/rcon/log` · `…/schedules` | `rcon` | Rcon-Variablen, Befehlsverlauf, geplante Befehle |
| `PUT …/welcome` · `PUT …/challenges` · `…/team` · `/api/serverconf/…` | Besitzer | Willkommensnachricht, Ziele der Herausforderungen, Team, Servereinstellungen |
| `/api/invitations` · `/api/serverconf` · `GET /api/audit` · `GET /api/observability` | Admin-Konto | Einladungen, Server, Aktivitätsprotokoll, Zustand der Log-Erfassung (eigene Server) |
| `/api/users` · `GET /metrics` | Super-Admin | Konten, Prometheus-Format |

</details>

### Releases (GoReleaser)

Jeder auf GitHub gepushte **Tag `v*`** startet den Workflow *Release* ([`.goreleaser.yaml`](.goreleaser.yaml)): die Tests, dann
- die Binärdateien (inklusive Oberfläche) für Linux (amd64, arm64), Windows (amd64) und macOS (amd64, arm64), mit `checksums.txt`, auf der Seite [Releases](https://github.com/FloMtz2312/cod4-monitor/releases), mit den Release Notes;
- das Docker-Image `ghcr.io/flomtz2312/cod4-monitor:<version>` (und `latest`) für amd64 und arm64, mit dem Demo-Viewer.

```sh
git tag v1.0.0 && git push origin v1.0.0   # a tag like v1.1.0-rc1 makes a pre-release (no "latest")
goreleaser release --snapshot --clean --skip=docker   # try it locally, files in dist/
cod4-monitor -version                      # the version of a binary
```

Die Binärdatei eines Releases läuft ohne Docker: dieselben `.env`-Einstellungen als Umgebungsvariablen (`.env.example` liegt im Archiv). Der Demo-Viewer ist nicht in den Archiven: Setz `DEMO_VIEWER_DIR` auf eine Kopie von [cod4-demo](https://github.com/TiPSYSPiT/cod4-demo) oder nimm das Docker-Image. Mach das Image beim ersten Mal auf GitHub öffentlich (*Packages → cod4-monitor → Package settings → Change visibility*), damit jeder es herunterladen kann.

---

## 13. Spielserver auf derselben Maschine und erweiterte Einstellungen

Standardmäßig liest der Monitor die Logs eines Servers per **FTP**. Läuft der Spielserver auf **derselben Maschine** wie der Monitor, kann der Monitor sie direkt von der Festplatte lesen: Das ist schneller, und es muss kein FTP eingerichtet werden. Dabei spielt es keine Rolle, wie der Server installiert ist (Docker, als Dienst, von Hand).

### Logs auf derselben Maschine

1. Trag in der `.env` den Ordner des Spielservers auf der Maschine ein: den ganzen Server oder nur den Mod-Ordner.
   ```ini
   LOCAL_LOGS_DIR=/opt/cod4-server/cod4
   ```
2. Häng `-f docker-compose.local-logs.yml` an alle deine `docker compose`-Befehle an, und an die `docker compose`-Zeilen von `/opt/cod4-monitor/deploy.sh`, falls du das [automatische Deployment](#automatisches-deployment-von-github) nutzt:
   ```sh
   docker compose -p cod4-monitor -f docker-compose.prod.yml -f docker-compose.local-logs.yml up -d --build
   ```
   Diese Datei bindet `LOCAL_LOGS_DIR` **schreibgeschützt** unter `/gamelogs/` im Monitor ein.
3. Im Dashboard (als `.env`-Konto): **Mehr → Alle Server** → die Einstellungen des Servers → **Server-Logs → Auf diesem Rechner**, mit dem Ordner so, wie der Monitor ihn sieht. Zum Beispiel `/gamelogs/mods/my_mod/`, wenn du den ganzen Server eingebunden hast.
4. Klicke auf **Testen**. Ist der Ordner falsch, schlägt der Test die vorhandenen vor.

Der Monitor kann nichts außerhalb von `/gamelogs/` lesen.

*Ohne Docker*: `LOCAL_LOGS_ROOT` legt den Ordner fest, aus dem der Monitor lesen darf (Standard `/gamelogs/`).

### Erweiterte Einstellungen (pro Server)

Im Formular eines Servers ist der Block **Erweiterte Einstellungen** für Server gedacht, die nicht die üblichen Namen verwenden:

| Einstellung | Standard | Wofür |
|---|---|---|
| Spiel-Log | `games_mp.log` | Chat, Kills, Runden (das `g_log` des Servers) |
| Konsole | `qconsole.log` | Live-Konsole, Absturzursache (der Monitor liest auch `<console>.old`) |
| Verbindungen | `enterleave.log` | Beitritte und Abgänge mit IP und GUID |
| Zeitzone | `SERVER_LOG_TZ` aus der `.env` | Die Uhr des Spielservers, falls sie von den anderen Servern abweicht |

Ein **Name** wird im Log-Ordner gesucht. Er kann einen Unterordner enthalten, zum Beispiel `logs/games.log`. Ein **Pfad, der mit `/` beginnt**, wird unverändert verwendet: ein vollständiger Pfad auf dem FTP-Server oder innerhalb von `/gamelogs/` für einen Server auf derselben Maschine.

---

## 14. B3: Strafen, Admins und Ränge über das Dashboard

Hat der Spielserver einen **B3**-Admin-Bot, liest und ändert das Dashboard dessen Datenbank: die **SQLite**-Datei von B3 (`b3.db`, auf der Maschine des Monitors, zum Beispiel mit dem Projekt `cod4-server`) oder seine **MySQL / MariaDB**-Datenbank (`storage: mysql://…` von B3, überall, wo der Monitor sie erreicht):

- **Moderation → B3 → Strafen**: Bans, temporäre Bans, Kicks und Verwarnungen, mit dem Spieler, dem Grund, wer sie verhängt hat (ein Admin im Spiel, `B3` oder `site: <account>`) und wann sie enden. Filtern, suchen, einen Ban aufheben oder eine Verwarnung entfernen.
- **Moderation → B3 → Admins**: wer Admin ist und mit welchem Rang (20 Moderator, 40 Admin, 60 Full Admin, 80 Senior Admin, 100 Super Admin), mit den Strafen, die jeder verhängt hat. Der Rang lässt sich in der Liste ändern.
- **Moderation → B3 → Stats**: Strafen pro Woche, häufigste Gründe, meistbestrafte Spieler, aktivste Admins (90 Tage).
- **Spielerprofil → Moderation → B3**: sein Rang, seine Strafen und die Steuerelemente **Verwarnen**, **Kicken**, **Temp-Ban**, **Bannen**, **Ban aufheben** und **B3-Rang**.
- **Moderation → B3 → Suche**: Suche nach Name (aktuell oder früher), IP (oder ihrem Anfang, z. B. `82.64.`) oder GUID, wie `!lookup`: Rang, Verbindungen, erster und letzter Besuch, IP, gebannt oder nicht.
- **Moderation → Umgehungen**: auch die nicht gebannten Spieler, die nach einem gültigen Ban von einer IP des gebannten Spielers kamen (oft dieselbe Person mit einer anderen GUID), mit einem Button, um sie ebenfalls zu bannen.
- **Herkunft der Strafen**: automatisch (Plugins von B3: Beleidigungen, Spam, zu viele Verwarnungen), durch einen Admin im Spiel oder über die Seite, mit dem Schlüsselwort von B3 (`#cuss`, `#spam`…).
- **Spielerprofil**: außerdem die Namen und IPs, die B3 kennt, die Konten, die eine seiner IPs teilen (gebannte markiert), sein maskierter Rang (`!mask`) und seine B3-Willkommensnachricht.
- **Alarme**: Jeder neue Ban oder temporäre Ban (im Spiel oder über die Seite) erscheint im Moderations-Posteingang, als Push-Benachrichtigung und als E-Mail, falls E-Mails eingerichtet sind.

So werden die Aktionen umgesetzt:

| Aktion | Wirkung |
|---|---|
| Verwarnen | In B3 gespeichert (von B3 für seine automatischen Kicks / Bans gezählt) + private Nachricht an den Spieler, wenn er im Spiel ist |
| Kicken | Sofort gekickt (rcon) + in der Historie von B3 festgehalten |
| Ban / temporärer Ban | In B3 gespeichert: Der Spieler wird sofort gekickt, wenn er im Spiel ist, und B3 weist ihn bei jeder Verbindung ab |
| Ban aufheben | Die B3-Bans des Spielers werden aufgehoben, der Server-Ban ebenfalls (`unban`) |
| Rang | In B3 geschrieben; gilt ab der **nächsten Verbindung** des Spielers |

Nur der Besitzer des Servers (und das `.env`-Konto) kann handeln. Moderatoren mit Zugriff auf die privaten Daten sehen die Strafen, können sie aber nicht ändern.

### Einrichtung mit MySQL / MariaDB (jeder Serverbesitzer)

1. Finde die Datenbank von B3 in seiner `b3.ini`: `storage: mysql://user:password@host/database`.
2. Stell sicher, dass der Monitor sie erreicht: MySQL muss auf einer Adresse lauschen, die die Maschine des Monitors erreicht (nicht nur `127.0.0.1`), und seine Firewall muss den Monitor durchlassen. Am besten ist ein eigenes MySQL-Konto für den Monitor, zum Beispiel:
   ```sql
   CREATE USER 'monitor'@'<monitor IP>' IDENTIFIED BY '<password>';
   GRANT SELECT, INSERT, UPDATE ON b3.* TO 'monitor'@'<monitor IP>';
   ```
   (`SELECT` allein reicht, um alles zu sehen; `INSERT` und `UPDATE` braucht es für die Aktionen: Strafen, Ban aufheben, Ränge.)
3. Im Dashboard: **Mehr → Alle Server → die Einstellungen des Servers → B3 → MySQL / MariaDB**: Host, Port, Datenbank, Benutzer, Passwort, verschlüsselte Verbindung, falls der Server sie unterstützt. **Testen** zeigt, wie viele Admins und Strafen gefunden wurden.

*MariaDB neben B3 auf derselben Maschine* (die `docker-compose.mariadb.yml` des Projekts `cod4-server`): Mit `docker-compose.b3.yml` sieht der Monitor den Datenordner von B3, in dem MariaDB seinen Socket ablegt. Trag ins Host-Feld den Pfad des Sockets ein, `/b3/mysqld/mysqld.sock`: Es wird kein Netzwerkport geöffnet (nur das `.env`-Konto kann das einstellen).

Das Passwort wird verschlüsselt gespeichert, wie das rcon-Passwort, und nie an den Browser zurückgeschickt. Unterstützt werden MySQL 5.7+, MySQL 8 und MariaDB 10.2+. Aus Sicherheitsgründen kann ein Besitzer, der nicht das `.env`-Konto ist, den Monitor nicht auf eine lokale oder private Adresse zeigen lassen (`127.0.0.1`, `10.x`, `192.168.x`…): Das kann nur das `.env`-Konto, für ein B3 auf der Maschine oder im Netzwerk des Monitors.

### Einrichtung mit der SQLite-Datei von B3 (das `.env`-Konto)

1. Trag in der `.env` des Dashboards den Ordner von B3 ein, der `b3.db` enthält:
   ```ini
   B3_DATA_DIR=/opt/cod4-server/b3data
   ```
2. Häng `-f docker-compose.b3.yml` an alle deine `docker compose`-Befehle an (neben `docker-compose.local-logs.yml`, falls du sie nutzt), und an die `docker compose`-Zeilen von `/opt/cod4-monitor/deploy.sh`, falls du das [automatische Deployment](#automatisches-deployment-von-github) nutzt:
   ```sh
   docker compose -p cod4-monitor -f docker-compose.prod.yml -f docker-compose.b3.yml up -d --build
   ```
   Diese Datei bindet den Ordner von B3 mit Lese- und Schreibzugriff unter `/b3/` ein. Der Monitor kann nur eine B3-Datenbank innerhalb dieses Ordners öffnen.
3. Im Dashboard: **Mehr → Alle Server → die Einstellungen des Servers → B3 → SQLite-Datei auf diesem Rechner**: `/b3/b3.db`. **Testen** zeigt, wie viele Admins und Strafen gefunden wurden.

*Ohne Docker*: `B3_ROOT` legt den Ordner fest, aus dem der Monitor B3-Datenbanken öffnen darf (Standard `/b3/`).

*Entwickler*: `B3_TEST_MYSQL=mysql://user:pass@127.0.0.1:3306/db go test ./internal/b3/` führt die B3-Tests auch mit MySQL aus (die CI macht das mit MySQL 8). Die Datenbank wird dabei geleert.


## 15. Server-Demos

Der Monitor kann den Spielserver eine **Demo jedes Spielers** aufzeichnen lassen (der CoD4X-Befehl `record`: eine Datei pro Spieler und pro Map, aus der Sicht des Spielers) und sie im Browser mit dem Viewer [cod4-demo](https://github.com/TiPSYSPiT/cod4-demo) öffnen: Es muss nichts heruntergeladen oder importiert werden.

- **Einschalten**: **Mehr → Alle Server → die Einstellungen des Servers → Demos** (der Besitzer des Servers; das rcon-Passwort ist nötig). Sobald mindestens **N echte Spieler** da sind (standardmäßig 2), schickt der Monitor für jeden von ihnen `record <slot>`; Bots werden nie aufgezeichnet. Unterhalb der Schwelle, oder wenn die Option ausgeschaltet wird, schickt er `stoprecord all`.
- **Ansehen**: **Moderation → Demos** listet die Demos auf (Team mit Zugriff auf die privaten Daten), mit einer Auswahl, um eine direkt im Viewer zu öffnen, einem Download-Button und **Behalten**. Der Block **Demos** im Profil eines Spielers (Tab Moderation) zeigt seine letzten.
- **Mit der Moderation verknüpft**: Die Demos eines Spielers, der gemeldet wird, auffällige Stats hat, beobachtet wird, einem gebannten Spieler ähnelt oder gebannt wird (B3 oder die Seite), von 30 Minuten davor bis 10 Minuten danach, werden **90 Tage aufbewahrt** und lassen sich mit einem Klick aus der Karte im Posteingang öffnen.
- **Aufbewahrungsdauer**: Die übrigen Demos werden nach der gewählten Zeit gelöscht (standardmäßig 48 h). Fällt der freie Speicher eines Servers auf derselben Maschine unter 10 %, werden zuerst die ältesten nicht behaltenen gelöscht.

**Wie viel Platz?** Die Einstellungen zeigen beim Tippen eine Schätzung:

> Platz pro Tag = durchschnittliche Spieler × Stunden pro Tag mit mindestens N Spielern × MB pro Spieler und Stunde
> mögliche Aufbewahrungsdauer = verfügbarer Platz ÷ Platz pro Tag

Die Besucherzahlen stammen aus den letzten 30 Tagen des Servers. Eine Demo wiegt etwa **45 MB pro Spieler und Stunde**, bis der Monitor deine gemessen hat (ab 3 Demos). Der verfügbare Platz ist der freie Speicherplatz (20 % bleiben als Reserve) für einen Server auf derselben Maschine, oder der Platz, den du für Demos reservierst (erweiterte Einstellungen), für einen FTP-Server. *Beispiel*: 8 Spieler, 6 h am Tag → 8 × 6 × 45 MB ≈ 2,2 GB pro Tag; mit 20 GB etwa 9 Tage.

**Wo die Dateien liegen**: CoD4X schreibt sie nach `demos/` in seinem Home-Ordner (`fs_homepath`), standardmäßig `../../demos/` vom Log-Ordner (`mods/<mod>/`) aus. Ändere das in den erweiterten Einstellungen, wenn deine Logs woanders liegen.

- *FTP*: Das FTP-Konto muss Dateien in diesem Ordner lesen und löschen dürfen.
- *Dieselbe Maschine* ([Abschnitt 13](#13-spielserver-auf-derselben-maschine-und-erweiterte-einstellungen)): Der Spielordner ist schreibgeschützt eingebunden, daher kann der Monitor alte Demos nicht löschen. Füge `docker-compose.local-demos.yml` hinzu, um den Demo-Ordner mit Lese- und Schreibzugriff einzubinden, nachdem du ihn für den Benutzer des Spielservers angelegt hast:
  ```sh
  mkdir -p /opt/cod4-server/cod4/demos && chown 1000:1000 /opt/cod4-server/cod4/demos
  docker compose -p cod4-monitor -f docker-compose.prod.yml -f docker-compose.local-logs.yml -f docker-compose.local-demos.yml up -d --build
  ```

**Der Viewer** ist im Docker-Image enthalten (Ordner `/demo-viewer`, `DEMO_VIEWER_DIR`). Die Map-Geometrie (119 MB) ist nicht dabei; baue mit `--build-arg DEMO_GEOMETRY=1`, um sie hinzuzufügen. Ohne Docker setzt du `DEMO_VIEWER_DIR` auf eine Kopie von [cod4-demo](https://github.com/TiPSYSPiT/cod4-demo). Der Viewer stammt von TiPSYSPiT und steht unter der GPL-3.0.

Sag deinen Spielern, dass die Spiele aufgezeichnet werden, zum Beispiel in den Regeln oder in der Nachricht des Tages.

## 16. Fehler melden / Mitwirken

- **Ein Fehler oder eine Idee**: [Eröffne ein Issue](https://github.com/FloMtz2312/cod4-monitor/issues/new/choose) und füll das Formular aus, auf Englisch, Französisch oder Deutsch. Der Link ist auch im Dashboard: **Mehr → Über → Bug melden oder Idee vorschlagen**. Füge niemals Passwörter, rcon-Passwörter oder deine `.env` ein.
- **Ein Sicherheitsproblem**: Melde es vertraulich, wie in [SECURITY.md](SECURITY.md) beschrieben, niemals in einem öffentlichen Issue.
- **Eine Code-Änderung**: siehe [CONTRIBUTING.md](CONTRIBUTING.md) (auszuführende Prüfungen, Übersetzungen, Pull Request).

## Umgesetzte Roadmap, Sicherheit und Anleitung zur Wiederherstellung

Siehe [docs/ROADMAP.md](docs/ROADMAP.md) (auf Englisch) für die Funktionen, die Tests und wie du den Verschlüsselungsschlüssel zusammen mit deinen Backups aufbewahrst.

### Serverbesitz und Einladungen

Administratoren und Moderatoren kommen per Einladung dazu. Siehe [Rollen, Einladungen und Rechte pro Server](docs/ACCOUNTS.de.md).

---

## Lizenz

Copyright © 2026 Florian Martinez.

CoD4 Monitor ist freie Software unter der [GNU Affero General Public License v3.0](LICENSE) (AGPL-3.0): Du darfst sie nutzen, ändern und weitergeben, solange du den Urheberrechtshinweis des Autors beibehältst und deine Änderungen unter derselben Lizenz veröffentlichst. Das gilt auch für eine geänderte Version, die du als Website für andere betreibst: Ihre Nutzer müssen an ihren Quellcode kommen können.

Die Map-Bilder (`ui/public/maps/`) gehören Activision und den Map-Autoren; sie fallen nicht unter diese Lizenz.

Das Docker-Image enthält außerdem den Demo-Viewer [cod4-demo](https://github.com/TiPSYSPiT/cod4-demo) von TiPSYSPiT (GPL-3.0), unverändert (seine Tools und Dokumentation sind nicht enthalten), ausgeliefert unter `/demo-viewer/`.
