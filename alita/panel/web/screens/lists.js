// Filters, notes and blacklists: live search, paginated rows with delete, and
// a bulk-import box with per-line results. Server data reaches the DOM only
// through textContent; replies may contain HTML or newlines verbatim.
import { api, initData } from "../api.js";
import { confirmDestructive, emptyState, errorMessage, haptic, loading as loadingStatus, onLeave, showError, textElement, toast } from "../ui.js";

const PAGE_SIZE = 100;
const SEARCH_DELAY_MS = 300;

const LISTS = {
	filters: {
		label: "Filters",
		noun: "filter",
		plural: "filters",
		endpoint: "filters",
		emptyHint: "Add one in the group with /filter <keyword> <reply>, or import a list below.",
		importHint: "One filter per line: keyword=reply, or paste an exported JSON list",
		exportName: "filters.json",
	},
	notes: {
		label: "Notes",
		noun: "note",
		plural: "notes",
		endpoint: "notes",
		emptyHint: "Save one in the group with /save <name> <text>, or import a list below.",
		importHint: "One note per line: name=reply, or paste an exported JSON list",
		exportName: "notes.json",
	},
	blacklists: {
		label: "Blacklists",
		noun: "blacklisted word",
		plural: "blacklisted words",
		endpoint: "blacklists",
		emptyHint: "Add one in the group with /addblacklist <word>, or import a list below.",
		importHint: "One word per line, or paste an exported list",
		exportName: "blacklists.txt",
	},
};

const readOnlyMessages = {
	member: "You are not an admin in this chat, so this list is read-only.",
	bot: "I am not an admin in this chat, so this list is read-only. Promote me to change it.",
};

export function renderFilters(root, chatID, chat) {
	return renderList(root, chatID, chat, "filters");
}

export function renderNotes(root, chatID, chat) {
	return renderList(root, chatID, chat, "notes");
}

export function renderBlacklists(root, chatID, chat) {
	return renderList(root, chatID, chat, "blacklists");
}

