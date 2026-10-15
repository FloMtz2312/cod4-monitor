# CoD4 Monitor

🇬🇧 [English](README.md) · 🇫🇷 **Français** · 🇩🇪 [Deutsch](README.de.md)

Un tableau de bord web pour suivre un serveur **Call of Duty 4 / CoD4X** : qui joue, quand, sur quelle map, avec quelles stats, et pour l'administrer depuis un navigateur ou un téléphone.

- **En direct** : état du serveur, joueurs connectés, map, console et chat.
- **Statistiques** : activité, classements, badges, records, historique des matchs, fiche de chaque joueur avec sa progression.
- **Administration** : changement de map, kick et ban, commandes programmées, journal des actions, détection des contournements de ban.
- **Accès** : super admin, propriétaires de serveurs, modérateurs avec des permissions par serveur, et visiteurs ; comptes sur invitation uniquement. En français, en anglais et en allemand, utilisable comme une application sur téléphone (notifications et mails en français ou en anglais).

Un bug ou une idée ? [Ouvre une issue](https://github.com/FloMtz2312/cod4-monitor/issues/new/choose) : le formulaire te guide.

---

## Sommaire

1. [Avant de commencer](#1-avant-de-commencer)
2. [Installation sur un VPS](#2-installation-sur-un-vps-recommandé) (recommandé)
3. [Installation sur ton PC](#3-installation-sur-ton-pc-pour-tester) (pour tester)
4. [Mettre à jour](#4-mettre-à-jour)
5. [Configuration (`.env`)](#5-configuration-env)
6. [Comptes et rôles](#6-comptes-et-rôles)
7. [Ce que tu trouves dans le dashboard](#7-ce-que-tu-trouves-dans-le-dashboard)
8. [Logs FTP et commandes RCON](#8-logs-ftp-et-commandes-rcon)
9. [Sauvegardes](#9-sauvegardes)
10. [Dépannage](#10-dépannage)
11. [Aide-mémoire des commandes](#11-aide-mémoire-des-commandes)
12. [Pour les développeurs](#12-pour-les-développeurs)
13. [Serveur de jeu sur la même machine et réglages avancés](#13-serveur-de-jeu-sur-la-même-machine-et-réglages-avancés)
14. [B3 : sanctions, admins et grades depuis le dashboard](#14-b3--sanctions-admins-et-grades-depuis-le-dashboard)
15. [Démos du serveur](#15-démos-du-serveur)
16. [Signaler un bug / Contribuer](#16-signaler-un-bug--contribuer)
- [Licence](#licence)

---

## 1. Avant de commencer

Rassemble ces informations. Seules les deux premières sont obligatoires.

| Quoi | Exemple | À quoi ça sert |
|---|---|---|
| **IP et port du serveur CoD4** | `203.0.113.10:55555` | Interroger le serveur (obligatoire) |
| **Un mot de passe fort** pour le dashboard | — | Protéger le site (obligatoire sur un VPS) |
| Le `rcon_password` du serveur | dans son `server.cfg` | Envoyer des commandes depuis le site (map, kick, ban…) |
| Un accès FTP au dossier du mod | `ftp://user:pass@ip:21/mods/mon_mod/` | Console, chat, kills, connexions avec IP |

Pour une installation en ligne, il te faut aussi :

- **un VPS** sous Ubuntu 24.04 ou Debian 12, 1 à 2 Go de RAM (OVH, Hetzner, Contabo…) ;
- **une adresse web**, gratuite avec [DuckDNS](https://www.duckdns.org) : par exemple `mon-serveur.duckdns.org`.

> 💡 Tout peut se faire depuis un téléphone : l'application **Termius** (iPhone / Android) pour te connecter au VPS, et le navigateur pour GitHub et DuckDNS.

---

## 2. Installation sur un VPS (recommandé)

Le site tourne dans Docker, derrière **Caddy**, qui obtient et renouvelle tout seul le certificat HTTPS.

### Étape 1 : faire pointer ton adresse vers le VPS

Sur [duckdns.org](https://www.duckdns.org), connecte-toi, crée un sous-domaine (ex. `mon-serveur`) et colle l'**IP de ton VPS** dans le champ *current ip*. Valide avec *update ip*.

### Étape 2 : se connecter au VPS

Depuis un terminal (ou Termius, ou la console web de ton hébergeur) :

```sh
ssh ubuntu@IP_DU_VPS
```

Remplace `ubuntu` par l'utilisateur donné par ton hébergeur (parfois `root`).

### Étape 3 : récupérer le projet

Installe git, puis clone le projet dans `/opt/cod4-monitor` :

```sh
sudo apt update && sudo apt install -y git
sudo mkdir -p /opt/cod4-monitor && sudo chown $USER /opt/cod4-monitor
git clone https://github.com/FloMtz2312/cod4-monitor.git /opt/cod4-monitor
cd /opt/cod4-monitor
```

### Étape 4 : préparer le VPS

Une seule commande installe Docker, active le pare-feu (seuls SSH, HTTP et HTTPS restent ouverts) et les mises à jour de sécurité automatiques :

```sh
sudo sh deploy/setup-vps.sh
sudo usermod -aG docker $USER
```

**Déconnecte-toi puis reconnecte-toi** au VPS pour pouvoir utiliser `docker` sans `sudo`.

> ⚠️ Si ton hébergeur a aussi un pare-feu dans son espace client, ouvre-y les ports **80** et **443**.

### Étape 5 : configurer

```sh
cd /opt/cod4-monitor
cp .env.example .env
nano .env
```

Remplis au minimum ces lignes :

```ini
SERVERS=Mon Serveur|203.0.113.10:55555
DASHBOARD_USER=admin
DASHBOARD_PASSWORD=un-mot-de-passe-long-et-unique
DOMAIN=mon-serveur.duckdns.org
```

Enregistre avec **Ctrl+O** puis **Entrée**, et quitte avec **Ctrl+X**.

Pour la console, le chat, les commandes et toutes les options, voir [Configuration](#5-configuration-env) et [Logs FTP et commandes RCON](#8-logs-ftp-et-commandes-rcon).

### Étape 6 : démarrer

```sh
mkdir -p backups && sudo chown 1000 backups && chmod 600 .env
docker compose -p cod4-monitor -f docker-compose.prod.yml up -d --build
```

La première construction prend quelques minutes. Vérifie ensuite que tout tourne :

```sh
docker compose -p cod4-monitor -f docker-compose.prod.yml ps
```

Les deux lignes (`monitor` et `caddy`) doivent afficher **Up**. Si `monitor` affiche *Restarting*, va voir le [Dépannage](#10-dépannage).

### Étape 7 : ouvrir le site

Va sur `https://mon-serveur.duckdns.org`. Le certificat peut mettre une minute à arriver la première fois.

- **Sans te connecter**, tu vois le site comme un visiteur : les stats publiques.
- **Se connecter** (en haut à droite), avec `DASHBOARD_USER` et `DASHBOARD_PASSWORD` : c'est le compte super admin, avec tous les accès.
- Pour donner un accès à quelqu'un d'autre : **Plus → Équipe et invitations** (un lien d'invitation, voir [Comptes et rôles](#6-comptes-et-rôles)).

📱 **Sur téléphone** : ouvre le site, puis *Partager* → **Sur l'écran d'accueil** (iPhone) ou *menu* → **Ajouter à l'écran d'accueil** (Android). Le site s'ouvre alors comme une application.

> ℹ️ **Tu avais déjà un monitor sur ton PC ?** Arrête-le (`docker compose down`). Deux monitors sur le même serveur enverraient chacun les commandes programmées.

---

## 3. Installation sur ton PC (pour tester)

Il faut [Docker Desktop](https://www.docker.com/products/docker-desktop/). Dans le dossier du projet :

```sh
cp .env.example .env      # puis modifie au moins SERVERS, DASHBOARD_USER et DASHBOARD_PASSWORD
docker compose up -d
```

Ouvre ensuite http://localhost:8080 (ou le port choisi dans `HTTP_PORT`).

- **Avec Docker, `DASHBOARD_USER` et `DASHBOARD_PASSWORD` sont obligatoires, même sur un PC.** Avec un mot de passe vide, le monitor n'écoute que sur `127.0.0.1` dans son conteneur : le port publié par Docker ne peut pas l'atteindre.
- Un mot de passe vide ne marche qu'en lançant le binaire directement (`go run ./cmd/monitor`, voir [Pour les développeurs](#12-pour-les-développeurs)) : aucune connexion n'est demandée, et le site et ses commandes ne fonctionnent que depuis le PC lui-même (`localhost`).
- Pour ouvrir le site depuis un téléphone **sur le même Wi-Fi**, va sur `http://IP_DU_PC:8080`. Sous Windows, autorise une fois le port dans le pare-feu (PowerShell en administrateur) :
  ```powershell
  New-NetFirewallRule -DisplayName "CoD4 Monitor (LAN)" -Direction Inbound -Protocol TCP -LocalPort 8080 -Action Allow -Profile Private
  ```
  Le trafic n'est pas chiffré : à garder sur le réseau local. Pour un accès depuis internet, utilise l'installation sur VPS.

Pour que le monitor démarre avec Windows, active *Start Docker Desktop when you sign in* dans les réglages de Docker Desktop.

---

## 4. Mettre à jour

Sur le VPS :

```sh
cd /opt/cod4-monitor && git pull && docker compose -p cod4-monitor -f docker-compose.prod.yml up -d --build
```

Le `.env`, la base de données et les sauvegardes sont conservés. Les tests automatiques tournent pendant la construction : s'ils échouent, la construction s'arrête et **l'ancienne version reste en ligne**.

Si tu utilises des fichiers compose en plus (`docker-compose.local-logs.yml`, `docker-compose.b3.yml`), ajoute aussi leurs `-f …` à cette commande.

**Option : déployer depuis ton PC.** `sh deploy/push.sh ubuntu@IP_DU_VPS` envoie ton dossier local (`.env` compris) et ta base locale (seulement si le VPS n'en a pas encore), puis démarre le site. ⚠️ Ce script **remplace le `.env` du VPS** par celui de ton PC.

### Déploiement automatique depuis GitHub

Optionnel, pour ceux qui font tourner leur propre fork : chaque push sur `main` met le VPS à jour tout seul.

1. **Forke** le dépôt sur GitHub. Sur le VPS, le projet doit venir de ton fork : clone l'adresse de ton fork à l'[étape 3](#étape-3--récupérer-le-projet), ou lance `git remote set-url origin https://github.com/TON_NOM/cod4-monitor.git`.
2. Sur le VPS, copie le script de déploiement :
   ```sh
   cd /opt/cod4-monitor
   cp deploy/deploy.sh /opt/cod4-monitor/deploy.sh && chmod +x /opt/cod4-monitor/deploy.sh
   ```
   Si le projet n'est pas dans `/opt/cod4-monitor`, change la ligne `cd` de la copie et utilise ton chemin dans la suite. Si tu utilises des fichiers compose en plus, ajoute leurs `-f …` aux lignes `docker compose` de la copie.
3. Crée une clé qui ne sert qu'au déploiement :
   ```sh
   ssh-keygen -t ed25519 -N "" -C github-deploy -f ~/.ssh/github_deploy
   ```
4. Autorise cette clé, mais seulement pour lancer `deploy.sh` :
   ```sh
   echo "command=\"/opt/cod4-monitor/deploy.sh\",no-port-forwarding,no-agent-forwarding,no-X11-forwarding,no-pty $(cat ~/.ssh/github_deploy.pub)" >> ~/.ssh/authorized_keys
   ```
5. Dans ton fork : **Settings → Secrets and variables → Actions → New repository secret**. Crée quatre secrets :

   | Secret | Valeur |
   |---|---|
   | `VPS_HOST` | L'IP du VPS |
   | `VPS_USER` | Ton utilisateur sur le VPS (ce qu'affiche `whoami`) |
   | `VPS_SSH_KEY` | Le contenu de `~/.ssh/github_deploy` (la clé privée : `cat ~/.ssh/github_deploy`) |
   | `VPS_KNOWN_HOST` | Ce qu'affiche `ssh-keyscan -t ed25519 IP_DU_VPS` |

   Sur un fork, GitHub désactive d'abord les workflows : ouvre l'onglet **Actions** de ton fork et active-les.

Ensuite, chaque push sur `main` lance les vérifications (`go vet`, tests) sur GitHub, se connecte au VPS et lance `deploy.sh` : il suit `main` exactement, reconstruit le monitor et attend jusqu'à 2 minutes qu'il soit en bonne santé. Les modifications faites à la main sur des fichiers suivis par git sur le VPS sont perdues ; le `.env`, la base et les sauvegardes sont gardés. Pour déployer à la main : **Actions → « Déploiement VPS » → Run workflow**. Sans les secrets `VPS_*`, le workflow ne fait rien.

---

## 5. Configuration (`.env`)

Après toute modification du `.env`, relance avec la commande de démarrage de l'[étape 6](#étape-6--démarrer).

### L'essentiel

| Variable | Exemple | Rôle |
|---|---|---|
| `SERVERS` | `Mon Serveur\|203.0.113.10:55555` | Le ou les serveurs à suivre au **premier démarrage** (détails ci-dessous). Ensuite, ils se gèrent depuis le site |
| `DASHBOARD_USER` | `admin` | Identifiant du compte **super admin**. **Obligatoire** avec un mot de passe : sans lui, le monitor ne démarre pas |
| `DASHBOARD_PASSWORD` | — | Mot de passe du super admin. **Obligatoire avec Docker.** Vide = aucune connexion, le monitor n'écoute que sur `127.0.0.1` : seulement en lançant le binaire directement sur ton PC |
| `DOMAIN` | `mon-serveur.duckdns.org` | Adresse publique (VPS uniquement) |
| `TZ` | `Europe/Paris` | Fuseau horaire d'affichage |

### Le format de `SERVERS`

> **`SERVERS` n'est lu qu'au premier démarrage** : les serveurs sont alors copiés dans la base. Ensuite, ils s'ajoutent, se modifient, se testent, s'arrêtent ou reprennent depuis **Plus → Tous les serveurs** (réglages de chaque serveur, ou **+ Ajouter un serveur** : adresse, FTP, rcon), sans redémarrer le monitor. Une fois les serveurs dans la base, modifier `SERVERS` n'a plus d'effet. Tu peux même le laisser vide et ajouter ton premier serveur depuis le site.

Chaque serveur s'écrit avec jusqu'à 4 champs séparés par `|`. Seuls les deux premiers sont obligatoires :

```
Nom|ip:port|url_ftp_du_dossier_du_mod|rcon_password
```

Exemple complet :

```ini
SERVERS=Mon Serveur|203.0.113.10:55555|ftp://user:motdepasse@203.0.113.10:21/mods/mon_mod/|monrcon
```

- **Plusieurs serveurs** : sépare-les par `;`. Une liste déroulante apparaît alors dans le site.
- **Caractères spéciaux** dans le mot de passe FTP, à remplacer : `@` → `%40`, `:` → `%3A`, `/` → `%2F`, `#` → `%23`, `%` → `%25`.

### Les autres réglages (les valeurs par défaut conviennent)

| Variable | Défaut | Rôle |
|---|---|---|
| `POLL_INTERVAL` | `15s` (`5s` dans `.env.example`) | Fréquence d'interrogation du serveur (5 s minimum) |
| `QUERY_TIMEOUT` | `2s` | Délai d'attente d'une réponse |
| `OFFLINE_AFTER` | `45s` | Durée sans réponse avant d'afficher « hors ligne » (un changement de map ne répond pas pendant 20 à 40 s) |
| `LOG_POLL_INTERVAL` | `2s` | Fréquence de lecture des logs par FTP |
| `LOG_FTP_SESSION` | `60s` | La connexion FTP est renouvelée après cette durée : certains serveurs FTP montrent un fichier tel qu'il était à l'ouverture de la connexion, le chat n'arrivait alors qu'à la coupure de la connexion par le serveur. `0` = jamais |
| `HISTORY_FTP_MAX_MB` | `50` | Taille maximale du `games_mp.log` dont le passé est importé par FTP au premier démarrage (il est téléchargé deux fois). `0` = jamais par FTP |
| `IMPORT_MAX_MB` | `2048` | Taille maximale d'un ancien fichier de log envoyé par l'import des réglages du serveur |
| `SERVER_LOG_TZ` | `UTC` | Fuseau horaire de l'horloge du serveur de jeu |
| `HTTP_PORT` | `8080` | Port d'écoute du monitor (sur un VPS, Caddy lui transmet les requêtes) |
| `HTTP_BIND` | vide (toutes les adresses) | Adresse d'écoute du monitor. Forcée à `127.0.0.1` quand `DASHBOARD_PASSWORD` est vide |
| `TRUSTED_PROXIES` | `127.0.0.1/32,::1/128` | Reverse proxys autorisés à transmettre l'IP du visiteur (`X-Forwarded-For`). `docker-compose.prod.yml` met l'adresse de Caddy tout seul. `none` = ignorer ces en-têtes |
| `DB_PATH` | `monitor.db` (`/data/monitor.db` dans `.env.example`) | La base de données. Avec Docker, elle est dans le volume `data` |
| `MONITOR_SECRET_KEY` | — | Clé qui chiffre les mots de passe FTP, rcon et B3 enregistrés (base64 de 32 octets, ex. `openssl rand -base64 32`). Vide = le fichier `DB_PATH.key` est créé à côté de la base. Voir [Sauvegardes](#9-sauvegardes) |
| `RETENTION_DAYS` | `90` | Durée de conservation des sondages, en jours (`0` = toujours) |
| `BACKUP_DIR` | vide (`/backups` dans `.env.example`) | Dossier des sauvegardes quotidiennes (`./backups` du projet avec Docker). Vide = pas de sauvegarde |
| `BACKUP_KEEP` | `14` | Nombre de sauvegardes quotidiennes gardées |
| `DISCORD_WEBHOOK_URL` | — | Alertes Discord : serveur en panne ou de retour |
| `DISCORD_PLAYER_THRESHOLD` | `0` | Alerte Discord quand le nombre de joueurs atteint ce seuil (`0` = désactivé) |

### Mails (optionnel)

Le monitor peut t'envoyer des mails (à l'admin). Indique un compte mail qui sert à envoyer, et l'adresse qui reçoit :

| Variable | Exemple | Rôle |
|---|---|---|
| `SMTP_HOST` | `smtp.gmail.com` | Serveur mail du compte qui envoie. **Vide = aucun mail.** |
| `SMTP_PORT` | `587` | `587` (STARTTLS) ou `465` (TLS) |
| `SMTP_USER` / `SMTP_PASSWORD` | `mon.monitor@gmail.com` | Identifiants du compte qui envoie |
| `MAIL_FROM` | — | Adresse de l'expéditeur (par défaut : `SMTP_USER`) |
| `MAIL_TO` | `moi@outlook.com` | Qui reçoit les mails (plusieurs : séparés par `,`) |
| `MAIL_LANG` | `fr` | `fr` ou `en` |
| `WEEKLY_MAIL` | — | `0` = pas de résumé de la semaine |
| `VPS_DISK_ALERT` / `VPS_MEM_ALERT` | `90` | Seuil d'alerte du disque / de la mémoire, en % |
| `VPS_LOAD_ALERT` | `2` | Alerte processeur : charge sur 15 minutes par cœur |

**Avec Gmail** (le plus simple) : active la validation en deux étapes sur le compte, crée un *mot de passe d'application* (compte Google → Sécurité → Mots de passe des applications) et mets-le dans `SMTP_PASSWORD`. Un compte gratuit [Brevo](https://www.brevo.com) marche aussi (`smtp-relay.brevo.com`, port 587).

#### Avec Resend (adresse de ton propre domaine)

[Resend](https://resend.com) envoie les mails depuis une adresse de ton domaine (ex. `monitor@mon-domaine.fr`), et ils arrivent rarement en indésirables. L'offre gratuite suffit largement : 3 000 mails par mois, 100 par jour.

Il te faut **un nom de domaine à toi** (quelques euros par an chez OVH, Gandi, Cloudflare…) : Resend demande d'ajouter des enregistrements DNS, ce qu'une adresse DuckDNS ne permet pas.

1. Crée un compte sur [resend.com](https://resend.com).
2. **Domains → Add Domain** : entre ton domaine. Un sous-domaine (ex. `mail.mon-domaine.fr`) garde ton domaine principal à l'écart.
3. Resend affiche des enregistrements DNS (un `MX` et un `TXT` pour SPF, un `TXT` pour DKIM). Ajoute-les tels quels dans la zone DNS de ton domaine, chez ton registraire, puis clique sur **Verify DNS Records**. Le domaine passe en *Verified* en quelques minutes, parfois quelques heures.
4. **API Keys → Create API Key** : nom `cod4-monitor`, permission **Sending access**, limitée à ton domaine. Copie la clé (elle commence par `re_`) : elle ne s'affiche qu'une fois.
5. Dans le `.env` du monitor :
   ```sh
   SMTP_HOST=smtp.resend.com
   SMTP_PORT=465
   SMTP_USER=resend
   SMTP_PASSWORD=re_ta_cle_api
   MAIL_FROM=monitor@mon-domaine.fr
   MAIL_TO=moi@exemple.fr
   ```
   `SMTP_USER` vaut toujours le mot `resend`. `MAIL_FROM` est obligatoire : une adresse de ton domaine vérifié (elle n'a pas besoin d'exister comme boîte mail). Le port `587` marche aussi.
6. Relance le monitor (`docker compose … up -d`). Un mail de test part au démarrage ; tu le vois aussi dans **Emails** sur le site de Resend. S'il n'arrive pas : `docker logs cod4-monitor-monitor-1 | grep mail`.

Pour un simple essai sans domaine, Resend accepte `MAIL_FROM=onboarding@resend.dev`, mais seulement vers l'adresse de ton compte Resend.

Ce que tu reçois :

- **Un mail de test** quand le monitor démarre avec de nouveaux réglages mail : s'il n'arrive pas, regarde `docker logs cod4-monitor-monitor-1 | grep mail`.
- **Alertes du VPS** : disque presque plein, mémoire ou processeur saturés depuis plus de 5 minutes, monitor qui redémarre en boucle. Puis un rappel toutes les 24 h tant que ça dure, et un mail quand c'est réglé.
- **Résumé de la semaine**, le lundi à partir de 9 h : joueurs, temps de jeu, pic, matchs, joueur de la semaine, records, top 5, map la plus fréquentée.
- **Crash du serveur** : quand le serveur de jeu ne répond plus, redémarre tout seul (crash + redémarrage automatique) ou revient, avec la cause probable (dernière erreur de sa console, par ex. `ERROR: Exceeded limit of 1000 'xmodel' assets`) et les dernières lignes de la console. Après un redémarrage, la fin de l'ancienne console est lue dans `qconsole.log.old` (nécessite les logs FTP). Un arrêt normal (quit, redémarrage par l'hébergeur) est seulement enregistré, sans mail. La cause s'affiche aussi sous chaque coupure, sur l'accueil.
- **Alertes de joueurs** : quand se connecte un joueur qui a une note d'admin **⚠ Suspect** ou **👁 À surveiller**, ou un `!report` pas encore traité (30 derniers jours). Au plus un mail par joueur toutes les 6 heures.

### Notifications push (téléphone)

Chaque compte de l'équipe peut aussi recevoir des **notifications sur son téléphone, même application fermée** : **Plus → Notifications** → *Activer les notifications*, puis un interrupteur par alerte :

- **Serveur tombé ou revenu** (crash, redémarrage inattendu, retour en ligne, avec la cause probable) : tout compte qui a des droits sur le serveur.
- **Nouveau signalement** (`!report` en jeu) : les comptes qui peuvent traiter les signalements ou voir les données privées.
- **Joueur suspect ou banni qui revient** (stats anormales, joueur noté suspect / à surveiller en jeu, contournement de ban possible) et **Ban B3** : les comptes qui voient les données privées.

Rien à configurer : aucun compte tiers, aucune variable dans le `.env`. Le monitor crée sa propre clé (VAPID) au premier démarrage et la garde dans sa base ; le service push du téléphone (Google, Apple, Mozilla, Microsoft) livre les notifications. Les permissions sont vérifiées à chaque envoi : retirer un droit coupe aussi ses notifications. **iPhone** : iOS 16.4 ou plus, et seulement depuis l'application ajoutée à l'écran d'accueil (Safari → Partager → *Sur l'écran d'accueil*, puis l'ouvrir depuis là). Chaque téléphone ou navigateur a ses propres choix, et peut couper un serveur. Les mails ci-dessus ne changent pas.

---

## 6. Comptes et rôles

| Rôle | Qui | Voit | Peut faire |
|---|---|---|---|
| 👑 **Super admin** | le compte du `.env` (`DASHBOARD_USER`) | tout, sur tous les serveurs | tout : tous les serveurs, invite les administrateurs, gère les comptes, transfère les serveurs, la machine du monitor (logs et B3 sur cette machine) |
| 🏠 **Admin** (propriétaire de serveur) | invité par le super admin | tout sur ses propres serveurs ; seulement les stats publiques des autres | ajoute ses serveurs et les règle (adresse, FTP, rcon, B3), invite des modérateurs, toutes les actions sur ses serveurs |
| 🛡 **Modérateur** | invité sur un serveur par son propriétaire | ce que ses permissions permettent, serveur par serveur | ce que ses permissions permettent (ci-dessous) |
| 👀 **Visiteur** | n'importe qui, sans compte | les stats publiques : état, joueurs, activité, classements, matchs | rien |

Les permissions d'un modérateur se choisissent par serveur : **consulter les données privées** (IP, chat, logs, notes), **expulser** un joueur, **bannir** et débannir, **traiter les signalements**, **modifier les notes joueurs**, **changer la map** / relancer la partie, **envoyer un message**, **console RCON complète** et commandes programmées (contrôle total du jeu). Un modérateur n'en a aucune par défaut, et voit les siennes dans **Plus → Mes droits sur … → Mes permissions**.

- **Les comptes se créent uniquement sur invitation** : **Plus → Équipe et invitations** → un lien d'invitation valable 48 h. Le propriétaire invite les modérateurs du serveur ; le super admin peut aussi inviter des administrateurs. Le super admin gère tous les comptes dans **Plus → Comptes du dashboard** (nouveau mot de passe, suppression).
- Les **visiteurs ne voient jamais** les IP, la console, le chat ni les notes d'admin. Ce blocage est fait par le serveur, pas seulement masqué dans la page.
- **Sécurité** : mots de passe hachés (PBKDF2-SHA256), connexion valable 7 jours, blocage de 15 minutes après 10 échecs depuis une même IP.

Détails (invitations, permissions, transfert d'un serveur) : [docs/ACCOUNTS.md](docs/ACCOUNTS.md).

---

## 7. Ce que tu trouves dans le dashboard

Le site est une application pensée d'abord pour le téléphone (onglets en bas sur téléphone, menu latéral sur ordinateur). Elle s'installe sur l'écran d'accueil (**Plus → Installer l'application**). Les onglets : **Direct**, **Classements**, **Joueurs**, **Matchs** (visiteurs) ou **Modération** (équipe), **Plus**.

| Écran | Contenu |
|---|---|
| **Direct** | Carte du serveur (état, map, score du match en direct, rejoindre / partager), latence, ping et disponibilité sur 30 jours, détails du serveur, joueurs connectés, activité du jour, les 7 derniers jours (joueur de la semaine, top 5, records). Pour l'équipe qui a les droits : changer de map (recherche, images, mode de jeu), redémarrer, messages, kick / ban, maps du vote de fin de match, boost XP. |
| **Modération** (équipe) | Boîte de réception (À traiter / Historique) : signalements, stats suspectes, joueurs à surveiller en jeu, contournements de ban possibles, bans B3. Outils, selon les permissions : Chat avec recherche, Console (console du jeu en direct), Bans (bans du serveur et débannissement), Contournements (indices de contournement de ban), Comptes liés (pseudos par GUID, comptes liés par IP), B3 (sanctions, admins et grades, recherche, stats). |
| **Joueurs** | Tous les joueurs de la période (filtres : en ligne, nouveaux, notés), recherche par pseudo actuel ou ancien (et par IP pour l'équipe). Sur ordinateur, un tableau avec temps de jeu, sessions, ping, meilleur score, messages… |
| **Fiche joueur** | Résumé (forme par rapport à la période précédente, comparaison avec un joueur, progression K/D / headshots / précision), combat (armes, zones touchées, némésis), matchs, modération (note, message de bienvenue, sessions, IP, comptes liés, chat, grade et sanctions B3). Boutons **Copier le lien**, **Duel** et **Carte Discord** (une image PNG des 7 derniers jours du joueur, sans IP ni GUID, à poster sur Discord). |
| **Classements** | Badges, records (joueur du jour et de la semaine), classement par critère (tableau complet triable sur ordinateur), défis de la semaine (kills, headshots, objectifs de toute la communauté ; le propriétaire règle les objectifs), **Duel** (qui a tué qui entre deux joueurs, depuis le début). |
| **Matchs** (visiteurs) | Matchs et rounds : tableaux des scores, faits marquants, une **Carte Discord** (PNG) par match. Les comptes de l'équipe les trouvent dans Plus → Matchs récents. |
| **Plus** | **Stats avancées** (affluence, maps avec verdict à garder / à remplacer et votes de map, fidélité, combat), **Matchs récents**, **État du serveur** (disponibilité, coupures et leur cause, latence, ping des joueurs, erreurs de la console, variables du serveur, collecte des logs et sauvegardes), **Tous les serveurs** (réglages de chaque serveur, **+ Ajouter un serveur**). Puis, pour le serveur sélectionné et selon les droits : Personnalisation, Équipe et invitations, Réglages du serveur, Console, Commandes programmées, Comptes du dashboard, Logs du monitor (super admin : les dernières lignes de `docker logs`, secrets masqués), Journal d'activité. Puis Notifications, Installer l'application, Mot de passe, et **À propos** (Signaler un bug ou proposer une idée, Code source). |

Aussi :

- **Sur ordinateur** : menu latéral, recherche avec **Ctrl K** ou **/** (joueurs par pseudo actuel ou ancien, écrans).
- Les **anciens liens** marchent toujours : `/v2/…`, `/login.html`, `/invite.html#…`, `#accueil`, `#joueur/<clé>`, `#admin`…
- **Langue** : **Plus → Langue** (français, anglais, allemand). Par défaut, le site prend la langue du navigateur.
- **Badges** : meilleur joueur de la période dans 7 catégories. L'icône s'affiche à côté du pseudo partout sur le site.

---

## 8. Logs FTP et commandes RCON

### Logs FTP (console, chat, kills, connexions)

Avec l'URL FTP du dossier du mod en 3e champ de `SERVERS`, le monitor lit trois fichiers :

| Fichier | Ce qu'il apporte |
|---|---|
| `qconsole.log` | Console affichée en direct (rien n'est stocké) |
| `games_mp.log` | Chat, kills, rounds, précision, liens pseudo ↔ GUID. Au premier lancement, la partie écrite avant le monitor est importée une fois en arrière-plan (joueurs, kills, chat), datée grâce à `enterleave.log` : ce log ne contient que le temps depuis le démarrage du serveur, donc une période que `enterleave.log` ne couvre pas est ignorée, et ces anciens kills n'ont pas de round. |
| `enterleave.log` | Connexions et départs avec IP et GUID. Tout l'historique est importé au premier lancement. |

Sans FTP, le site fonctionne quand même, mais seulement avec les infos publiques du serveur : joueurs, scores, map.

**Importer d'anciens logs** : dans les réglages d'un serveur (son propriétaire), envoie un ancien `enterleave.log` ou `games_mp.log` (le fichier mis de côté quand le serveur en a commencé un nouveau, une sauvegarde…). Le monitor l'importe en arrière-plan sans arrêter la lecture en direct, et ignore ce qu'il a déjà : un fichier peut être renvoyé sans risque. Envoie d'abord l'`enterleave.log` : il sert à dater les lignes du `games_mp.log` de la même période. Jusqu'à `IMPORT_MAX_MB` (2048) par fichier.

L'écran **Joueurs** liste les connexions d'`enterleave.log` (avec le temps de jeu), complétées par les joueurs vus seulement dans `games_mp.log` ou par B3 (marqués « Vu dans le log du jeu » / « Vu par B3 », sans temps de jeu).

### Commandes RCON

Avec le `rcon_password` en 4e champ de `SERVERS` (ou dans les réglages du serveur), le propriétaire et les modérateurs qui ont les permissions correspondantes ont accès à :

- **changer de map et de mode**, depuis une galerie des maps du serveur (maps stock et `usermaps/`, lues par FTP) ;
- **map suivante, redémarrage, message à tous, commande libre** ;
- **kick et ban** depuis la liste des joueurs ou leur fiche ;
- **maps du vote de fin de match et boost XP** ;
- **commandes programmées**, toutes les N minutes ou chaque jour à heure fixe.

Chaque action est enregistrée dans le **journal d'activité** (Plus → Journal d'activité), avec le compte qui l'a faite.

> Le mod ne peut pas être changé à chaud (CoD4X protège `fs_game`) : il se règle dans les paramètres de démarrage du serveur (OGP).

### Stats avancées avec le mod `pm_sniper`

Le mod `pm_sniper` (dépôt séparé) écrit des lignes supplémentaires dans `games_mp.log`. Le monitor les lit automatiquement, sans aucun réglage :

| Ligne | Ce qu'elle apporte au dashboard |
|---|---|
| `KE` | Distance de chaque kill, noscope, quickscope, wallbang, points |
| `RE` / `CL` | Équipe gagnante et score de chaque round, clutchs (dernier en vie qui gagne) |
| `P_P` / `P_D` | Bombe posée / désamorcée (existait déjà dans Promod) |
| `MS` / `ME` | Score officiel, niveau et prestige de chaque joueur en fin de match, gagnant du match |
| `MV` / `MVW` | Résultats du vote de map de fin de match |
| `PI` | Profil du joueur à son arrivée (niveau, prestige, stats de carrière) : détection des contournements de ban |

On les retrouve dans le classement (noscopes, quickscopes, wallbangs, plus long kill, clutchs, premier sang, % de victoires), les records et badges, les matchs (gagnant, score, faits marquants de chaque round), la fiche joueur et **Plus → Stats avancées → Maps** (votes de map). Sans ce mod, ces colonnes et sections sont masquées et tout le reste fonctionne.

### Commandes dans le chat du jeu

Avec le `rcon_password` dans `SERVERS`, le monitor répond aux joueurs en message privé (`tell`) :

| Commande | Réponse |
|---|---|
| `!stats` ou `!stats pseudo` | Kills, morts, K/D, % de headshots, noscopes, plus long kill |
| `!top` | Les 5 meilleurs tueurs des 7 derniers jours |
| `!report pseudo raison` | Signalement envoyé aux admins : il apparaît dans **Modération** (et en notification push) |
| `!me` | Un lien court à taper sur le téléphone (`ton-domaine/me/K7QPX`, ou un code sans `DOMAIN`) : il ouvre la fiche du joueur et l'épingle (**C'est moi**). Valable 24 heures |
| `!link CODE` | Relie le joueur au compte du site qui a affiché ce code (tchat des admins, voir plus bas). Valable 10 minutes, une seule fois |
| `!help` | Liste des commandes (si le serveur a B3, c'est B3 qui répond : sa liste inclut ces commandes) |

Une commande toutes les 3 secondes par joueur, un signalement par minute.

### Tchat des admins

**Modération → Tchat des admins** : un tchat entre les admins du serveur, sur le site. Il faut la connexion par compte. **Sans B3**, il est ouvert à toute l'équipe du serveur (propriétaire et modérateurs), sans rien à relier, et on mentionne avec `@identifiant` du compte. **Avec B3**, il est réservé aux comptes de l'équipe dont le joueur en jeu est **admin B3 de niveau 20 ou plus** :

1. La première fois, *Obtenir un code*, puis taper `!link CODE` dans le chat du jeu sur ce serveur (10 minutes). Le GUID vient du log du serveur : le compte est relié à ce joueur. *Délier* pour changer de joueur.
2. Le grade est relu dans B3 : un admin rétrogradé sous le niveau 20 perd l'accès.
3. **Mentions** : `@12` prévient l'admin dont le numéro B3 est 12 (le `@id` de B3, affiché à côté de chaque admin), `@pseudo` par compte ou pseudo en jeu, `@all` tout le monde. Taper `@` propose la liste.
4. L'admin mentionné reçoit une **notification push** (s'il a activé les notifications sur son téléphone, quels que soient ses choix d'alertes) et une **pastille** sur l'onglet Modération et la cloche, jusqu'à ce qu'il lise le message.

### Contournements de ban (spoofer de GUID)

**Modération → Contournements** : chaque joueur actif depuis un ban est comparé au joueur banni, et chaque indice ajoute des points. Une alerte en direct s'affiche à partir de **50**. Rien n'est banni automatiquement : c'est à toi de vérifier.

| Indice | Points |
|---|---|
| Même IP, ou même Steam ID | 60 |
| Même profil de carrière : même prestige, kills de carrière juste au-dessus de ceux du banni (le profil reste sur le PC du joueur, même s'il change de GUID) | 40 |
| Même pseudo (sans couleurs ni l33t) | 25 |
| Même réseau (IP qui ne diffère que par le dernier nombre) | 20 |
| Nouveau GUID qui arrive déjà avec un gros profil (niveau 10+ ou prestige) | 20 |
| Mêmes réglages réseau, s'ils ne sont pas ceux par défaut (`rate`, `snaps`, `cl_maxpackets`, lus par rcon `dumpuser`) | 15 |
| Pseudo à 1 ou 2 lettres près | 15 |
| Style de jeu proche (% de headshots, % de noscopes, distance des kills) | 10 |
| GUID apparu après le ban | 10 |

Les indices faibles ne comptent qu'en plus d'un indice qui relie le joueur au banni, et un joueur déjà connu avant le ban n'est signalé que sur un indice fort (IP, Steam ID, profil, pseudo). Le profil vient de la ligne `PI` du mod `pm_sniper`, et les réglages réseau du mot de passe rcon. Un tricheur qui change à la fois de GUID, d'IP (VPN), de profil CoD4, de pseudo et de façon de jouer reste indétectable.

**Message de bienvenue** : dans la fiche d'un joueur (onglet Modération), le **propriétaire** du serveur (pas un modérateur) peut écrire un message personnel, envoyé en privé 30 secondes après chaque connexion de ce joueur. `{name}` est remplacé par son pseudo.

**Images des maps custom** : les vignettes sont dans `ui/public/maps/`. Après avoir ajouté des maps à ton jeu, régénère-les depuis ton PC :

```sh
docker run --rm -v "$PWD":/src -v "<dossier cod4>":/game:ro -w /src golang:1.26-alpine go run ./cmd/mapimages /game ui/public/maps
```

---

## 9. Sauvegardes

Chaque jour, une copie de la base est écrite dans `backups/monitor-AAAA-MM-JJ.db` (`BACKUP_DIR`), puis vérifiée en la restaurant. Les 14 dernières sont gardées (`BACKUP_KEEP`).

> ⚠️ Ces sauvegardes sont **sur le VPS lui-même**. Pense à en copier une de temps en temps ailleurs. Depuis ton PC : `scp ubuntu@IP_DU_VPS:/opt/cod4-monitor/backups/monitor-*.db .`

**Garde aussi la clé.** Les mots de passe FTP, rcon et B3 enregistrés sont chiffrés avec une clé qui n'est **pas** dans les sauvegardes : le fichier `monitor.db.key` à côté de la base (`DB_PATH` + `.key`, dans le volume `data` avec Docker), ou `MONITOR_SECRET_KEY` si tu l'as défini. Une base restaurée sans sa clé perd ces mots de passe : le monitor refuse de démarrer (`cannot decrypt server secrets`) dès qu'un serveur a un mot de passe FTP ou rcon enregistré. Copie la clé une fois, ailleurs que les sauvegardes :

```sh
docker run --rm -v cod4-monitor_data:/data alpine cat /data/monitor.db.key > monitor.db.key && chmod 600 monitor.db.key
```

**Restaurer une sauvegarde** (sur le VPS) :

```sh
cd /opt/cod4-monitor
docker compose -p cod4-monitor -f docker-compose.prod.yml stop monitor
docker run --rm -v cod4-monitor_data:/data -v "$PWD/backups":/b alpine \
  sh -c 'cp /b/monitor-2026-10-06.db /data/monitor.db && rm -f /data/monitor.db-wal /data/monitor.db-shm && chown -R 1000 /data'
docker compose -p cod4-monitor -f docker-compose.prod.yml start monitor
```

Remplace la date par celle de la sauvegarde voulue. Sur le même VPS, la clé est toujours dans le volume. Sur une nouvelle machine (ou un nouveau volume), remets la clé avant `start monitor` : copie `monitor.db.key` dans `backups/`, lance la commande ci-dessous, puis supprime-la de `backups/`.

```sh
docker run --rm -v cod4-monitor_data:/data -v "$PWD/backups":/b alpine \
  sh -c 'cp /b/monitor.db.key /data/monitor.db.key && chmod 600 /data/monitor.db.key && chown 1000 /data/monitor.db.key'
```

Ce qui est conservé pour toujours : chat, pseudos, kills, rounds, connexions, bans, journal d'activité. Les sondages et les sessions sont purgés après `RETENTION_DAYS` jours.

---

## 10. Dépannage

| Problème | Solution |
|---|---|
| `monitor` affiche **Restarting** | Regarde la cause avec `docker logs --tail 20 cod4-monitor-monitor-1`, puis voir les lignes suivantes. |
| Logs : `attempt to write a readonly database` | Les droits du dossier de données sont mauvais. Lance `docker run --rm -v cod4-monitor_data:/data alpine chown -R 1000:1000 /data`, puis `docker restart cod4-monitor-monitor-1`. |
| Logs : `invalid SERVERS entry` | La ligne `SERVERS` est mal écrite : format `Nom\|ip:port`, sans espace autour des `\|`. |
| Logs : `invalid log URL` | L'URL FTP doit commencer par `ftp://`, et les caractères spéciaux du mot de passe doivent être encodés (voir [ici](#le-format-de-servers)). |
| Le site ne s'ouvre pas / erreur de certificat | Vérifie que DuckDNS pointe bien sur l'IP du VPS, et que les ports 80 et 443 sont ouverts (pare-feu du VPS **et** de l'hébergeur). Regarde `docker logs cod4-monitor-caddy-1`. |
| Impossible de se connecter avec le compte du `.env` | `DASHBOARD_USER` est-il rempli dans le `.env` ? Après une modification, relance le site. Après 10 échecs, attends 15 minutes. |
| « Logs non configurés » dans la console | Ajoute l'URL FTP en 3e champ de `SERVERS`. |
| Pas de commandes serveur (map, messages, console) | Ajoute le mot de passe rcon (4e champ de `SERVERS`, ou Plus → Tous les serveurs → réglages du serveur), et connecte-toi en **propriétaire** du serveur, ou en modérateur avec la permission map, message ou RCON. |
| Logs : `cannot decrypt server secrets` | La base a été restaurée sans sa clé. Remets le `monitor.db.key` correspondant (ou `MONITOR_SECRET_KEY`), voir [Sauvegardes](#9-sauvegardes). |
| `fatal: not a git repository` | Tu n'es pas dans le bon dossier : `cd /opt/cod4-monitor`. |
| `permission denied` avec `docker` | Lance `sudo usermod -aG docker $USER`, puis déconnecte-toi et reconnecte-toi. |
| iPhone : le haut de page est caché | Mets le site à jour, puis ferme complètement l'application et rouvre-la. |

---

## 11. Aide-mémoire des commandes

À lancer sur le VPS, dans `/opt/cod4-monitor`. Pour raccourcir, tu peux créer un alias une fois :

```sh
echo "alias cod4='docker compose -p cod4-monitor -f /opt/cod4-monitor/docker-compose.prod.yml'" >> ~/.bashrc && source ~/.bashrc
```

| Action | Commande |
|---|---|
| État des conteneurs | `cod4 ps` |
| Logs en direct (Ctrl+C pour quitter) | `cod4 logs -f monitor` |
| Redémarrer | `cod4 restart` |
| Arrêter | `cod4 down` (la base est gardée) |
| Démarrer / appliquer un `.env` modifié | `cod4 up -d` |
| Mettre à jour | `git pull && cod4 up -d --build` |

---

## 12. Pour les développeurs

Le monitor est écrit en Go (Gin, SQLite sans CGO). L'interface (`ui/`, Svelte) est compilée par le `Dockerfile` et intégrée au binaire.

```sh
go test ./...                              # tests
go run ./cmd/probe 203.0.113.10:55555     # interroge un serveur une fois et affiche la réponse
DB_PATH=monitor.db go run ./cmd/monitor    # lance le monitor en local
```

- **Interface** : une application Svelte dans `ui/`, servie à la racine du site. Pour travailler dessus, lance le monitor (port 8080), puis :

  ```sh
  cd ui && npm ci
  npm run dev     # http://localhost:5173/ , les appels /api vont au monitor sur le port 8080
  npm run check   # vérification Svelte (aussi en CI)
  npm run build   # ui/dist, intégré au binaire par go build
  ```

  Sans `npm run build`, le site répond « Interface non compilée ». Les textes sont écrits en français ; les traductions anglaises sont dans `ui/src/lib/i18n.svelte.js` et `ui/src/lib/en/*.js` : tout nouveau texte doit y être ajouté. Les traductions allemandes sont dans `ui/src/lib/de/*.js` (à défaut, le texte anglais s'affiche).
- **Rôles** : les routes qu'un visiteur peut lire sont dans `internal/api/auth.go` (`visitorRoutes`) ; les règles des comptes et des permissions par serveur sont dans `internal/api/tenancy.go` (`tenantAllowed`).
- **Base de données** : `docker compose cp monitor:/data/monitor.db .` puis l'ouvrir avec `sqlite3`.

<details>
<summary><b>API</b> (cliquer pour afficher)</summary>

Toutes les routes `/api/servers/:id/…` acceptent `?range=24h|7d|30d|90d` ou `?from=<unix>&to=<unix>` quand elles portent sur une période.

| Route | Qui (propriétaire et super admin : tout sur leurs serveurs) | Contenu |
|---|---|---|
| `GET /api/health` | public | État du monitor (healthcheck Docker) |
| `POST /api/login` · `POST /api/logout` · `GET /api/me` | public | Connexion, compte courant et rôle |
| `GET /api/servers` | visiteur | Liste des serveurs et dernier état |
| `GET …/status` · `/history` · `/uptime` · `/maps` | visiteur | État, courbe de joueurs, disponibilité, maps |
| `GET …/players` · `/player?key=` · `/progress?key=` | visiteur | Stats joueurs (sans IP pour un visiteur), fiche, progression |
| `GET …/combat` · `/rounds` · `/records` · `/badges` · `/duel` | visiteur | Combat, rounds, records, badges, duel |
| `GET …/matches` · `/match?id=` · `/mapstats` · `/heatmap` | visiteur | Matchs, tableau des scores, rétention par map, heures actives |
| `GET /api/events` | visiteur | Flux temps réel (SSE). Un visiteur ne reçoit ni console ni chat. |
| `GET …/mapvotes` | visiteur | Votes de map (mod) |
| `GET …/maprank` · `/week` | visiteur | Comparaison des maps (Stats avancées → Maps), 7 derniers jours (accueil) |
| `GET …/maprounds` · `/retention` | visiteur | Rounds par map (durée, attaque / défense), fidélité des joueurs |
| `GET …/challenges` · `/card.png` | visiteur | Défis de la semaine, carte Discord (PNG) |
| `GET …/logs` · `/chat` · `/aliases` · `/alts` · `/errors` · `/pings` · `/cvars` · `/evasion` · `/b3/…` | `private` | Console, chat, pseudos, IP, erreurs, ping, variables, contournements, B3 |
| `GET …/notes` | `private` ou `notes` | Notes des joueurs |
| `GET …/reports` · `/inbox` · `POST …/reports/:rid/done` · `/inbox/dismiss` | `reports` (lecture : ou `private`) | Signalements, boîte de réception de la modération |
| `POST …/kick` · `/tell` · `/ban` · `/unban` · `PUT …/notes` | `kick` · `say` · `ban` · `ban` · `notes` | Actions (enregistrées dans le journal d'activité) |
| `GET …/bans` | `ban` | Bans du serveur |
| `GET …/catalog` | `maps` ou `rcon` | Maps du serveur |
| `POST …/rcon` | `rcon`, ou `maps` / `say` pour les seules commandes de map et de message | Commandes |
| `GET …/dvar` · `/rcon/log` · `…/schedules` | `rcon` | Variables rcon, historique des commandes, commandes programmées |
| `PUT …/welcome` · `PUT …/challenges` · `…/team` · `/api/serverconf/…` | propriétaire | Message de bienvenue, objectifs des défis, équipe, réglages du serveur |
| `/api/invitations` · `/api/serverconf` · `GET /api/audit` · `GET /api/observability` | compte admin | Invitations, serveurs, journal d'activité, santé de la collecte des logs (ses serveurs) |
| `/api/users` · `GET /metrics` | super admin | Comptes, format Prometheus |

</details>

### Versions publiées (GoReleaser)

Chaque **tag `v*`** poussé sur GitHub lance le workflow *Release* ([`.goreleaser.yaml`](.goreleaser.yaml)) : les tests, puis
- les binaires (interface incluse) pour Linux (amd64, arm64), Windows (amd64) et macOS (amd64, arm64), avec `checksums.txt`, sur la page [Releases](https://github.com/FloMtz2312/cod4-monitor/releases), avec les notes de version ;
- l'image Docker `ghcr.io/flomtz2312/cod4-monitor:<version>` (et `latest`), pour amd64 et arm64, avec le lecteur de démos.

```sh
git tag v1.0.0 && git push origin v1.0.0   # un tag comme v1.1.0-rc1 fait une pré-version (pas de « latest »)
goreleaser release --snapshot --clean --skip=docker   # essai en local, fichiers dans dist/
cod4-monitor -version                      # la version d'un binaire
```

Le binaire d'une version tourne sans Docker : mêmes réglages que le `.env`, en variables d'environnement (`.env.example` est dans l'archive). Le lecteur de démos n'est pas dans les archives : mets dans `DEMO_VIEWER_DIR` une copie de [cod4-demo](https://github.com/TiPSYSPiT/cod4-demo), ou utilise l'image Docker. La première fois, rends l'image publique sur GitHub (*Packages → cod4-monitor → Package settings → Change visibility*) pour que tout le monde puisse la télécharger.

---

## 13. Serveur de jeu sur la même machine et réglages avancés

Par défaut, le monitor lit les logs d'un serveur par **FTP**. Si le serveur de jeu tourne sur la **même machine** que le monitor, il peut les lire directement sur le disque : c'est plus rapide, et il n'y a pas de FTP à configurer. Peu importe comment le serveur est installé (Docker, service, à la main).

### Logs sur la même machine

1. Dans le `.env`, indique le dossier du serveur de jeu sur la machine : tout le serveur, ou seulement le dossier du mod.
   ```ini
   LOCAL_LOGS_DIR=/opt/cod4-server/cod4
   ```
2. Ajoute `-f docker-compose.local-logs.yml` à toutes tes commandes `docker compose`, et aux lignes `docker compose` de `/opt/cod4-monitor/deploy.sh` si tu utilises le [déploiement automatique](#déploiement-automatique-depuis-github) :
   ```sh
   docker compose -p cod4-monitor -f docker-compose.prod.yml -f docker-compose.local-logs.yml up -d --build
   ```
   Ce fichier monte `LOCAL_LOGS_DIR` en **lecture seule** sur `/gamelogs/` dans le monitor.
3. Dans le dashboard (avec le compte du `.env`) : **Plus → Tous les serveurs** → réglages du serveur → **Logs du serveur → Sur cette machine**, avec le dossier vu par le monitor. Par exemple, `/gamelogs/mods/mon_mod/` si tu as monté tout le serveur.
4. Clique sur **Tester**. Si le dossier est faux, le test propose ceux qui existent.

Le monitor ne peut rien lire en dehors de `/gamelogs/`.

*Sans Docker* : `LOCAL_LOGS_ROOT` fixe le dossier où le monitor a le droit de lire (par défaut `/gamelogs/`).

### Réglages avancés (par serveur)

Dans les réglages d'un serveur, le bloc **Réglages avancés** sert aux serveurs qui ne suivent pas les noms habituels :

| Réglage | Par défaut | À quoi ça sert |
|---|---|---|
| Log des parties | `games_mp.log` | Chat, kills, rounds (`g_log` du serveur) |
| Console | `qconsole.log` | Console en direct, cause des crashs (le monitor lit aussi `<console>.old`) |
| Connexions | `enterleave.log` | Arrivées et départs avec IP et GUID |
| Fuseau horaire | `SERVER_LOG_TZ` du `.env` | Horloge du serveur de jeu, si elle diffère des autres serveurs |

Un **nom** est cherché dans le dossier des logs. Il peut contenir un sous-dossier, par exemple `logs/games.log`. Un **chemin qui commence par `/`** est utilisé tel quel : chemin complet sur le FTP, ou dans `/gamelogs/` pour un serveur sur la même machine.

---

## 14. B3 : sanctions, admins et grades depuis le dashboard

Si le serveur de jeu a un bot d'administration **B3**, le dashboard lit et modifie sa base : le fichier **SQLite** de B3 (`b3.db`, sur la machine du monitor, par exemple avec le projet `cod4-server`) ou sa base **MySQL / MariaDB** (le `storage: mysql://…` de B3, n'importe où tant que le monitor peut la joindre) :

- **Modération → B3 → Sanctions** : bans, bans temporaires, kicks et avertissements, avec le joueur, la raison, l'auteur (admin en jeu, `B3` ou `site: <compte>`) et la date de fin. On peut filtrer et chercher, et lever un ban ou retirer un avertissement.
- **Modération → B3 → Admins** : qui est admin et à quel grade (20 modo, 40 admin, 60 full admin, 80 senior admin, 100 super admin), avec les sanctions données par chacun. Le grade se change dans la liste.
- **Modération → B3 → Stats** : sanctions par semaine, raisons les plus fréquentes, joueurs les plus sanctionnés, admins les plus actifs (90 jours).
- **Fiche d'un joueur → Modération → B3** : son grade, ses sanctions, et les boutons **Avertir**, **Kick**, **Ban temporaire**, **Ban**, **Lever le ban** et **Grade B3**.
- **Modération → B3 → Recherche** : recherche par pseudo (actuel ou ancien), IP (ou son début, ex. `82.64.`) ou GUID, comme `!lookup` : grade, connexions, première et dernière visite, IP, banni ou non.
- **Modération → Contournements** : aussi les joueurs non bannis venus, après un ban en cours, depuis une IP du joueur banni (souvent la même personne avec un autre GUID), avec un bouton pour les bannir aussi.
- **Origine des sanctions** : automatique (plugins de B3 : insultes, spam, trop d'avertissements), par un admin en jeu, ou depuis le site, avec le mot-clé de B3 (`#cuss`, `#spam`…).
- **Fiche d'un joueur** : aussi les pseudos et les IP connus de B3, les comptes qui partagent une de ses IP (bannis signalés), son grade masqué (`!mask`) et son message d'accueil B3.
- **Alertes** : chaque nouveau ban ou ban temporaire (en jeu ou depuis le site) apparaît dans la boîte de réception de la modération, en notification push, et par mail si les mails sont configurés.

Comment les actions s'appliquent :

| Action | Effet |
|---|---|
| Avertir | Enregistré dans B3 (compté par B3 pour ses kicks / bans automatiques) + message privé au joueur s'il est en jeu |
| Kick | Expulsé tout de suite (rcon) + noté dans l'historique B3 |
| Ban / ban temporaire | Enregistré dans B3 : le joueur est expulsé tout de suite s'il est en jeu, et B3 le refuse à chaque connexion |
| Lever le ban | Les bans B3 du joueur sont levés, et le ban du serveur aussi (`unban`) |
| Grade | Écrit dans B3 ; il s'applique à la **prochaine connexion** du joueur |

Seul le propriétaire du serveur (et le compte du `.env`) peut agir. Les modérateurs qui voient les données privées voient les sanctions sans pouvoir les modifier.

### Mise en place avec MySQL / MariaDB (tout propriétaire de serveur)

1. Trouve la base de B3 dans son `b3.ini` : `storage: mysql://utilisateur:motdepasse@hôte/base`.
2. Vérifie que le monitor peut la joindre : MySQL doit écouter sur une adresse que la machine du monitor peut atteindre (pas seulement `127.0.0.1`), et son pare-feu doit laisser passer le monitor. Le mieux est un compte MySQL dédié au monitor, par exemple :
   ```sql
   CREATE USER 'monitor'@'<IP du monitor>' IDENTIFIED BY '<mot de passe>';
   GRANT SELECT, INSERT, UPDATE ON b3.* TO 'monitor'@'<IP du monitor>';
   ```
   (`SELECT` suffit pour tout voir ; `INSERT` et `UPDATE` servent aux actions : sanctions, levée de ban, grades.)
3. Dans le dashboard : **Plus → Tous les serveurs → réglages du serveur → B3 → MySQL / MariaDB** : hôte, port, base, utilisateur, mot de passe, connexion chiffrée si le serveur la gère. **Tester** affiche le nombre d'admins et de sanctions trouvés.

*MariaDB à côté de B3 sur la même machine* (le `docker-compose.mariadb.yml` du projet `cod4-server`) : avec `docker-compose.b3.yml`, le monitor voit le dossier de données de B3, où MariaDB met son socket. Dans le champ hôte, mets le chemin du socket, `/b3/mysqld/mysqld.sock` : aucun port réseau n'est ouvert (seul le compte du `.env` peut le régler).

Le mot de passe est stocké chiffré, comme le mot de passe rcon, et n'est jamais renvoyé au navigateur. MySQL 5.7+, MySQL 8 et MariaDB 10.2+ sont gérés. Par sécurité, un propriétaire qui n'est pas le compte du `.env` ne peut pas diriger le monitor vers une adresse locale ou privée (`127.0.0.1`, `10.x`, `192.168.x`…) : seul le compte du `.env` le peut, pour un B3 sur la machine ou le réseau du monitor.

### Mise en place avec le fichier SQLite de B3 (le compte du `.env`)

1. Dans le `.env` du dashboard, indique le dossier de B3 qui contient `b3.db` :
   ```ini
   B3_DATA_DIR=/opt/cod4-server/b3data
   ```
2. Ajoute `-f docker-compose.b3.yml` à toutes tes commandes `docker compose` (en plus de `docker-compose.local-logs.yml` si tu l'utilises), et aux lignes `docker compose` de `/opt/cod4-monitor/deploy.sh` si tu utilises le [déploiement automatique](#déploiement-automatique-depuis-github) :
   ```sh
   docker compose -p cod4-monitor -f docker-compose.prod.yml -f docker-compose.b3.yml up -d --build
   ```
   Ce fichier monte le dossier de B3 en lecture/écriture sur `/b3/`. Le monitor ne peut ouvrir une base B3 que dans ce dossier.
3. Dans le dashboard : **Plus → Tous les serveurs → réglages du serveur → B3 → Fichier SQLite sur cette machine** : `/b3/b3.db`. **Tester** affiche le nombre d'admins et de sanctions trouvés.

*Sans Docker* : `B3_ROOT` fixe le dossier où le monitor a le droit d'ouvrir une base B3 (par défaut `/b3/`).

*Développeurs* : `B3_TEST_MYSQL=mysql://user:pass@127.0.0.1:3306/db go test ./internal/b3/` lance aussi les tests B3 sur MySQL (la CI le fait avec MySQL 8). La base est vidée.


## 15. Démos du serveur

Le monitor peut faire enregistrer au serveur de jeu une **démo de chaque joueur** (commande `record` de CoD4X : un fichier par joueur et par map, vu par ses yeux), et les ouvrir dans le navigateur avec le lecteur [cod4-demo](https://github.com/TiPSYSPiT/cod4-demo) : rien à télécharger ni à importer.

- **Activer** : **Plus → Tous les serveurs → réglages du serveur → Démos** (le propriétaire du serveur ; il faut le mot de passe rcon). Dès qu'il y a au moins **N vrais joueurs** (2 par défaut), le monitor envoie `record <slot>` pour chacun ; les bots ne sont jamais enregistrés. Sous le seuil, ou quand l'option est désactivée, il envoie `stoprecord all`.
- **Regarder** : **Modération → Démos** liste les démos (équipe avec les données privées), avec une liste déroulante pour en ouvrir une directement dans le lecteur, un bouton de téléchargement et **Garder**. Le bloc **Démos** de la fiche d'un joueur (onglet Modération) montre ses dernières.
- **Liées à la modération** : les démos d'un joueur signalé, aux stats suspectes, surveillé, qui ressemble à un banni ou qui est banni (B3 ou le site), de 30 minutes avant à 10 minutes après, sont **gardées 90 jours** et s'ouvrent en un clic depuis la carte de la boîte de réception.
- **Durée de conservation** : les autres démos sont supprimées après la durée choisie (48 h par défaut). Si le disque d'un serveur sur la même machine passe sous 10 % libres, les plus vieilles non gardées partent en premier.

**Combien de place ?** Les réglages affichent une estimation pendant la saisie :

> place par jour = joueurs en moyenne × heures par jour à N joueurs ou plus × Mo par joueur et par heure
> durée possible = place disponible ÷ place par jour

L'affluence vient des 30 derniers jours du serveur. Une démo pèse environ **45 Mo par joueur et par heure** tant que le monitor n'a pas mesuré les tiennes (dès 3 démos). La place disponible est l'espace libre du disque (20 % gardés en réserve) pour un serveur sur la même machine, ou la place que tu réserves aux démos (réglages avancés) pour un serveur en FTP. *Exemple* : 8 joueurs, 6 h par jour → 8 × 6 × 45 Mo ≈ 2,2 Go par jour ; avec 20 Go, environ 9 jours.

**Où sont les fichiers** : CoD4X les écrit dans `demos/` de son dossier personnel (`fs_homepath`), par défaut `../../demos/` depuis le dossier des logs (`mods/<mod>/`). À changer dans les réglages avancés si tes logs sont ailleurs.

- *FTP* : le compte FTP doit pouvoir lire et supprimer les fichiers de ce dossier.
- *Même machine* ([section 13](#13-serveur-de-jeu-sur-la-même-machine-et-réglages-avancés)) : le dossier du jeu est monté en lecture seule, le monitor ne peut donc pas supprimer les vieilles démos. Ajoute `docker-compose.local-demos.yml` pour monter le dossier des démos en lecture-écriture, après l'avoir créé pour l'utilisateur du serveur de jeu :
  ```sh
  mkdir -p /opt/cod4-server/cod4/demos && chown 1000:1000 /opt/cod4-server/cod4/demos
  docker compose -p cod4-monitor -f docker-compose.prod.yml -f docker-compose.local-logs.yml -f docker-compose.local-demos.yml up -d --build
  ```

**Le lecteur** est inclus dans l'image Docker (dossier `/demo-viewer`, `DEMO_VIEWER_DIR`). La géométrie des maps (119 Mo) n'est pas incluse ; construis avec `--build-arg DEMO_GEOMETRY=1` pour l'ajouter. Sans Docker, mets dans `DEMO_VIEWER_DIR` une copie de [cod4-demo](https://github.com/TiPSYSPiT/cod4-demo). Le lecteur est de TiPSYSPiT, sous licence GPL-3.0.

Préviens tes joueurs que les parties sont enregistrées, par exemple dans les règles ou le message du jour.

## 16. Signaler un bug / Contribuer

- **Un bug ou une idée** : [ouvre une issue](https://github.com/FloMtz2312/cod4-monitor/issues/new/choose) et remplis le formulaire, en français, en anglais ou en allemand. Le lien est aussi dans le dashboard : **Plus → À propos → Signaler un bug ou proposer une idée**. Ne colle jamais de mot de passe, de mot de passe rcon ni ton `.env`.
- **Une faille de sécurité** : signale-la en privé, comme expliqué dans [SECURITY.md](SECURITY.md), jamais dans une issue publique.
- **Une modification du code** : voir [CONTRIBUTING.md](CONTRIBUTING.md) (vérifications à lancer, traductions, pull request).

## Roadmap implémentée, sécurité et restauration

Voir [docs/ROADMAP.fr.md](docs/ROADMAP.fr.md) pour les fonctionnalités, les tests et les instructions de sauvegarde de la clé de chiffrement.

### Comptes et équipes par serveur

Les administrateurs et modérateurs rejoignent sur invitation. Voir [les rôles, invitations et permissions](docs/ACCOUNTS.md).

---

## Licence

Copyright © 2026 Florian Martinez.

CoD4 Monitor est un logiciel libre, sous [licence publique générale GNU Affero v3.0](LICENSE) (AGPL-3.0) : tu peux l'utiliser, le modifier et le partager, à condition de garder la mention de l'auteur et de publier tes modifications sous la même licence. Cela vaut aussi pour une version modifiée mise en ligne comme site pour d'autres : ses utilisateurs doivent pouvoir en obtenir le code source.

Les images des cartes (`ui/public/maps/`) appartiennent à Activision et aux auteurs des cartes ; elles ne sont pas couvertes par cette licence.

L'image Docker contient aussi le lecteur de démos [cod4-demo](https://github.com/TiPSYSPiT/cod4-demo) de TiPSYSPiT (GPL-3.0), sans modification (sans ses outils ni sa documentation), servi sur `/demo-viewer/`.
