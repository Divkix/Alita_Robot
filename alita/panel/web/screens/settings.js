import { api } from "../api.js";
import { clear, showError, textElement } from "../ui.js";

const readOnlyMessages = {
	not_admin: "You are not an admin in this chat, so settings are read-only.",
	bot_not_admin: "I am not an admin in this chat, so settings are read-only.",
};

function labelFor(key) {
	const words = key.replaceAll("_", " ");
	return words.charAt(0).toUpperCase() + words.slice(1);
}

function formatValue(field, value) {
	if (field.type === "bool") {
		return value ? "on" : "off";
	}
	if (field.type === "list") {
		return value.length > 0 ? value.join(", ") : "none";
	}
	return String(value);
}

function sameValue(left, right) {
	if (Array.isArray(left) && Array.isArray(right)) {
		return left.length === right.length && left.every((value, index) => value === right[index]);
	}
	return left === right;
}

function readControl(field, control) {
	if (field.type === "bool") {
		return control.checked;
	}
	if (field.type === "int") {
		if (control.value === "") {
			return "";
		}
		const value = Number(control.value);
		return Number.isInteger(value) ? value : control.value;
	}
	return control.value;
}

function buildControl(field, disabled) {
	if (field.type === "list") {
		const list = document.createElement("span");
		list.className = "setting-list";
		list.textContent = formatValue(field, field.value ?? []);
		return list;
	}
	if (field.type === "bool") {
		const checkbox = document.createElement("input");
		checkbox.type = "checkbox";
		checkbox.checked = Boolean(field.value);
		checkbox.disabled = disabled;
		return checkbox;
	}
	if (field.options && field.options.length > 0) {
		const select = document.createElement("select");
		for (const option of field.options) {
			const item = document.createElement("option");
			item.value = option;
			item.textContent = option;
			item.selected = option === field.value;
			select.append(item);
		}
		select.disabled = disabled;
		return select;
	}
	if (field.type === "int") {
		const input = document.createElement("input");
		input.type = "number";
		input.step = "1";
		input.value = String(field.value);
		input.disabled = disabled;
		return input;
	}
	if (field.writable) {
		const input = document.createElement("input");
		input.type = "text";
		input.value = String(field.value ?? "");
		input.disabled = disabled;
		return input;
	}
	const textarea = document.createElement("textarea");
	textarea.rows = 4;
	textarea.readOnly = true;
	textarea.value = String(field.value ?? "");
	return textarea;
}

// drawSettings renders the grouped form and returns its error helpers so a
// failed save can mark fields without losing what the admin typed.
function drawSettings(root, chatID, data, notice) {
	clear(root);

	const banner = textElement("p", notice ?? "", notice ? "notice" : "notice hidden");
	root.append(banner);

	if (data.read_only) {
		root.append(textElement("p", readOnlyMessages[data.read_only_reason] ?? "Settings are read-only.", "muted"));
	}

	const form = document.createElement("form");
	form.className = "settings-form";

	const changes = new Map();
	const errors = new Map();

	const save = document.createElement("button");
	save.type = "submit";
	save.className = "settings-save";
	save.textContent = "Save changes";
	save.disabled = true;

	const showErrors = (fieldErrors) => {
		for (const [key, element] of errors) {
			element.textContent = fieldErrors[key] ?? "";
		}
	};

	for (const module of data.modules) {
		const section = document.createElement("section");
		section.className = "settings-module";
		section.append(textElement("h3", module.label));

		for (const field of module.fields) {
			const key = `${module.key}.${field.key}`;
			const row = document.createElement("div");
			row.className = field.changed ? "setting-row changed" : "setting-row";

			const label = textElement("label", labelFor(field.key), "setting-name");
			const control = buildControl(field, data.read_only || !field.writable);
			if (control.tagName !== "SPAN") {
				control.id = key;
				label.htmlFor = control.id;
			}
			const meta = textElement(
				"span",
				`current: ${formatValue(field, field.value)} · default: ${formatValue(field, field.default)}`,
				"setting-meta",
			);
			const error = textElement("p", "", "field-error");

			const noteDirty = () => {
				const value = readControl(field, control);
				const dirty = !sameValue(value, field.value);
				if (dirty) {
					changes.set(key, value);
				} else {
					changes.delete(key);
				}
				row.classList.toggle("changed", dirty || field.changed);
				save.disabled = changes.size === 0;
			};
			control.addEventListener("input", noteDirty);
			control.addEventListener("change", noteDirty);

			const head = document.createElement("div");
			head.className = "setting-head";
			head.append(label, control);
			row.append(head, meta, error);
			section.append(row);

			errors.set(key, error);
		}

		form.append(section);
	}

	form.append(save);
	form.addEventListener("submit", async (event) => {
		event.preventDefault();
		if (changes.size === 0) {
			return;
		}
		const payload = {};
		for (const [key, value] of changes) {
			payload[key] = value;
		}
		save.disabled = true;
		banner.textContent = "";
		banner.classList.add("hidden");
		showErrors({});

		let result;
		try {
			result = await api(`chats/${encodeURIComponent(chatID)}/settings`, {
				method: "PATCH",
				body: { changes: payload },
			});
		} catch (error) {
			const detail = error.payload ?? {};
			if (detail.failed) {
				const notice = `Saved ${detail.saved.length} setting(s); ${Object.keys(detail.failed).length} failed.`;
				renderResult(detail, notice);
				return;
			}
			if (detail.errors) {
				showErrors(detail.errors);
				banner.textContent = error.message;
				banner.classList.remove("hidden");
				save.disabled = changes.size === 0;
				return;
			}
			banner.textContent = error.message;
			banner.classList.remove("hidden");
			save.disabled = changes.size === 0;
			return;
		}

		renderResult(result, "Settings saved.");
	});

	function renderResult(result, notice) {
		if (!result.modules) {
			return;
		}
		const next = drawSettings(
			root,
			chatID,
			{
				modules: result.modules,
				read_only: result.read_only,
				read_only_reason: result.read_only_reason,
			},
			notice,
		);
		if (result.failed) {
			next.showErrors(result.failed);
		}
	}

	root.append(form);
	return { showErrors };
}

export async function renderSettings(root, chatID) {
	clear(root);
	root.append(textElement("p", "Loading settings…", "status"));

	let data;
	try {
		data = await api(`chats/${encodeURIComponent(chatID)}/settings`);
	} catch (error) {
		showError(root, error);
		return;
	}
	drawSettings(root, chatID, data);
}
