# Comptes, invitations et serveurs

L'interface et la navigation existantes sont conservées. La section Admin contient les invitations, les serveurs du propriétaire et leurs équipes.

## Rôles

- **Super admin** : compte `DASHBOARD_USER` / `DASHBOARD_PASSWORD` du `.env`. Gère tous les serveurs, invite les administrateurs, supprime/réinitialise les comptes et transfère les serveurs. Les accès au VPS, aux métriques globales et aux fichiers locaux/B3 restent réservés à ce compte.
- **Admin** : rejoint avec une invitation du super admin. Ajoute ses serveurs (rattachement automatique), configure adresse, FTP/FTPS, logs et RCON, teste la connexion, active/arrête la surveillance et gère son équipe. Consulte uniquement les statistiques publiques des autres serveurs.
- **Modérateur** : rejoint avec une invitation sur un serveur. Aucun droit privé ou d'action par défaut. Un même compte peut être invité sur plusieurs serveurs avec des droits différents ; il accepte les invitations suivantes avec son mot de passe existant.
- **Visiteur** : sans compte, consulte les statistiques publiques.

## Invitations

Dans **Admin → Invitations**, choisir l'identifiant, le rôle et, pour un modérateur, le serveur et les permissions. Copier le lien privé et le transmettre au destinataire. Aucun envoi d'e-mail automatique.

Le destinataire choisit son mot de passe, puis se connecte avec l'identifiant indiqué après l'acceptation. Le lien expire après 48 heures et ne peut être accepté qu'une fois. Il peut être révoqué avant usage. Son secret n'est conservé qu'en empreinte SHA-256 et est placé dans le fragment du lien, retiré de l'adresse à l'ouverture ; il n'est pas envoyé dans les journaux d'URL.

## Permissions par serveur

- Consultation des **données privées** : un groupe explicite incluant IP, chat, logs, sessions, notes et informations de modération. Ce groupe préserve le fonctionnement des anciens comptes en lecture seule.
- Expulsion d'un joueur.
- Bannissement et débannissement natifs du serveur.
- Traitement des signalements.
- Modification des notes joueurs.
- Changement de map, map suivante et redémarrage rapide.
- Envoi de messages au serveur.
- Console RCON complète et commandes programmées : donne le contrôle total du jeu ; ne pas attribuer pour un simple changement de map. Les raccourcis map/message sont contrôlés séparément et refusent les injections de commandes.

La gestion des grades B3 et des identifiants de connexion reste au propriétaire/super admin. Les champs de mots de passe enregistrés ne sont jamais renvoyés. Les modérateurs utilisent le panneau « Mes permissions sur ce serveur » pour leurs actions déléguées.

Les contrôles sont appliqués à chaque requête. Les événements privés en direct revérifient la session et les permissions pour chaque événement : retirer un modérateur coupe aussi son accès privé sur un flux déjà ouvert.

## Migration et transfert

Les serveurs existants restent attachés au compte `.env` via un propriétaire interne vide, stable même si l'identifiant du super admin change. Les comptes secondaires existants deviennent modérateurs et reçoivent une seule fois la lecture privée sur les serveurs existants ; aucun droit n'est ajouté automatiquement sur les futurs serveurs.

**Mes serveurs → Équipe** permet de modifier ou retirer les permissions. Retirer un membre révoque aussi ses invitations en attente sur le serveur.

**Transférer**, réservé au super admin, change le propriétaire vers un compte admin existant (identifiant vide pour revenir au super admin). Le transfert retire l'équipe précédente et révoque les invitations non acceptées, afin que le nouveau propriétaire attribue les accès explicitement. Un propriétaire ne peut pas être supprimé avant le transfert de ses serveurs.

Les mots de passe du compte principal restent gérés par le `.env`. Les autres comptes peuvent changer leur propre mot de passe ; toutes leurs sessions sont alors révoquées.
