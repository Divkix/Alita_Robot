// Warnings: admin-only pages of warned users with all their entries, expired
// ones labelled, and a remove button that runs the same repository call as
// /unwarn. Server data reaches the DOM only through textContent.
import { api } from "../api.js";
import { textElement } from "../ui.js";

const PAGE_SIZE = 50;

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
	const loadMore = textElement("button", "Load more");
	loadMore.type = "button";
	loadMore.hidden = true;
	container.append(status, list, loadMore);
	if (readOnly) {
		container.append(textElement("p", "Read-only: warnings cannot be removed in this chat.", "muted"));
	}

	const state = { cursor: 0, loading: false };

	async function loadPage(reset) {
		if (state.loading) {
			return;
		}
		state.loading = true;
		status.className = "status";
		status.textContent = "Loading warnings…";
		try {
			const params = new URLSearchParams({ limit: String(PAGE_SIZE) });
			if (!reset && state.cursor > 0) {
				params.set("cursor", String(state.cursor));
			}
			const data = await api(`chats/${encodeURIComponent(chatID)}/warnings?${params}`);
			if (reset) {
				list.replaceChildren();
				state.cursor = 0;
			}
			for (const user of data.users ?? []) {
				list.append(userRow(user));
			}
			state.cursor = data.next_cursor ?? 0;
			loadMore.hidden = !state.cursor;
			status.textContent = list.childElementCount === 0
				? "No warnings in this chat."
				: `${list.childElementCount} warned ${list.childElementCount === 1 ? "member" : "members"} shown`;
		} catch (error) {
			status.className = "error";
			status.textContent = error?.message ?? "Could not load warnings.";
		} finally {
			state.loading = false;
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
					await loadPage(true);
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

	loadMore.addEventListener("click", () => loadPage(false));

	await loadPage(true);
}
