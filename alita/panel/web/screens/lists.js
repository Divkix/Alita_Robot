// Filters, notes and blacklists: search, paginated rows with delete, and a
// bulk-import box with per-line results. Server data reaches the DOM only
// through textContent; replies may contain HTML or newlines verbatim.
import { api, initData } from "../api.js";
import { textElement } from "../ui.js";

const PAGE_SIZE = 100;

const LISTS = {
	filters: {
		label: "Filters",
		endpoint: "filters",
		importHint: "One filter per line: keyword=reply, or paste an exported JSON list",
		exportName: "filters.json",
	},
	notes: {
		label: "Notes",
		endpoint: "notes",
		importHint: "One note per line: name=reply, or paste an exported JSON list",
		exportName: "notes.json",
	},
	blacklists: {
		label: "Blacklists",
		endpoint: "blacklists",
		importHint: "One word per line, or paste an exported list",
		exportName: "blacklists.txt",
	},
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

	const searchForm = document.createElement("form");
	searchForm.className = "list-controls";
	const searchInput = document.createElement("input");
	searchInput.type = "search";
	searchInput.placeholder = `Search ${config.label.toLowerCase()}`;
	searchInput.setAttribute("aria-label", `Search ${config.label.toLowerCase()}`);
	const searchButton = textElement("button", "Search");
	searchButton.type = "submit";
	searchForm.append(searchInput, searchButton);

	const status = textElement("p", "", "status");
	const list = document.createElement("ul");
	list.className = "list-rows";

	const loadMore = textElement("button", "Load more");
	loadMore.type = "button";
	loadMore.hidden = true;

	const exportButton = textElement("button", "Export");
	exportButton.type = "button";

	const importHint = textElement("p", config.importHint, "muted");
	const textarea = document.createElement("textarea");
	textarea.rows = 4;
	textarea.placeholder = config.importHint;
	const importButton = textElement("button", "Import");
	importButton.type = "button";
	importButton.disabled = readOnly;

	const importResults = document.createElement("div");
	importResults.className = "import-results";

	const actions = document.createElement("div");
	actions.className = "list-actions";
	actions.append(exportButton);

	container.append(searchForm, status, list, loadMore, actions, importHint, textarea, importButton, importResults);

	let loading = false;

	function setStatus(text, isError) {
		status.textContent = text;
		status.className = isError ? "error" : "status";
	}

	function updateStatus() {
		const shown = list.childElementCount;
		setStatus(shown === 0 ? "Nothing here yet." : `${shown} shown`, false);
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
			remove.addEventListener("click", async () => {
				remove.disabled = true;
				try {
					const result = await api(`chats/${encodeURIComponent(chatID)}/${config.endpoint}`, {
						method: "DELETE",
						body: { entries: [entry.name] },
					});
					const outcome = result.results?.[0]?.status;
					item.remove();
					setStatus(outcome === "removed" ? `Deleted ${entry.name}` : `${entry.name} was already gone`, false);
				} catch (error) {
					remove.disabled = false;
					setStatus(error?.message ?? "Could not delete this entry.", true);
				}
			});
			item.append(remove);
		}
		return item;
	}

	async function loadPage(reset) {
		if (loading) {
			return;
		}
		loading = true;
		setStatus("Loading…", false);
		try {
			const params = new URLSearchParams({ limit: String(PAGE_SIZE) });
			if (state.query) {
				params.set("q", state.query);
			}
			if (!reset && state.cursor > 0) {
				params.set("cursor", String(state.cursor));
			}
			const data = await api(`chats/${encodeURIComponent(chatID)}/${config.endpoint}?${params}`, { method: "GET" });
			if (reset) {
				list.replaceChildren();
				state.cursor = 0;
			}
			for (const entry of data.entries ?? []) {
				list.append(row(entry));
			}
			state.cursor = data.next_cursor ?? 0;
			loadMore.hidden = !state.cursor;
			updateStatus();
		} catch (error) {
			setStatus(error?.message ?? "Could not load this list.", true);
		} finally {
			loading = false;
		}
	}

	searchForm.addEventListener("submit", (event) => {
		event.preventDefault();
		state.query = searchInput.value.trim();
		loadPage(true);
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
			setStatus(`Exported ${config.exportName}`, false);
		} catch (error) {
			setStatus(error?.message ?? "Export failed.", true);
		} finally {
			exportButton.disabled = false;
		}
	});

	importButton.addEventListener("click", async () => {
		const raw = textarea.value.trim();
		if (!raw) {
			importResults.replaceChildren(textElement("p", "Paste entries first.", "error"));
			return;
		}
		let body;
		if (raw.startsWith("[")) {
			try {
				body = { entries: JSON.parse(raw) };
			} catch {
				importResults.replaceChildren(textElement("p", "That looks like JSON but does not parse.", "error"));
				return;
			}
		} else {
			body = { text: textarea.value };
		}
		importButton.disabled = true;
		try {
			const data = await api(`chats/${encodeURIComponent(chatID)}/${config.endpoint}`, {
				method: "POST",
				body,
			});
			renderImportResults(importResults, data.results ?? []);
			await loadPage(true);
		} catch (error) {
			importResults.replaceChildren(textElement("p", error?.message ?? "Import failed.", "error"));
		} finally {
			importButton.disabled = readOnly;
		}
	});

	if (readOnly) {
		textarea.disabled = true;
	}
	await loadPage(true);
}

function renderImportResults(container, results) {
	container.replaceChildren();
	if (results.length === 0) {
		return;
	}
	const table = document.createElement("table");
	table.className = "import-table";
	const head = document.createElement("tr");
	for (const label of ["Line", "Value", "Status"]) {
		head.append(textElement("th", label));
	}
	table.append(head);
	for (const result of results) {
		const row = document.createElement("tr");
		row.className = `import-${result.status}`;
		const detail = result.error ? `${result.status}: ${result.error}` : result.status;
		row.append(textElement("td", String(result.line)), textElement("td", result.value), textElement("td", detail));
		table.append(row);
	}
	container.append(table);
}
