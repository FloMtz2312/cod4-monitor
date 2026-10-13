// Moderation inbox of the selected server (staff): what is left to handle, for the badge of the tab and the bell.
import { get, post } from './api.js';

export const inbox = $state({ serverId: null, items: null, b3: false, private: false });

export async function loadInbox(serverId) {
  try {
    const r = await get(`/api/servers/${serverId}/inbox`);
    inbox.serverId = serverId;
    inbox.items = r.items ?? [];
    inbox.b3 = !!r.b3;
    inbox.private = !!r.private;
  } catch {
    if (inbox.serverId !== serverId) { inbox.serverId = serverId; inbox.items = null; }
  }
}

/** Classifies an item for the whole team (a report is marked handled). */
export async function dismiss(serverId, id) {
  await post(`/api/servers/${serverId}/inbox/dismiss`, { id });
  if (inbox.items) inbox.items = inbox.items.filter(it => it.id !== id);
}

export const inboxCount = serverId => (inbox.serverId === serverId ? inbox.items?.length ?? 0 : 0);