async function renderList(root, chatID, chat, kind) {
	const config = LISTS[kind];
	const container = document.createElement("div");
	container.className = "list-screen";
	root.append(container);

	const readOnly = chat.role !== "admin" || !chat.bot_admin;
	const state = { query: "", cursor: 0 };
	// request numbers each load; a response for an older one is dropped, so
	// typing fast never shows the results of an earlier search. resetting is
	// true while a first page is on its way and the list on screen is stale.
	let request = 0;
	let resetting = false;

	if (readOnly) {
		container.append(textElement("p", chat.role !== "admin" ? readOnlyMessages.member : readOnlyMessages.bot, "callout warn"));
	}

	const searchForm = document.createElement("form");
	searchForm.className = "list-controls";
	searchForm.setAttribute("role", "search");
	const searchInput = document.createElement("input");
	searchInput.type = "search";
	searchInput.placeholder = `Search ${config.label.toLowerCase()}`;
	searchInput.setAttribute("aria-label", `Search ${config.label.toLowerCase()}`);
	searchInput.enterKeyHint = "search";
	const exportButton = textElement("button", "Export");
	exportButton.type = "button";
	searchForm.append(searchInput, exportButton);

	const count = textElement("h2", "", "section-header");
	count.setAttribute("aria-live", "polite");
	const status = document.createElement("div");
	status.setAttribute("aria-live", "polite");
	const list = document.createElement("ul");
	list.className = "list-rows";

	const loadMore = textElement("button", "Load more");
	loadMore.type = "button";
	loadMore.hidden = true;

	container.append(searchForm, count, status, list, loadMore);

	let shown = 0;
	function updateCount(more) {
		count.textContent = shown === 0 ? "" : `${shown}${more ? "+" : ""} ${shown === 1 ? config.noun : config.plural}`;
		count.hidden = shown === 0;
		status.replaceChildren();
		if (shown > 0) {
			return;
		}
		status.append(
			state.query
				? emptyState(`No ${config.plural} match “${state.query}”.`)
				: emptyState(`No ${config.plural} yet`, readOnly ? null : config.emptyHint),
		);
	}

	function row(entry) {
		const item = document.createElement("li");
		item.className = "list-row";

		const text = document.createElement("div");
		text.className = "list-text";
		text.append(textElement("span", entry.name, "list-name"));
		if (entry.reply) {
			text.append(textElement("span", entry.reply, "list-reply muted"));
		}
		item.append(text);

		if (!readOnly) {
			const remove = textElement("button", "Delete", "danger");
			remove.type = "button";
			remove.setAttribute("aria-label", `Delete ${entry.name}`);
			remove.addEventListener("click", async () => {
				const confirmed = await confirmDestructive({
					title: `Delete ${config.noun}?`,
					message: `“${entry.name}” will be removed from this chat. This cannot be undone.`,
					action: "Delete",
				});
				if (!confirmed) {
					return;
				}
				remove.disabled = true;
				try {
					const result = await api(`chats/${encodeURIComponent(chatID)}/${config.endpoint}`, {
						method: "DELETE",
						body: { entries: [entry.name] },
					});
					const outcome = result.results?.[0]?.status;
					const neighbour = item.nextElementSibling ?? item.previousElementSibling;
					item.remove();
					shown -= 1;
					// The cursor is an offset; every later entry moved up one.
					if (state.cursor > 0) {
						state.cursor -= 1;
					}
					(neighbour?.querySelector("button") ?? searchInput).focus();
					if (!resetting) {
						// With more pages left, an emptied screen loads the next
						// entries rather than claiming the list is empty.
						if (shown === 0 && state.cursor) {
							loadPage(true);
						} else {
							updateCount(Boolean(state.cursor));
						}
					}
					haptic("success");
					toast(outcome === "removed" ? `Deleted “${entry.name}”` : `“${entry.name}” was already gone`);
				} catch (error) {
					remove.disabled = false;
					haptic("error");
					toast(errorMessage(error, "Could not delete this entry."), "error");
				}
			});
			item.append(remove);
		}
		return item;
	}

	async function loadPage(reset) {
		const current = ++request;
		loadMore.disabled = true;
		resetting = reset;
		if (reset) {
			status.replaceChildren(loadingStatus("Loading…"));
		}
		try {
			const params = new URLSearchParams({ limit: String(PAGE_SIZE) });
			if (state.query) {
				params.set("q", state.query);
			}
			if (!reset && state.cursor > 0) {
				params.set("cursor", String(state.cursor));
			}
			const data = await api(`chats/${encodeURIComponent(chatID)}/${config.endpoint}?${params}`, { method: "GET" });
			if (current !== request) {
				return;
			}
			if (reset) {
				list.replaceChildren();
				shown = 0;
			}
			for (const entry of data.entries ?? []) {
				list.append(row(entry));
				shown += 1;
			}
			state.cursor = data.next_cursor ?? 0;
			loadMore.hidden = !state.cursor;
			updateCount(Boolean(state.cursor));
		} catch (error) {
			if (current !== request) {
				return;
			}
			// The rows on screen belong to the previous search; showing them
			// under the new query would be wrong, and so would paging on.
			if (reset) {
				list.replaceChildren();
				shown = 0;
				state.cursor = 0;
				loadMore.hidden = true;
				count.hidden = true;
			}
			showError(status, error, () => loadPage(reset));
		} finally {
			if (current === request) {
				loadMore.disabled = false;
				resetting = false;
			}
		}
	}

	let searchTimer = 0;
	onLeave(() => clearTimeout(searchTimer));
	// search loads the typed query; force reloads it even when unchanged, so
	// Enter retries a search that failed.
	function search(force) {
		clearTimeout(searchTimer);
		const query = searchInput.value.trim();
		if (query === state.query && !force) {
			return;
		}
		state.query = query;
		loadPage(true);
	}
	searchInput.addEventListener("input", () => {
		clearTimeout(searchTimer);
		searchTimer = setTimeout(search, SEARCH_DELAY_MS);
	});
	searchForm.addEventListener("submit", (event) => {
		event.preventDefault();
		search(true);
		searchInput.blur();
	});

	loadMore.addEventListener("click", () => loadPage(false));

	exportButton.addEventListener("click", async () => {
		exportButton.disabled = true;
		try {
			const params = new URLSearchParams({ format: "text" });
			if (state.query) {
				params.set("q", state.query);
			}
			const response = await fetch(`/panel/api/v1/chats/${encodeURIComponent(chatID)}/${config.endpoint}?${params}`, {
				headers: { "X-Telegram-Init-Data": initData },
				credentials: "omit",
				cache: "no-store",
			});
			if (!response.ok) {
				throw new Error(`Export failed (${response.status})`);
			}
			const blob = await response.blob();
			const url = URL.createObjectURL(blob);
			const link = document.createElement("a");
			link.href = url;
			link.download = config.exportName;
			document.body.append(link);
			link.click();
			link.remove();
			URL.revokeObjectURL(url);
			toast(`Exported ${config.exportName}`);
		} catch (error) {
			haptic("error");
			toast(errorMessage(error, "Export failed."), "error");
		} finally {
			exportButton.disabled = false;
		}
	});

	if (!readOnly) {
		container.append(importBox(chatID, config, () => loadPage(true)));
	}
	await loadPage(true);
}

