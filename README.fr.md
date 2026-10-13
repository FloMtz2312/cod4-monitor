# CoD4 Monitor

🇬🇧 [English](README.md) · 🇫🇷 **Français**

Un tableau de bord web pour suivre un serveur **Call of Duty 4 / CoD4X** : qui joue, quand, sur quelle map, avec quelles stats, et pour l'administrer depuis un navigateur ou un téléphone.

- **En direct** : état du serveur, joueurs connectés, map, console et chat.
- **Statistiques** : activité, classements, badges, records, historique des matchs, fiche de chaque joueur avec sa progression.
- **Administration** : changement de map, kick et ban, commandes programmées, journal des actions, détection des contournements de ban.
- **Accès** : trois niveaux de compte (admin, semi-admin, visiteur), en français et en anglais, utilisable comme une application sur téléphone.

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
git clone https://github.com/FlorianMartinez01/cod4-monitor.git /opt/cod4-monitor
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
- **Se connecter** (en haut à droite), avec `DASHBOARD_USER` et `DASHBOARD_PASSWORD`, te donne l'accès admin complet.
- Pour donner un accès à quelqu'un d'autre : onglet **Admin** → *Utilisateurs du dashboard* (voir [Comptes et rôles](#6-comptes-et-rôles)).

📱 **Sur téléphone** : ouvre le site, puis *Partager* → **Sur l'écran d'accueil** (iPhone) ou *menu* → **Ajouter à l'écran d'accueil** (Android). Le site s'ouvre alors comme une application.

> ℹ️ **Tu avais déjà un monitor sur ton PC ?** Arrête-le (`docker compose down`). Deux monitors sur le même serveur enverraient chacun les commandes programmées.

---

## 3. Installation sur ton PC (pour tester)

Il faut [Docker Desktop](https://www.docker.com/products/docker-desktop/). Dans le dossier du projet :

```sh
cp .env.example .env      # puis modifie au moins SERVERS
docker compose up -d
```

Ouvre ensuite http://localhost:8080 (ou le port choisi dans `HTTP_PORT`).

- Sans `DASHBOARD_PASSWORD`, aucune connexion n'est demandée, mais les commandes ne fonctionnent que depuis le PC lui-même (`localhost`).
- Pour ouvrir le site depuis un téléphone **sur le même Wi-Fi**, mets un mot de passe, puis va sur `http://IP_DU_PC:8080`. Sous Windows, autorise une fois le port dans le pare-feu (PowerShell en administrateur) :
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

> ⏰ Le jeton GitHub a une date d'expiration. Quand `git pull` refuse de se connecter, crée un nouveau jeton (voir [étape 3](#étape-3--récupérer-le-projet)), puis lance `rm ~/.git-credentials && git pull` et colle le nouveau jeton.

**Option : déployer depuis ton PC.** `sh deploy/push.sh ubuntu@IP_DU_VPS` envoie ton dossier local (`.env` compris) et ta base locale (seulement si le VPS n'en a pas encore), puis démarre le site. ⚠️ Ce script **remplace le `.env` du VPS** par celui de ton PC.

---

## 5. Configuration (`.env`)

Après toute modification du `.env`, relance avec la commande de démarrage de l'[étape 6](#étape-6--démarrer).

### L'essentiel

| Variable | Exemple | Rôle |
|---|---|---|
| `SERVERS` | `Mon Serveur\|203.0.113.10:55555` | Le ou les serveurs à suivre au **premier démarrage** (détails ci-dessous). Ensuite, ils se gèrent depuis le site |
| `DASHBOARD_USER` | `admin` | Identifiant du compte **admin**. **À remplir** : sans lui, impossible de se connecter en admin |
| `DASHBOARD_PASSWORD` | — | Mot de passe admin. **Vide = aucune connexion** : uniquement pour un usage local |
| `DOMAIN` | `mon-serveur.duckdns.org` | Adresse publique (VPS uniquement) |
| `TZ` | `Europe/Paris` | Fuseau horaire d'affichage |

### Le format de `SERVERS`

> **`SERVERS` n'est lu qu'au premier démarrage** : les serveurs sont alors copiés dans la base. Ensuite, l'admin les ajoute, modifie, teste, arrête ou reprend depuis l'onglet **Admin → Serveurs surveillés** (adresse, FTP, rcon), sans redémarrer le monitor. Une fois les serveurs dans la base, modifier `SERVERS` n'a plus d'effet. Tu peux même le laisser vide et ajouter ton premier serveur depuis le site.

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
| `POLL_INTERVAL` | `5s` | Fréquence d'interrogation du serveur (5 s minimum) |
| `QUERY_TIMEOUT` | `2s` | Délai d'attente d'une réponse |
| `OFFLINE_AFTER` | `45s` | Durée sans réponse avant d'afficher « hors ligne » (un changement de map ne répond pas pendant 20 à 40 s) |
| `LOG_POLL_INTERVAL` | `2s` | Fréquence de lecture des logs par FTP |
| `SERVER_LOG_TZ` | `UTC` | Fuseau horaire de l'horloge du serveur de jeu |
| `HTTP_PORT` | `8080` | Port du site (installation sur PC) |
| `RETENTION_DAYS` | `90` | Durée de conservation des sondages, en jours (`0` = toujours) |
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

---

## 6. Comptes et rôles

| Rôle | Qui | Voit | Peut faire |
|---|---|---|---|
| 👑 **Admin** | le compte du `.env` | tout | tout : commandes, kick, ban, map, comptes, notes |
| 🛡 **Semi-admin** | les comptes créés par l'admin | tout : console, chat, IP, stats, contournements de ban | seulement changer son mot de passe |
| 👀 **Visiteur** | n'importe qui, sans compte | les stats publiques : état, joueurs, activité, classements, matchs | rien |

- **Créer un compte semi-admin** : onglet **Admin** → *Utilisateurs du dashboard* → identifiant et mot de passe (8 caractères minimum).
- Les **visiteurs ne voient jamais** les IP, la console, le chat ni les notes d'admin. Ce blocage est fait par le serveur, pas seulement masqué dans la page.
- **Sécurité** : mots de passe chiffrés (PBKDF2-SHA256), connexion valable 30 jours, blocage de 15 minutes après 10 échecs depuis une même IP.

---

## 7. Ce que tu trouves dans le dashboard

| Onglet | Contenu |
|---|---|
| **Accueil** | État du serveur, joueurs connectés, disponibilité et coupures (avec leur cause : erreur de la console, redémarrage, arrêt normal), les 7 derniers jours (joueur de la semaine, top 5, records). Pour l'admin : commandes serveur. Pour les comptes : console et chat en direct. |
| **Activité** | Courbe du nombre de joueurs, maps jouées, heures les plus actives |
| **Maps** | Chaque map comparée à la moyenne du serveur (passages, joueurs, temps, serveur vide, départs, votes) avec un verdict : à garder / moyenne / à remplacer. Rétention par map et votes de map. Rounds par map : durée des rounds et des matchs, victoires attaque / défense (maps déséquilibrées). |
| **Joueurs** | Stats de chaque joueur. Fidélité : nouveaux joueurs, combien reviennent, évolution par semaine, réguliers qui ne viennent plus. Pour les comptes : pseudos par GUID et comptes liés par IP. |
| **Combat** | Badges de la période, records, classement, historique des matchs avec tableau des scores, statistiques suspectes, duels, rounds |
| **Admin** | Comptes, commandes programmées, alertes, contournements de ban suspects, journal des actions, bans |
| **Serveur** | Latence, ping des joueurs (par joueur et par heure), erreurs de la console, variables du serveur |

Autres fonctionnalités :

- **Fiche joueur** : clic sur un pseudo. Elle affiche son temps de jeu, ses armes, sa progression sur 12 semaines, ses némésis… Le bouton **🔗 Lien** copie une adresse qui ouvre directement sa fiche, à partager sur Discord.
- **Sur ordinateur** : menu latéral, recherche avec **Ctrl K** (joueurs, maps, pages), cloche de notifications pour les comptes (signalements à traiter, contournements de ban, crashs des dernières 24 h), score du match en direct dans la carte du serveur.
- **Langue** : bouton 🇫🇷 / 🇬🇧 en haut à droite. Par défaut, le site prend la langue du navigateur.
- **Badges** : meilleur joueur de la période dans 7 catégories. L'icône s'affiche à côté du pseudo partout sur le site.

---

## 8. Logs FTP et commandes RCON

### Logs FTP (console, chat, kills, connexions)

Avec l'URL FTP du dossier du mod en 3e champ de `SERVERS`, le monitor lit trois fichiers :

| Fichier | Ce qu'il apporte |
|---|---|
| `qconsole.log` | Console affichée en direct (rien n'est stocké) |
| `games_mp.log` | Chat, kills, rounds, précision, liens pseudo ↔ GUID |
| `enterleave.log` | Connexions et départs avec IP et GUID. Tout l'historique est importé au premier lancement. |

Sans FTP, le site fonctionne quand même, mais seulement avec les infos publiques du serveur : joueurs, scores, map.

### Commandes RCON

Avec le `rcon_password` en 4e champ de `SERVERS`, l'admin a accès à :

- **changer de map et de mode**, depuis une galerie des maps du serveur (maps stock et `usermaps/`, lues par FTP) ;
- **map suivante, redémarrage, message à tous, commande libre** ;
- **kick et ban** depuis la liste des joueurs ou leur fiche ;
- **maps du vote de fin de match et boost XP** ;
- **commandes programmées**, toutes les N minutes ou chaque jour à heure fixe.

Chaque action est enregistrée dans le **journal des actions** (onglet Admin), avec le compte qui l'a faite.

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

On les retrouve dans le classement (noscopes, quickscopes, wallbangs, plus long kill, clutchs, premier sang, % de victoires), les records et badges, les matchs (gagnant, score, faits marquants de chaque round), la fiche joueur et l'onglet Maps (votes de map). Sans ce mod, ces colonnes et sections sont masquées et tout le reste fonctionne.

### Commandes dans le chat du jeu

Avec le `rcon_password` dans `SERVERS`, le monitor répond aux joueurs en message privé (`tell`) :

| Commande | Réponse |
|---|---|
| `!stats` ou `!stats pseudo` | Kills, morts, K/D, % de headshots, noscopes, plus long kill |
| `!top` | Les 5 meilleurs tueurs des 7 derniers jours |
| `!report pseudo raison` | Signalement envoyé aux admins : il apparaît dans l'onglet **Admin**, avec une alerte en direct |
| `!me` | Un lien court à taper sur le téléphone (`ton-domaine/me/K7QPX`, ou un code sans `DOMAIN`) : il ouvre la fiche du joueur dans la nouvelle interface et l'épingle (**C'est moi**). Valable 24 heures |
| `!help` | Liste des commandes (si le serveur a B3, c'est B3 qui répond : sa liste inclut ces commandes) |

Une commande toutes les 3 secondes par joueur, un signalement par minute.

### Contournements de ban (spoofer de GUID)

Onglet **Admin** → *Contournements de ban suspects* : chaque joueur actif depuis un ban est comparé au joueur banni, et chaque indice ajoute des points. Une alerte en direct s'affiche à partir de **50**. Rien n'est banni automatiquement : c'est à toi de vérifier.

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

**Message de bienvenue** : dans la fiche d'un joueur, l'**admin** (pas le semi-admin) peut écrire un message personnel, envoyé en privé 30 secondes après chaque connexion de ce joueur. `{name}` est remplacé par son pseudo.

**Images des maps custom** : les vignettes sont dans `web/maps/`. Après avoir ajouté des maps à ton jeu, régénère-les depuis ton PC :

```sh
docker run --rm -v "$PWD":/src -v "<dossier cod4>":/game:ro -w /src golang:1.26-alpine go run ./cmd/mapimages /game web/maps
```

---

## 9. Sauvegardes

Chaque jour, une copie de la base est écrite dans `backups/monitor-AAAA-MM-JJ.db`. Les 14 dernières sont gardées (`BACKUP_KEEP`).

> ⚠️ Ces sauvegardes sont **sur le VPS lui-même**. Pense à en copier une de temps en temps ailleurs. Depuis ton PC : `scp ubuntu@IP_DU_VPS:/opt/cod4-monitor/backups/monitor-*.db .`

**Restaurer une sauvegarde** (sur le VPS) :

```sh
cd /opt/cod4-monitor
docker compose -p cod4-monitor -f docker-compose.prod.yml stop monitor
docker run --rm -v cod4-monitor_data:/data -v "$PWD/backups":/b alpine \
  sh -c 'cp /b/monitor-2026-10-06.db /data/monitor.db && rm -f /data/monitor.db-wal /data/monitor.db-shm && chown -R 1000 /data'
docker compose -p cod4-monitor -f docker-compose.prod.yml start monitor
```

Remplace la date par celle de la sauvegarde voulue.

Ce qui est conservé pour toujours : chat, pseudos, kills, rounds, connexions, bans, journal des actions. Les sondages et les sessions sont purgés après `RETENTION_DAYS` jours.

---

## 10. Dépannage

| Problème | Solution |
|---|---|
| `monitor` affiche **Restarting** | Regarde la cause avec `docker logs --tail 20 cod4-monitor-monitor-1`, puis voir les lignes suivantes. |
| Logs : `attempt to write a readonly database` | Les droits du dossier de données sont mauvais. Lance `docker run --rm -v cod4-monitor_data:/data alpine chown -R 1000:1000 /data`, puis `docker restart cod4-monitor-monitor-1`. |
| Logs : `invalid SERVERS entry` | La ligne `SERVERS` est mal écrite : format `Nom\|ip:port`, sans espace autour des `\|`. |
| Logs : `invalid log URL` | L'URL FTP doit commencer par `ftp://`, et les caractères spéciaux du mot de passe doivent être encodés (voir [ici](#le-format-de-servers)). |
| Le site ne s'ouvre pas / erreur de certificat | Vérifie que DuckDNS pointe bien sur l'IP du VPS, et que les ports 80 et 443 sont ouverts (pare-feu du VPS **et** de l'hébergeur). Regarde `docker logs cod4-monitor-caddy-1`. |
| Impossible de se connecter en admin | `DASHBOARD_USER` est-il rempli dans le `.env` ? Après une modification, relance le site. Après 10 échecs, attends 15 minutes. |
| « Logs non configurés » dans la console | Ajoute l'URL FTP en 3e champ de `SERVERS`. |
| Pas de section « Commandes serveur » | Ajoute le `rcon_password` en 4e champ de `SERVERS`, et connecte-toi en **admin**. |
| `git pull` : *Invalid username or token* | Le jeton GitHub a expiré ou est faux : `rm ~/.git-credentials`, puis `git pull` et colle un nouveau jeton. |
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

Le monitor est écrit en Go (Gin, SQLite sans CGO). La page web (`web/`) est intégrée au binaire.

```sh
go test ./...                              # tests
go run ./cmd/probe 203.0.113.10:55555     # interroge un serveur une fois et affiche la réponse
DB_PATH=monitor.db go run ./cmd/monitor    # lance le monitor en local
```

- **Traductions** : la page est écrite en français. Les traductions anglaises sont dans `web/i18n.js` : tout nouveau texte doit y être ajouté.
- **Nouvelle interface (v2, en cours)** : application Svelte dans `ui/`, servie sur `/v2/` à côté de l'interface actuelle. Le `Dockerfile` la compile et l'intègre au binaire. Pour travailler dessus, lancer le monitor (port 8080) puis :

  ```sh
  cd ui && npm ci
  npm run dev     # http://localhost:5173/v2/ , les appels /api vont au monitor sur le port 8080
  npm run check   # vérification Svelte (aussi en CI)
  npm run build   # ui/dist, intégré au binaire par go build
  ```

  Sans `npm run build`, `/v2/` répond « Interface v2 non compilée ». Les textes sont en français, traduits dans `ui/src/lib/i18n.svelte.js`.
- **Rôles** : la liste des routes accessibles à chaque rôle est dans `internal/api/auth.go` (`allowed`).
- **Base de données** : `docker compose cp monitor:/data/monitor.db .` puis l'ouvrir avec `sqlite3`.

<details>
<summary><b>API</b> (cliquer pour afficher)</summary>

Toutes les routes `/api/servers/:id/…` acceptent `?range=24h|7d|30d|90d` ou `?from=<unix>&to=<unix>` quand elles portent sur une période.

| Route | Rôle minimum | Contenu |
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
| `GET …/maprank` · `/week` | visiteur | Comparaison des maps (onglet Maps), 7 derniers jours (accueil) |
| `GET …/maprounds` · `/retention` | visiteur | Rounds par map (durée, attaque / défense), fidélité des joueurs |
| `GET …/pings` | compte | Ping des joueurs |
| `GET …/logs` · `/chat` · `/aliases` · `/alts` · `/notes` · `/errors` · `/cvars` · `/evasion` · `/reports` | semi-admin | Console, chat, pseudos, IP, notes, erreurs, variables, contournements |
| `POST …/rcon` · `/kick` · `/ban` · `/unban` · `PUT …/notes` · `PUT …/welcome` · `…/schedules` · `POST …/reports/:id/done` | admin | Actions (enregistrées dans le journal) |
| `GET …/bans` · `/dvar` · `/catalog` · `/rcon/log` · `/api/audit` · `/api/users` | admin | Bans, variables rcon, maps du serveur, historiques, comptes |
| `GET /metrics` | semi-admin | Format Prometheus |

</details>

---

## 13. Serveur de jeu sur la même machine et réglages avancés

Par défaut, le monitor lit les logs d'un serveur par **FTP**. Si le serveur de jeu tourne sur la **même machine** que le monitor, il peut les lire directement sur le disque : c'est plus rapide, et il n'y a pas de FTP à configurer. Peu importe comment le serveur est installé (Docker, service, à la main).

### Logs sur la même machine

1. Dans le `.env`, indique le dossier du serveur de jeu sur la machine : tout le serveur, ou seulement le dossier du mod.
   ```ini
   LOCAL_LOGS_DIR=/opt/cod4-server/cod4
   ```
2. Ajoute `-f docker-compose.local-logs.yml` à toutes tes commandes `docker compose`, y compris dans `deploy.sh` si tu as le déploiement automatique :
   ```sh
   docker compose -p cod4-monitor -f docker-compose.prod.yml -f docker-compose.local-logs.yml up -d --build
   ```
   Ce fichier monte `LOCAL_LOGS_DIR` en **lecture seule** sur `/gamelogs/` dans le monitor.
3. Dans le dashboard : onglet **Admin** → le serveur → **Lecture des logs : Sur cette machine**, avec le dossier vu par le monitor. Par exemple, `/gamelogs/mods/mon_mod/` si tu as monté tout le serveur.
4. Clique sur **Tester la connexion**. Si le dossier est faux, le test propose ceux qui existent.

Le monitor ne peut rien lire en dehors de `/gamelogs/`.

*Sans Docker* : `LOCAL_LOGS_ROOT` fixe le dossier où le monitor a le droit de lire (par défaut `/gamelogs/`).

### Réglages avancés (par serveur)

Dans le formulaire d'un serveur, le bloc **Réglages avancés** sert aux serveurs qui ne suivent pas les noms habituels :

| Réglage | Par défaut | À quoi ça sert |
|---|---|---|
| Log des parties | `games_mp.log` | Chat, kills, rounds (`g_log` du serveur) |
| Console | `qconsole.log` | Console en direct, cause des crashs (le monitor lit aussi `<console>.old`) |
| Connexions | `enterleave.log` | Arrivées et départs avec IP et GUID |
| Fuseau horaire | `SERVER_LOG_TZ` du `.env` | Horloge du serveur de jeu, si elle diffère des autres serveurs |

Un **nom** est cherché dans le dossier des logs. Il peut contenir un sous-dossier, par exemple `logs/games.log`. Un **chemin qui commence par `/`** est utilisé tel quel : chemin complet sur le FTP, ou dans `/gamelogs/` pour un serveur sur la même machine.

---

## 14. B3 : sanctions, admins et grades depuis le dashboard

Si le serveur de jeu a un bot d'administration **B3** sur la même machine (par exemple avec le projet `cod4-server`), le dashboard lit et modifie sa base (`b3.db`) :

- **Admin → Sanctions B3** : liste des bans, bans temporaires, kicks et avertissements, avec le joueur, la raison, l'auteur (admin en jeu, `B3` ou `site: <compte>`) et la date de fin. On peut filtrer et chercher, et lever un ban ou retirer un avertissement.
- **Admin → Admins B3** : qui est admin et à quel grade (20 modo, 40 admin, 60 full admin, 80 senior admin, 100 super admin), avec le nombre de sanctions données. Le grade se change dans la liste.
- **Admin → Modération** : graphique des sanctions par semaine, raisons les plus fréquentes, joueurs les plus sanctionnés, admins les plus actifs (90 jours).
- **Fiche d'un joueur → Gestion → B3** : son grade, ses sanctions, et les boutons **Avertir**, **Kick**, **Ban temporaire**, **Ban**, **Lever le ban** et **Grade B3**.
- **Admin → Joueurs B3** : recherche par pseudo (actuel ou ancien), IP (ou son début, ex. `82.64.`) ou GUID, comme `!lookup` : grade, connexions, première et dernière visite, IP, banni ou non.
- **Admin → Contournements de ban B3** : joueurs non bannis venus, après un ban en cours, depuis une IP du joueur banni (souvent la même personne avec un autre GUID), avec un bouton pour les bannir aussi.
- **Origine des sanctions** : automatique (plugins de B3 : insultes, spam, trop d'avertissements), par un admin en jeu, ou depuis le site, avec le mot-clé de B3 (`#cuss`, `#spam`…).
- **Fiche d'un joueur** : aussi les pseudos et les IP connus de B3, les comptes qui partagent une de ses IP (bannis signalés), son grade masqué (`!mask`) et son message d'accueil B3.
- **Alertes** : à chaque nouveau ban ou ban temporaire (en jeu ou depuis le site), une alerte sur le dashboard et un mail si les mails sont configurés.

Comment les actions s'appliquent :

| Action | Effet |
|---|---|
| Avertir | Enregistré dans B3 (compté par B3 pour ses kicks / bans automatiques) + message privé au joueur s'il est en jeu |
| Kick | Expulsé tout de suite (rcon) + noté dans l'historique B3 |
| Ban / ban temporaire | Enregistré dans B3 : le joueur est expulsé tout de suite s'il est en jeu, et B3 le refuse à chaque connexion |
| Lever le ban | Les bans B3 du joueur sont levés, et le ban du serveur aussi (`unban`) |
| Grade | Écrit dans B3 ; il s'applique à la **prochaine connexion** du joueur |

Seul le compte admin du `.env` peut agir. Les comptes semi-admin voient les sanctions sans pouvoir les modifier.

### Mise en place

1. Dans le `.env` du dashboard, indique le dossier de B3 qui contient `b3.db` :
   ```ini
   B3_DATA_DIR=/opt/cod4-server/b3data
   ```
2. Ajoute `-f docker-compose.b3.yml` à toutes tes commandes `docker compose` et à `deploy.sh` (en plus de `docker-compose.local-logs.yml` si tu l'utilises) :
   ```sh
   sed -i 's|-f docker-compose.local-logs.yml|-f docker-compose.local-logs.yml -f docker-compose.b3.yml|g' deploy.sh
   ./deploy.sh
   ```
   Ce fichier monte le dossier de B3 en lecture/écriture sur `/b3/`. Le monitor ne peut ouvrir une base B3 que dans ce dossier.
3. Dans le dashboard : **Admin → le serveur → Modifier → Réglages avancés → Base B3** : `/b3/b3.db`. **Tester la connexion** affiche le nombre d'admins et de sanctions trouvés.

*Sans Docker* : `B3_ROOT` fixe le dossier où le monitor a le droit d'ouvrir une base B3 (par défaut `/b3/`).


## Roadmap implémentée, sécurité et restauration

Voir [docs/ROADMAP.md](docs/ROADMAP.md) pour les fonctionnalités, les tests et les instructions de sauvegarde de la clé de chiffrement.

### Comptes et équipes par serveur

Les administrateurs et modérateurs rejoignent sur invitation. Voir [les rôles, invitations et permissions](docs/ACCOUNTS.md).

## Licence

Copyright © 2026 Florian Martinez.

CoD4 Monitor est un logiciel libre, sous [licence publique générale GNU Affero v3.0](LICENSE) (AGPL-3.0) : tu peux l'utiliser, le modifier et le partager, à condition de garder la mention de l'auteur et de publier tes modifications sous la même licence. Cela vaut aussi pour une version modifiée mise en ligne comme site pour d'autres : ses utilisateurs doivent pouvoir en obtenir le code source.

Les images des cartes (`ui/public/maps/`) appartiennent à Activision et aux auteurs des cartes ; elles ne sont pas couvertes par cette licence.
