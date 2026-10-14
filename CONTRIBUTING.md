# Contributing / Contribuer

Thanks for helping! English or French, both are fine. / Merci de ton aide ! En anglais ou en français, au choix.

## Report a bug or suggest an idea / Signaler un bug ou proposer une idée

Open an [issue](https://github.com/FloMtz2312/cod4-monitor/issues/new/choose) and fill in the form. The link is also in the dashboard: *More → About → Report a bug*. Never paste passwords, rcon passwords, tokens or your `.env`.

Ouvre une [issue](https://github.com/FloMtz2312/cod4-monitor/issues/new/choose) et remplis le formulaire. Le lien est aussi dans le dashboard : *Plus → À propos → Signaler un bug*. Ne colle jamais de mot de passe, de jeton ni ton `.env`.

Security issues: see [SECURITY.md](SECURITY.md). / Failles de sécurité : voir [SECURITY.md](SECURITY.md).

## Change the code / Modifier le code

1. Open an issue first for a big change, to agree on it before you spend time on it.
2. Fork, create a branch, make your change.
3. Before the pull request, run the same checks as the CI:
   ```sh
   gofmt -l . && go vet ./... && go test ./...
   cd ui && npm ci && npm run check && npm run build
   ```
4. Every text shown in the dashboard is written in French in the code and translated in `ui/src/lib/i18n.svelte.js` (or `ui/src/lib/en/*.js`): add the English text there too.
5. Open the pull request with what it changes and how you tested it.

The README section *For developers* explains how to run the project locally.

By contributing, you agree that your contribution is published under the project's license, [AGPL-3.0](LICENSE). / En contribuant, tu acceptes que ta contribution soit publiée sous la licence du projet, [AGPL-3.0](LICENSE).