// importBox is the folded bulk-import form; it reloads the list after an
// import so new entries show.
function importBox(chatID, config, reload) {
	const details = document.createElement("details");
	details.className = "import-box";
	details.append(textElement("summary", `Import ${config.label.toLowerCase()}`));

	const textarea = document.createElement("textarea");
	textarea.rows = 5;
	textarea.placeholder = config.importHint;
	textarea.setAttribute("aria-label", config.importHint);
	const importButton = textElement("button", "Import", "primary");
	importButton.type = "button";
	const results = document.createElement("div");
	results.className = "import-results";
	// The body sits in its own box: a details element lays out its content
	// in a slot that ignores the details' own flex rules.
	const body = document.createElement("div");
	body.className = "import-body";
	body.append(textElement("p", config.importHint, "muted"), textarea, importButton, results);
	details.append(body);

	importButton.addEventListener("click", async () => {
		const raw = textarea.value.trim();
		if (!raw) {
			results.replaceChildren(textElement("p", "Paste entries first.", "error"));
			return;
		}
		let body;
		if (raw.startsWith("[")) {
			try {
				body = { entries: JSON.parse(raw) };
			} catch {
				results.replaceChildren(textElement("p", "That looks like JSON but does not parse.", "error"));
				return;
			}
		} else {
			body = { text: textarea.value };
		}
		importButton.disabled = true;
		importButton.textContent = "Importing…";
		try {
			const data = await api(`chats/${encodeURIComponent(chatID)}/${config.endpoint}`, {
				method: "POST",
				body,
			});
			const outcome = renderImportResults(results, data.results ?? []);
			haptic(outcome.invalid > 0 ? "warning" : "success");
			if (outcome.created > 0) {
				if (outcome.invalid === 0) {
					textarea.value = "";
				}
				await reload();
			}
		} catch (error) {
			haptic("error");
			results.replaceChildren(textElement("p", errorMessage(error, "Import failed."), "error"));
		} finally {
			importButton.disabled = false;
			importButton.textContent = "Import";
		}
	});
	return details;
}

const importStatusLabels = { created: "Added", duplicate: "Already there", invalid: "Not added" };

// renderImportResults shows a one-line summary and, below it, each line's
// outcome; it returns the counts by status.
function renderImportResults(container, results) {
	container.replaceChildren();
	const counts = { created: 0, duplicate: 0, invalid: 0 };
	for (const result of results) {
		counts[result.status] = (counts[result.status] ?? 0) + 1;
	}
	if (results.length === 0) {
		return counts;
	}
	const parts = [`${counts.created} added`];
	if (counts.duplicate > 0) {
		parts.push(`${counts.duplicate} already there`);
	}
	if (counts.invalid > 0) {
		parts.push(`${counts.invalid} not added`);
	}
	container.append(textElement("p", parts.join(" · "), counts.invalid > 0 ? "callout warn" : "callout ok"));

	const scroller = document.createElement("div");
	scroller.className = "table-scroll";
	const table = document.createElement("table");
	table.className = "import-table";
	const head = document.createElement("tr");
	for (const label of ["Line", "Value", "Result"]) {
		head.append(textElement("th", label));
	}
	table.append(head);
	for (const result of results) {
		const row = document.createElement("tr");
		row.className = `import-${result.status}`;
		const label = importStatusLabels[result.status] ?? result.status;
		const detail = result.error ? `${label}: ${result.error}` : label;
		row.append(textElement("td", String(result.line)), textElement("td", result.value), textElement("td", detail));
		table.append(row);
	}
	scroller.append(table);
	container.append(scroller);
	return counts;
}
