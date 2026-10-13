# Accounts, invitations and servers

🇬🇧 **English** · 🇫🇷 [Français](ACCOUNTS.md)

Accounts are created by invitation only. For the selected server, **More → Team and invitations** holds the team and the invitations; **More → All servers** lists the servers (each one's settings, **+ Add a server**).

## Roles

- **Super admin**: the `DASHBOARD_USER` / `DASHBOARD_PASSWORD` account of `.env`. Manages every server, invites administrators, deletes/resets accounts (**More → Dashboard accounts**) and transfers servers. Access to the VPS, the global metrics and the local files/B3 stays reserved to this account.
- **Admin**: joins with an invitation from the super admin. Adds their servers (attached to them automatically), sets the address, FTP/FTPS, logs and RCON, tests the connection, turns monitoring on/off and manages their team. Only sees the public stats of the other servers.
- **Moderator**: joins with an invitation on a server. No private data and no action by default. The same account can be invited on several servers with different rights; it accepts the next invitations with its existing password.
- **Visitor**: no account, sees the public stats.

## Invitations

In **More → Team and invitations**, choose the username, the role (moderator of this server; the super admin can also invite an administrator) and, for a moderator, the permissions. Copy the private link and send it to the person. No email is sent automatically.

The person chooses their password, then logs in with the username shown after accepting. The link expires after 48 hours and can be accepted only once. It can be revoked before it is used. Its secret is only stored as a SHA-256 hash and sits in the link's fragment, which is removed from the address when the page opens; it is not sent to URL logs.

## Per-server permissions

- See the **private data**: an explicit group including IPs, chat, logs, sessions, notes and moderation information. This group keeps the old read-only accounts working.
- Kick a player.
- Ban and unban, with the server's own bans.
- Handle the reports.
- Edit the player notes (on its own, it also allows reading the notes).
- Change the map, next map and fast restart.
- Send messages to the server.
- Full RCON console and scheduled commands: gives full control of the game; don't grant it just to change maps. The map/message shortcuts are checked separately and refuse command injection.

Managing B3 grades and the login credentials stays with the owner/super admin. Saved password fields are never sent back. Moderators see their rights in **More → My rights on … → My permissions**.

The checks run on every request. Private live events check the session and the permissions again for each event: removing a moderator also cuts their private access on a stream that is already open.

## Migration and transfer

Existing servers stay attached to the `.env` account through an empty internal owner, which stays stable even if the super admin's username changes. Existing secondary accounts become moderators and get private read access on the existing servers, once; no right is added automatically on future servers.

**More → Team and invitations** lets you change or remove permissions. Removing a member also revokes their pending invitations on the server.

**Transfer** (**More → Server settings → Owner**), reserved to the super admin, changes the owner to an existing admin account ("Super admin (.env)" to give it back to the super admin). The transfer removes the previous team and revokes the invitations not accepted yet, so that the new owner grants access explicitly. An owner can't be deleted before their servers are transferred.

The main account's password stays managed by `.env`. The other accounts can change their own password (**More → Password**); all their sessions are then revoked.
