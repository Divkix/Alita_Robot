import { api } from "../api.js";
import { sections } from "../sections.js";
import { clear, showError, textElement } from "../ui.js";

async function loadChat(root, chatID) {
	clear(root);
	root.append(textElement("p", "Loading chat…", "status"));
	try {
		return await api(`chats/${encodeURIComponent(chatID)}`);
	} catch (error) {
		showError(root, error);
		return null;
	}
}

// renderSection renders one registered dashboard screen of a chat.
export async function renderSection(root, chatID, key) {
	const section = sections.find((entry) => entry.key === key);
	if (!section) {
		return renderChat(root, chatID);
	}
	const chat = await loadChat(root, chatID);
	if (!chat) {
		return;
	}
	clear(root);
	const back = document.createElement("a");
	back.href = `#/chats/${encodeURIComponent(chatID)}`;
	back.textContent = `← ${chat.title || String(chat.id)}`;
	const content = document.createElement("div");
	root.append(back, textElement("h2", section.label), content);
	if (section.adminOnly && chat.role !== "admin") {
		content.append(textElement("p", "Only chat admins can see this page.", "muted"));
		return;
	}
	await section.render(content, chatID, chat);
}

export async function renderChat(root, chatID) {
	const chat = await loadChat(root, chatID);
	if (!chat) {
		return;
	}

	clear(root);
	const back = document.createElement("a");
	back.href = "#/";
	back.textContent = "← All chats";
	root.append(back);

	root.append(textElement("h2", chat.title || String(chat.id)));

	const details = document.createElement("dl");

	const roleLabel = document.createElement("dt");
	roleLabel.textContent = "Your role";
	const roleValue = document.createElement("dd");
	roleValue.textContent = chat.role === "admin" ? "Admin" : "Read-only";
	details.append(roleLabel, roleValue);

	const botLabel = document.createElement("dt");
	botLabel.textContent = "Bot admin";
	const botValue = document.createElement("dd");
	botValue.textContent = chat.bot_admin ? "Yes" : "No — promote me to change settings";
	details.append(botLabel, botValue);

	root.append(details);

	if (chat.role !== "admin") {
		root.append(textElement("p", "You are not an admin in this chat, so the panel is read-only.", "muted"));
	}
	if (!chat.bot_admin) {
		root.append(textElement("p", "I am not an admin in this chat, so settings cannot be changed.", "empty"));
	}

	if (sections.length > 0) {
		const nav = document.createElement("ul");
		nav.className = "sections";
		for (const section of sections) {
			if (section.adminOnly && chat.role !== "admin") {
				continue;
			}
			const item = document.createElement("li");
			const link = document.createElement("a");
			link.href = `#/chats/${encodeURIComponent(chatID)}/${section.key}`;
			link.textContent = section.label;
			item.append(link);
			nav.append(item);
		}
		root.append(nav);
	}
}
