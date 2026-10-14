# Roadmap cod4-monitor — mise en œuvre

🇬🇧 [English](ROADMAP.md) · 🇫🇷 **Français**

## P0 — Sécurité et intégrité

- [x] **Proxy/IP de confiance** : `TRUSTED_PROXIES`, validation de la chaîne X-Forwarded-For par Gin, HTTPS transmis seulement par un pair autorisé. Docker production réserve un réseau à Caddy/monitor et ne fait confiance qu'à Caddy (`172.30.94.2`). En exécution native, seules les adresses loopback sont autorisées par défaut.
- [x] **Sessions/secrets** : sessions de 7 jours, cookies HttpOnly et SameSite Strict, révocation lors des changements de mot de passe, révocation des sessions administrateur après changement des identifiants dans l'environnement, limite de 10 sessions par compte. PBKDF2 à 600 000 itérations pour les nouveaux mots de passe ; les anciens hachages restent acceptés. Corps HTTP limité à 1 Mio, délais de lecture HTTP, écoute loopback imposée sans authentification. FTP/RCON chiffrés avec AES-256-GCM en base et migration automatique. FTPS explicite disponible dans le formulaire. Chemins de logs locaux confinés par `os.Root`, y compris les liens symboliques.
- [x] **Reprise des logs** : événements, horloge, KE en attente et position enregistrés dans la même transaction ; les effets externes attendent le commit. Empreinte des 128 octets précédant la position pour détecter les remplacements, même plus longs. Lectures limitées à 256 Kio, fragments conservés jusqu'au saut de ligne. Tests de rotation, troncature, redémarrage, replay et erreur SQLite.
- [x] **Timestamps** : horloge relative du jeu ancrée en UTC et persistée, conservation des écarts entre événements et après une interruption du monitor. Fractions de seconde acceptées au parsing ; le schéma des événements conserve les secondes Unix. Après une remise à zéro de l'horloge serveur ou une première observation sans ancre, le temps absolu reste une estimation. `enterleave.log` conserve ses dates absolues et son fuseau configuré.

## P1 — Fiabilité et maintenance

- [x] **CI/tests** : tests unitaires et d'intégration HTTP/SQLite/logs, `go vet`, `go mod verify`, `go test -race -count=1 ./...`, compilation sans CGO. Le workflow de déploiement effectue ces vérifications avant l'appel SSH existant ; le build Docker conserve son propre contrôle.
- [x] **Observabilité** : `/api/observability` réservé à l'administrateur, tableau dans **Plus → État du serveur**, métriques Prometheus pour octets de retard, dernière lecture réussie, lignes, rejets, erreurs, rotations, fichiers manquants et dernière restauration vérifiée. Un fichier silencieux lu correctement n'est pas considéré en retard ; utiliser le backlog et la date de lecture ensemble.
- [x] **SQLite** : index par manche, joueur/période et dernière observation ; agrégats quotidiens de combat maintenus transactionnellement par trigger et reconstruits une fois à la migration. La forme récente combine les journées agrégées avec les frontières temporelles exactes. `PRAGMA optimize` au démarrage et avec la maintenance quotidienne. Test `EXPLAIN QUERY PLAN` de l'index de manches.
- [x] **Sauvegardes** : chaque nouvelle sauvegarde est copiée/restaurée dans un dossier temporaire, vérifiée par `integrity_check`, lecture des tables et déchiffrement des secrets. Publication atomique avec permissions 0600 ; rétention seulement après succès. Au démarrage, la sauvegarde du jour préexistante est également vérifiée si elle n'a pas encore été validée. Une sauvegarde corrompue n'est jamais supprimée silencieusement.

## P2 — Expérience joueurs

- [x] **Profil enrichi** : 7 derniers jours / 7 jours précédents, différence K/D avec avertissement pour petits échantillons, précision, comparaison avec un autre joueur, 10 dernières manches cliquables.
- [x] **Timeline** : événements horodatés par manche, kills/headshots/teamkills, poses/désamorçages et événements d'objectif/clutch. Ouverture des manches à la demande. Limite explicite de 5 000 événements à l'affichage.
- [x] **Cartes partageables** : PNG 1200 × 630 de joueur (7 jours) ou de match, sans IP ni GUID dans l'image. Bouton sur les deux fiches ; partage natif de fichier quand disponible, sinon téléchargement à joindre dans Discord. Aucun envoi automatique à Discord et pas de bot requis.
- [x] **Défis communautaires** : semaine du lundi 00:00 UTC au lundi suivant, kills/headshots/objectifs, progression collective, top contributeurs et records journaliers. Objectifs par défaut 1 000 / 200 / 50 ; l'administrateur règle ceux des semaines suivantes. Les cibles de la semaine en cours restent figées.

## Exploitation et restauration

Le déploiement automatique depuis GitHub reste optionnel (README, « Déploiement automatique depuis GitHub »). Aucune nouvelle variable obligatoire n'est nécessaire pour une installation avec un compte administrateur et le compose de production fourni.

1. Au premier démarrage, le monitor crée `DB_PATH.key` à côté de la base si `MONITOR_SECRET_KEY` n'est pas défini, puis chiffre les configurations existantes. Avec Docker, le fichier est conservé dans le volume `/data`.
2. Conserver une copie de cette clé dans un gestionnaire de secrets ou une sauvegarde protégée **séparée** des fichiers `.db`. Les sauvegardes SQLite seules ne permettent plus de retrouver les mots de passe FTP/RCON.
3. Pour restaurer : arrêter le monitor, restaurer la base et sa clé correspondante (ou la même valeur `MONITOR_SECRET_KEY`), retirer les anciens fichiers `-wal`/`-shm` uniquement après l'arrêt et remplacement de la base, fixer les droits à 0600 et redémarrer. Ne pas restaurer par-dessus une base ouverte.
4. Une mise à jour du mot de passe principal déconnecte les sessions administrateur à la prochaine ouverture du router. Un changement de mot de passe secondaire déconnecte immédiatement toutes ses sessions.
5. Le chiffrement de la base ne chiffre pas le protocole RCON UDP du jeu. FTPS est optionnel parce que certains hébergeurs ne le proposent pas ; il vérifie le certificat TLS du serveur.
6. En cas d'échec de restauration automatique, conserver la sauvegarde signalée pour diagnostic, la déplacer hors du nom du jour, puis laisser le monitor réessayer. Les anciennes sauvegardes restent présentes tant qu'une nouvelle sauvegarde n'a pas été validée.
7. Les anciennes copies de la base réalisées avant cette migration peuvent encore contenir des secrets en clair. Les traiter comme l'ancien fichier `.env`.

### Vérification locale

```sh
go mod verify
go vet ./...
go test -race -count=1 ./...
CGO_ENABLED=0 go build -trimpath -o monitor ./cmd/monitor
```

Les tests utilisent des dossiers temporaires et des données synthétiques, sans accès au serveur COD4 réel.
