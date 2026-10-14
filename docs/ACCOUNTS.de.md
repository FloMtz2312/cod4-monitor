# Konten, Einladungen und Server

🇬🇧 [English](ACCOUNTS.en.md) · 🇫🇷 [Français](ACCOUNTS.md) · 🇩🇪 **Deutsch**

Konten gibt es nur per Einladung. Für den ausgewählten Server findest du unter **Mehr → Team und Einladungen** das Team und die Einladungen; **Mehr → Alle Server** listet die Server auf (Einstellungen jedes Servers, **+ Server hinzufügen**).

## Rollen

- **Super-Admin**: das Konto `DASHBOARD_USER` / `DASHBOARD_PASSWORD` aus der `.env`. Verwaltet alle Server, lädt Administratoren ein, löscht Konten bzw. setzt sie zurück (**Mehr → Dashboard-Konten**) und überträgt Server. Der Zugriff auf den VPS, die globalen Metriken und die lokalen Dateien/B3 bleibt diesem Konto vorbehalten.
- **Admin**: tritt mit einer Einladung des Super-Admins bei. Fügt seine Server hinzu (werden ihm automatisch zugeordnet), richtet Adresse, FTP/FTPS, Logs und RCON ein, testet die Verbindung, schaltet die Überwachung ein/aus und verwaltet sein Team. Sieht von den anderen Servern nur die öffentlichen Statistiken.
- **Moderator**: tritt mit einer Einladung für einen Server bei. Standardmäßig keine privaten Daten und keine Aktionen. Dasselbe Konto kann auf mehrere Server mit unterschiedlichen Rechten eingeladen werden; weitere Einladungen nimmt es mit seinem bestehenden Passwort an.
- **Besucher**: ohne Konto, sieht die öffentlichen Statistiken.

## Einladungen

Wähle unter **Mehr → Team und Einladungen** den Benutzernamen, die Rolle (Moderator dieses Servers; der Super-Admin kann auch einen Administrator einladen) und, bei einem Moderator, die Berechtigungen. Kopiere den privaten Link und schick ihn der Person. Es wird keine E-Mail automatisch verschickt.

Die Person wählt ihr Passwort und meldet sich nach dem Annehmen mit dem angezeigten Benutzernamen an. Der Link läuft nach 48 Stunden ab und kann nur einmal angenommen werden. Vor der Nutzung kann er widerrufen werden. Sein Geheimnis wird nur als SHA-256-Hash gespeichert und steht im Fragment des Links, das beim Öffnen der Seite aus der Adresse entfernt wird; es landet nicht in URL-Logs.

## Berechtigungen pro Server

- **Private Daten** ansehen: eine ausdrückliche Gruppe mit IPs, Chat, Logs, Sitzungen, Notizen und Moderationsinfos. Mit dieser Gruppe funktionieren die alten Nur-Lese-Konten weiter.
- Einen Spieler kicken.
- Bannen und Entbannen, mit den eigenen Bans des Servers.
- Meldungen bearbeiten.
- Spielernotizen bearbeiten (für sich allein erlaubt das auch, die Notizen zu lesen).
- Map wechseln, nächste Map und Fast Restart.
- Nachrichten an den Server senden.
- Volle RCON-Konsole und geplante Befehle: gibt die volle Kontrolle über das Spiel; nicht nur für einen Map-Wechsel vergeben. Die Map-/Nachrichten-Shortcuts werden separat geprüft und lassen keine Befehlsinjektion zu.

Die Verwaltung der B3-Ränge und der Zugangsdaten bleibt beim Besitzer/Super-Admin. Gespeicherte Passwortfelder werden nie zurückgeschickt. Moderatoren sehen ihre Rechte unter **Mehr → Meine Rechte auf … → Meine Berechtigungen**.

Die Prüfungen laufen bei jeder Anfrage. Private Live-Ereignisse prüfen Sitzung und Berechtigungen für jedes Ereignis erneut: Wenn du einen Moderator entfernst, verliert er auch auf einem bereits offenen Stream den privaten Zugriff.

## Migration und Übertragung

Bestehende Server bleiben über einen leeren internen Besitzer mit dem `.env`-Konto verbunden; das bleibt stabil, auch wenn sich der Benutzername des Super-Admins ändert. Bestehende Nebenkonten werden Moderatoren und erhalten einmalig Lesezugriff auf die privaten Daten der bestehenden Server; auf künftigen Servern wird kein Recht automatisch vergeben.

Unter **Mehr → Team und Einladungen** kannst du Berechtigungen ändern oder entziehen. Wenn du ein Mitglied entfernst, werden auch seine offenen Einladungen für den Server widerrufen.

**Übertragen** (**Mehr → Server-Einstellungen → Besitzer**), nur für den Super-Admin, ändert den Besitzer auf ein bestehendes Admin-Konto („Super-Admin (.env)“, um ihn an den Super-Admin zurückzugeben). Die Übertragung entfernt das bisherige Team und widerruft die noch nicht angenommenen Einladungen, damit der neue Besitzer die Zugriffe ausdrücklich vergibt. Ein Besitzer kann erst gelöscht werden, wenn seine Server übertragen sind.

Das Passwort des Hauptkontos wird weiter über die `.env` verwaltet. Die anderen Konten können ihr eigenes Passwort ändern (**Mehr → Passwort**); dabei werden alle ihre Sitzungen widerrufen.
