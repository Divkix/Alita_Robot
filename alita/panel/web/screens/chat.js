import { api } from "../api.js";
import { sections } from "../sections.js";
import { backLink, clear, loading, setHeader, showError, textElement } from "../ui.js";

async function loadChat(root, chatID, retry) {
	clear(root);
	root.append(loading("Loading chat…"));
	try {
		return await api(`chats/${encodeURIComponent(chatID)}`);
	} catch (error) {
		showError(root, error, retry);
		return null;
	}
}

function chatTitle(chat) {
	return chat.title || String(chat.id);
}

// renderSection renders one registered dashboard screen of a chat.
export async function renderSection(root, chatID, key) {
	const section = sections.find((entry) => entry.key === key);
	if (!section) {
		return renderChat(root, chatID);
	}
	setHeader(section.label);
	const chat = await loadChat(root, chatID, () => renderSection(root, chatID, key));
	if (!chat) {
		return;
	}
	clear(root);
	setHeader(section.label, chatTitle(chat));
	const back = backLink(`#/chats/${encodeURIComponent(chatID)}`, chatTitle(chat));
	if (back) {
		root.append(back);
	}
	const content = document.createElement("div");
	content.className = "section-content";
	root.append(content);
	if (section.adminOnly && chat.role !== "admin") {
		content.append(textElement("p", "Only chat admins can see this page.", "empty"));
		return;
	}
	await section.render(content, chatID, chat);
}

// statusRow is one line of the chat's access summary: a label and a pill
// that is green when the panel can act and amber when it cannot.
function statusRow(label, value, ok) {
	const item = document.createElement("li");
	item.className = "info-row";
	item.append(textElement("span", label), textElement("span", value, ok ? "pill ok" : "pill warn"));
	return item;
}

export async function renderChat(root, chatID) {
	setHeader("Chat");
	const chat = await loadChat(root, chatID, () => renderChat(root, chatID));
	if (!chat) {
		return;
	}

	clear(root);
	setHeader(chatTitle(chat), "Dashboard");
	const back = backLink("#/", "All chats");
	if (back) {
		root.append(back);
	}

	const status = document.createElement("ul");
	status.className = "group-list";
	status.append(
		statusRow("Your role", chat.role === "admin" ? "Admin" : "Read-only", chat.role === "admin"),
		statusRow("Bot is admin", chat.bot_admin ? "Yes" : "No", chat.bot_admin),
	);
	root.append(textElement("h2", "Access", "section-header"), status);

	if (chat.role !== "admin") {
		root.append(textElement("p", "You are not an admin in this chat, so the panel is read-only.", "section-footer"));
	} else if (!chat.bot_admin) {
		root.append(
			textElement("p", "I am not an admin in this chat, so nothing can be changed. Promote me to edit settings and lists.", "callout warn"),
		);
	}

	const visible = sections.filter((section) => !section.adminOnly || chat.role === "admin");
	if (visible.length === 0) {
		return;
	}
	const nav = document.createElement("ul");
	nav.className = "group-list";
	for (const section of visible) {
		const item = document.createElement("li");
		const link = document.createElement("a");
		link.className = "nav-row";
		link.href = `#/chats/${encodeURIComponent(chatID)}/${section.key}`;
		const text = document.createElement("span");
		text.className = "nav-text";
		text.append(textElement("span", section.label, "nav-title"));
		if (section.description) {
			text.append(textElement("span", section.description, "nav-subtitle"));
		}
		link.append(text, textElement("span", "›", "chevron"));
		item.append(link);
		nav.append(item);
	}
	root.append(textElement("h2", "Manage", "section-header"), nav);
}
