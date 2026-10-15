// Admins' chat of the selected server: whether the account is in it (its tab in the menu), its unread messages and
// mentions (the badge).
import { get } from './api.js';

export const adminChat = $state({ serverId: null, admin: false, unread: 0, mentions: 0 });

export async function loadAdminChat(serverId) {
  try {
    const r = await get(`/api/servers/${serverId}/adminchat/unread`);
    adminChat.serverId = serverId;
    adminChat.admin = !!r.admin;
    adminChat.unread = r.unread ?? 0;
    adminChat.mentions = r.mentions ?? 0;
  } catch {
    if (adminChat.serverId !== serverId) Object.assign(adminChat, { serverId, admin: false, unread: 0, mentions: 0 });
  }
}

/** The account is in the admins' chat of this server: the chat has its own tab. */
export const chatTab = serverId => adminChat.serverId === serverId && adminChat.admin;

/** Mentions of the account not read yet on this server (the badge), and unread messages. */
export const chatMentions = serverId => (adminChat.serverId === serverId ? adminChat.mentions : 0);
export const chatUnread = serverId => (adminChat.serverId === serverId ? adminChat.unread : 0);
