import { api } from "../api.js";
import { clear, emptyState, initials, loading, relativeTime, setHeader, showError, textElement } from "../ui.js";

// chatRow is one tappable group: an initials avatar, the title, and either
// the group the panel was opened from or when the user was last active there.
function chatRow(chat, pinned) {
	const item = document.createElement("li");
	const link = document.createElement("a");
	link.className = "nav-row";
	link.href = `#/chats/${chat.id}`;

	const title = chat.title || String(chat.id);
	const avatar = textElement("span", initials(title), "avatar");
	avatar.setAttribute("aria-hidden", "true");
	avatar.dataset.tone = String(Math.abs(Number(chat.id)) % 6);

	const text = document.createElement("span");
	text.className = "nav-text";
	text.append(textElement("span", title, "nav-title"));
	const subtitle = pinned ? "Opened from this group" : chat.last_activity ? `Active ${relativeTime(chat.last_activity)}` : "";
	if (subtitle) {
		text.append(textElement("span", subtitle, pinned ? "nav-subtitle accent" : "nav-subtitle"));
	}

	link.append(avatar, text, textElement("span", "›", "chevron"));
	item.append(link);
	return item;
}

export async function renderChats(root) {
	setHeader("Your groups");
	clear(root);
	root.append(loading("Loading chats…"));

	let data;
	try {
		data = await api("session", { method: "POST" });
	} catch (error) {
		showError(root, error, () => renderChats(root));
		return;
	}

	clear(root);
	if (data.user?.first_name) {
		setHeader("Your groups", `Signed in as ${data.user.first_name}`);
	}

	const chats = Array.isArray(data.chats) ? data.chats : [];
	if (chats.length === 0) {
		root.append(emptyState("No groups yet", "Add me to a group, or send /connect in a private chat, then open the panel again."));
		return;
	}

	const pinnedID = data.pinned_chat_id == null ? null : String(data.pinned_chat_id);
	// The group the panel was opened from leads; the rest keep the server's
	// most-recently-active order.
	const ordered = [...chats].sort((left, right) => (String(right.id) === pinnedID) - (String(left.id) === pinnedID));

	const list = document.createElement("ul");
	list.className = "group-list";
	for (const chat of ordered) {
		list.append(chatRow(chat, String(chat.id) === pinnedID));
	}
	root.append(textElement("h2", "Groups", "section-header"), list);
	root.append(textElement("p", "Groups where I have seen you, most recently active first.", "section-footer"));
}
