# Contributing / Contribuer / Mitmachen

Thanks for helping! English, French or German, all are fine. / Merci de ton aide ! En anglais, en français ou en allemand, au choix. / Danke für deine Hilfe! Auf Englisch, Französisch oder Deutsch, wie du willst.

## Report a bug or suggest an idea / Signaler un bug ou proposer une idée / Bug melden oder Idee vorschlagen

Open an [issue](https://github.com/FloMtz2312/cod4-monitor/issues/new/choose) and fill in the form. The link is also in the dashboard: *More → About → Report a bug*. Never paste passwords, rcon passwords, tokens or your `.env`.

Ouvre une [issue](https://github.com/FloMtz2312/cod4-monitor/issues/new/choose) et remplis le formulaire. Le lien est aussi dans le dashboard : *Plus → À propos → Signaler un bug*. Ne colle jamais de mot de passe, de jeton ni ton `.env`.

Öffne ein [Issue](https://github.com/FloMtz2312/cod4-monitor/issues/new/choose) und füll das Formular aus. Der Link ist auch im Dashboard: *Mehr → Über → Bug melden oder Idee vorschlagen*. Füge nie Passwörter, Rcon-Passwörter, Tokens oder deine `.env` ein.

Security issues: see [SECURITY.md](SECURITY.md). / Failles de sécurité : voir [SECURITY.md](SECURITY.md). / Sicherheitslücken: siehe [SECURITY.md](SECURITY.md).

## Change the code / Modifier le code / Code ändern

1. Open an issue first for a big change, to agree on it before you spend time on it.
2. Fork, create a branch, make your change.
3. Before the pull request, run the same checks as the CI:
   ```sh
   gofmt -l . && go vet ./... && go test ./...
   cd ui && npm ci && npm run check && npm run build
   ```
4. Every text shown in the dashboard is written in French in the code and translated in `ui/src/lib/i18n.svelte.js` (or `ui/src/lib/en/*.js`): add the English text there too. The German texts are in `ui/src/lib/de/*.js` (same keys); a text missing there is shown in English.
5. Open the pull request with what it changes and how you tested it.

The README section *For developers* explains how to run the project locally.

By contributing, you agree that your contribution is published under the project's license, [AGPL-3.0](LICENSE). / En contribuant, tu acceptes que ta contribution soit publiée sous la licence du projet, [AGPL-3.0](LICENSE). / Mit deinem Beitrag bist du einverstanden, dass er unter der Lizenz des Projekts veröffentlicht wird, [AGPL-3.0](LICENSE).
