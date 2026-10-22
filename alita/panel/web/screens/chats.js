import { api } from "../api.js";
import { clear, showError, textElement } from "../ui.js";

export async function renderChats(root) {
	clear(root);
	root.append(textElement("p", "Loading chats…", "status"));

	let data;
	try {
		data = await api("session", { method: "POST" });
	} catch (error) {
		showError(root, error);
		return;
	}

	clear(root);
	if (data.user?.first_name) {
		root.append(textElement("p", `Signed in as ${data.user.first_name}`, "muted"));
	}

	if (!data.chats || data.chats.length === 0) {
		root.append(
			textElement(
				"p",
				"No groups yet. Add me to a group, or send /connect in a private chat, then open the panel again.",
				"empty",
			),
		);
		return;
	}

	const list = document.createElement("ul");
	list.className = "chat-list";
	for (const chat of data.chats) {
		const item = document.createElement("li");
		const link = document.createElement("a");
		link.className = "chat-link";
		link.href = `#/chats/${chat.id}`;
		link.textContent = chat.title || String(chat.id);
		item.append(link);
		if (data.pinned_chat_id !== undefined && data.pinned_chat_id !== null && String(chat.id) === String(data.pinned_chat_id)) {
			item.append(textElement("span", "Opened from this group", "muted"));
		}
		list.append(item);
	}
	root.append(list);
}
