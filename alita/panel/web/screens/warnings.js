// Warnings: every warned user with all their entries, expired ones labelled,
// and a remove button that runs the same repository call as /unwarn. Server
// data reaches the DOM only through textContent.
import { api } from "../api.js";
import { textElement } from "../ui.js";

function formatTime(value) {
	if (!value) {
		return "no timestamp";
	}
	const date = new Date(value);
	return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function issuerLabel(entry) {
	if (entry.anonymous) {
		return "anonymous admin";
	}
	if (entry.automatic) {
		return "automatic";
	}
	return entry.warned_by ? `by ${entry.warned_by}` : "unknown issuer";
}

export async function renderWarnings(root, chatID, chat) {
	const container = document.createElement("div");
	container.className = "warnings-screen";
	root.append(container);

	const readOnly = chat.role !== "admin" || !chat.bot_admin;
	const status = textElement("p", "", "status");
	const list = document.createElement("ul");
	list.className = "list-rows";
	container.append(status, list);
	if (readOnly) {
		container.append(textElement("p", "Read-only: only chat admins can remove a warning.", "muted"));
	}

	let loading = false;

	async function load() {
		if (loading) {
			return;
		}
		loading = true;
		status.className = "status";
		status.textContent = "Loading warnings…";
		try {
			const data = await api(`chats/${encodeURIComponent(chatID)}/warnings`);
			const users = data.users ?? [];
			list.replaceChildren();
			for (const user of users) {
				list.append(userRow(user));
			}
			status.textContent = users.length === 0
				? "No warnings in this chat."
				: `${users.length} warned ${users.length === 1 ? "member" : "members"}`;
		} catch (error) {
			status.className = "error";
			status.textContent = error?.message ?? "Could not load warnings.";
		} finally {
			loading = false;
		}
	}

	function userRow(user) {
		const item = document.createElement("li");
		item.className = "warn-user";

		const head = document.createElement("div");
		head.className = "warn-head";
		head.append(
			textElement("span", `User ${user.user_id}`, "list-name"),
			textElement("span", `${user.active} active`, "badge"),
		);
		item.append(head);

		const entries = document.createElement("ul");
		entries.className = "warn-entries";
		for (const entry of user.entries ?? []) {
			const row = document.createElement("li");
			row.className = entry.expired ? "warn-entry expired" : "warn-entry";
			row.append(textElement("span", entry.reason || "(no reason)", "warn-reason"));
			const meta = [issuerLabel(entry), formatTime(entry.warned_at), entry.expired ? "expired" : "active"];
			row.append(textElement("span", meta.join(" · "), "muted"));
			entries.append(row);
		}
		item.append(entries);

		if (!readOnly) {
			const remove = textElement("button", "Remove newest warning", "danger");
			remove.type = "button";
			remove.addEventListener("click", async () => {
				remove.disabled = true;
				try {
					const result = await api(`chats/${encodeURIComponent(chatID)}/warnings`, {
						method: "DELETE",
						body: { user_id: user.user_id },
					});
					await load();
					status.textContent = result.removed
						? `Removed one warning for user ${user.user_id}.`
						: `User ${user.user_id} has no warnings left.`;
				} catch (error) {
					remove.disabled = false;
					status.className = "error";
					status.textContent = error?.message ?? "Could not remove this warning.";
				}
			});
			item.append(remove);
		}

		return item;
	}

	await load();
}
