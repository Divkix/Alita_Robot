// Moderation timeline: newest first, keyset cursor pages appended on scroll or
// with "Load more" so the reading position never moves, and filters by action
// and actor. It carries actions only, never message text.
import { api } from "../api.js";
import { textElement } from "../ui.js";

const PAGE_SIZE = 50;
const ACTIONS = ["ban", "kick", "mute", "unban"];

function formatTime(value) {
	const date = new Date(value);
	return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

export async function renderTimeline(root, chatID, chat) {
	const container = document.createElement("div");
	container.className = "timeline-screen";
	root.append(container);

	const form = document.createElement("form");
	form.className = "timeline-filters";

	const actionSelect = document.createElement("select");
	actionSelect.setAttribute("aria-label", "Filter by action");
	actionSelect.append(new Option("All actions", ""));
	for (const action of ACTIONS) {
		actionSelect.append(new Option(action, action));
	}

	const actorInput = document.createElement("input");
	actorInput.type = "number";
	actorInput.inputMode = "numeric";
	actorInput.min = "1";
	actorInput.placeholder = "Actor id";
	actorInput.setAttribute("aria-label", "Filter by actor id");

	const apply = textElement("button", "Apply");
	apply.type = "submit";
	form.append(actionSelect, actorInput, apply);

	const status = textElement("p", "", "status");
	const list = document.createElement("ol");
	list.className = "timeline";
	const loadMore = textElement("button", "Load more");
	loadMore.type = "button";
	loadMore.hidden = true;

	container.append(form, status, list, loadMore);

	const state = { cursor: "", action: "", actor: "", loading: false, done: false };

	async function loadPage(reset) {
		if (state.loading) {
			return;
		}
		state.loading = true;
		if (reset) {
			state.cursor = "";
			state.done = false;
			list.replaceChildren();
		}
		status.className = "status";
		status.textContent = "Loading…";
		try {
			const params = new URLSearchParams({ limit: String(PAGE_SIZE) });
			if (state.action) {
				params.set("action", state.action);
			}
			if (state.actor) {
				params.set("actor", state.actor);
			}
			if (!reset && state.cursor) {
				params.set("cursor", state.cursor);
			}
			const data = await api(`chats/${encodeURIComponent(chatID)}/timeline?${params}`, { method: "GET" });
			for (const event of data.events ?? []) {
				list.append(eventRow(event));
			}
			state.cursor = data.next_cursor ?? "";
			state.done = !state.cursor;
			loadMore.hidden = state.done;
			status.textContent = list.childElementCount === 0
				? "No moderation events in the last 180 days."
				: `${list.childElementCount} shown`;
		} catch (error) {
			status.className = "error";
			status.textContent = error?.message ?? "Could not load the timeline.";
		} finally {
			state.loading = false;
		}
	}

	function eventRow(event) {
		const item = document.createElement("li");
		item.className = `timeline-event action-${event.action}`;

		const head = document.createElement("div");
		head.className = "timeline-head";
		head.append(
			textElement("span", `#${event.action}`, "badge"),
			textElement("span", `actor ${event.actor_id <= 0 ? "automatic" : event.actor_id}`, "list-name"),
			textElement("span", `→ ${event.target_user_id}`, "list-name"),
		);
		item.append(head);

		if (event.reason) {
			item.append(textElement("p", event.reason, "timeline-reason muted"));
		}
		item.append(textElement("span", formatTime(event.created_at), "muted"));
		return item;
	}

	form.addEventListener("submit", (event) => {
		event.preventDefault();
		state.action = actionSelect.value;
		state.actor = actorInput.value.trim();
		loadPage(true);
	});

	loadMore.addEventListener("click", () => loadPage(false));

	// Loading the next page appends rows, so the page itself does not move.
	const observer = new IntersectionObserver((entries) => {
		if (entries.some((entry) => entry.isIntersecting) && !state.done && !state.loading) {
			loadPage(false);
		}
	});
	observer.observe(loadMore);

	await loadPage(true);
}
