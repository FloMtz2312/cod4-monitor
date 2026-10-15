// Admins' chat of the selected server: its unread messages and mentions, for the badge of the Moderation tab.
import { get } from './api.js';

export const adminChat = $state({ serverId: null, unread: 0, mentions: 0 });

export async function loadAdminChat(serverId) {
  try {
    const r = await get(`/api/servers/${serverId}/adminchat/unread`);
    adminChat.serverId = serverId;
    adminChat.unread = r.unread ?? 0;
    adminChat.mentions = r.mentions ?? 0;
  } catch {
    if (adminChat.serverId !== serverId) Object.assign(adminChat, { serverId, unread: 0, mentions: 0 });
  }
}

/** Mentions of the account not read yet on this server (the badge), and unread messages. */
export const chatMentions = serverId => (adminChat.serverId === serverId ? adminChat.mentions : 0);
export const chatUnread = serverId => (adminChat.serverId === serverId ? adminChat.unread : 0);
