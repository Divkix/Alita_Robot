import { api, telegram } from "../api.js";
import { clear, errorMessage, haptic, leaveScreen, loading, onLeave, setLeaveGuard, showError, supports, textElement, toast } from "../ui.js";

const readOnlyMessages = {
	not_admin: "You are not an admin in this chat, so settings are read-only.",
	bot_not_admin: "I am not an admin in this chat, so settings are read-only. Promote me to change them.",
};

// fieldInfo turns "<module>.<field>" keys into the words an admin knows from
// the commands: a label, an optional hint, a unit and the range the server
// accepts for numbers. A field the server adds later still renders, labelled
// from its key.
const fieldInfo = {
	"language.code": { label: "Language", hint: "The language of my replies in this chat" },
	"greetings.welcome_enabled": { label: "Welcome new members" },
	"greetings.clean_welcome": { label: "Delete the previous welcome", hint: "Keep only the latest welcome message" },
	"greetings.goodbye_enabled": { label: "Say goodbye to leaving members" },
	"greetings.clean_goodbye": { label: "Delete the previous goodbye" },
	"greetings.clean_service": { label: "Delete join and leave notices", hint: "Telegram's “joined the group” service messages" },
	"rules.rules_text": { label: "Rules", hint: "Change them with /setrules in the group" },
	"rules.private_rules": { label: "Send rules in a private chat", hint: "/rules answers with a button that opens them in PM" },
	"captcha.enabled": { label: "Captcha for new members" },
	"captcha.mode": { label: "Captcha type" },
	"captcha.timeout": { label: "Time to answer", unit: "min", hint: "1 to 10 minutes", min: 1, max: 10 },
	"captcha.action": { label: "When the captcha fails" },
	"captcha.max_attempts": { label: "Attempts allowed", hint: "1 to 10", min: 1, max: 10 },
	"antiflood.limit": { label: "Flood limit", unit: "msgs", hint: "Messages in a row before acting: 3 to 100, or 0 for off", max: 100 },
	"antiflood.action": { label: "Flood action" },
	"antiflood.delete_messages": { label: "Delete flood messages" },
	"antispam.aispam_enabled": { label: "AI spam detection" },
	"antispam.spamdb_enabled": { label: "Check joins against spam databases" },
	"antispam.spamdb_action": { label: "Action for listed members", hint: "Log only reports them and takes no action" },
	"antispam.spamdb_cas": { label: "Ask CAS (cas.chat)" },
	"antispam.spamdb_lols": { label: "Ask lols.bot", hint: "Lists more accounts; expect more false positives" },
	"antispam.spamdb_shared": { label: "Use shared bans", hint: "Users several chats removed recently" },
	"antispam.probation_enabled": { label: "New-member probation", hint: "Only members who join from now on; needs the restrict right" },
	"antispam.probation_time": { label: "Probation lasts", unit: "s", duration: true, hint: "1 hour to 30 days", min: 3600, max: 2592000 },
	"antispam.probation_messages": { label: "Messages that end probation", unit: "msgs", hint: "1 to 100 different messages", min: 1, max: 100 },
	"antispam.probation_min_length": { label: "Characters a message needs to count", unit: "chars", hint: "1 to 500", min: 1, max: 500 },
	"antispam.probation_block_links": { label: "Probation blocks links" },
	"antispam.probation_block_forwards": { label: "Probation blocks forwards" },
	"antispam.probation_block_media": { label: "Probation blocks media" },
	"antispam.probation_notice": { label: "Tell the member once why a message was removed" },
	"antiraid.raid_time": { label: "Raid mode lasts", unit: "s", duration: true },
	"antiraid.raid_action_time": { label: "Raid temp-ban length", unit: "s", duration: true },
	"antiraid.auto_threshold": { label: "Auto raid mode at", unit: "joins/min", hint: "0 turns the automatic trigger off" },
	"locks.active": { label: "Locked", hint: "Change them with /lock and /unlock in the group" },
	"log_channel.bound": { label: "Log channel connected", hint: "Set one with /setlog" },
};

function capitalize(text) {
	return text.charAt(0).toUpperCase() + text.slice(1);
}

function infoFor(moduleKey, field) {
	return fieldInfo[`${moduleKey}.${field.key}`] ?? { label: capitalize(field.key.replaceAll("_", " ")) };
}

const languageNames = {
	en: "English",
	es: "Español",
	fr: "Français",
	hi: "हिन्दी",
	id: "Bahasa Indonesia",
	pt: "Português",
	ru: "Русский",
};

function optionLabel(moduleKey, option) {
	if (moduleKey === "language") {
		return languageNames[option] ?? option;
	}
	return capitalize(option);
}

// duration renders seconds the way the antiraid commands take them: "6h",
// "1h 30m", "45s".
function duration(seconds) {
	if (!Number.isInteger(seconds) || seconds < 0) {
		return "";
	}
	if (seconds === 0) {
		return "0s";
	}
	const parts = [];
	for (const [unit, size] of [
		["d", 86400],
		["h", 3600],
		["m", 60],
		["s", 1],
	]) {
		if (seconds >= size) {
			parts.push(`${Math.floor(seconds / size)}${unit}`);
			seconds %= size;
		}
	}
	return parts.join(" ");
}

function formatValue(moduleKey, field, value) {
	if (field.type === "bool") {
		return value ? "On" : "Off";
	}
	if (field.options?.length) {
		return optionLabel(moduleKey, String(value));
	}
	if (infoFor(moduleKey, field).duration) {
		return duration(value);
	}
	if (value === "") {
		return "Empty";
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

function writeControl(field, control, value) {
	if (field.type === "bool") {
		control.checked = Boolean(value);
	} else {
		control.value = String(value ?? "");
	}
}

// buildControl returns the input for a writable field, or a read-only view
// of the value for one the panel cannot change.
function buildControl(moduleKey, field, info, disabled) {
	if (field.type === "list") {
		const list = document.createElement("span");
		list.className = "setting-chips";
		const values = Array.isArray(field.value) ? field.value : [];
		if (values.length === 0) {
			list.append(textElement("span", "None", "muted"));
		}
		for (const value of values) {
			list.append(textElement("span", value, "pill accent"));
		}
		return list;
	}
	if (field.type === "bool" && !field.writable) {
		return textElement("span", field.value ? "Yes" : "No", "muted");
	}
	if (field.type === "bool") {
		const toggle = document.createElement("input");
		toggle.type = "checkbox";
		toggle.className = "switch";
		toggle.setAttribute("role", "switch");
		toggle.checked = Boolean(field.value);
		toggle.disabled = disabled;
		return toggle;
	}
	if (field.options && field.options.length > 0) {
		const select = document.createElement("select");
		for (const option of field.options) {
			const item = new Option(optionLabel(moduleKey, option), option);
			item.selected = option === field.value;
			select.append(item);
		}
		select.disabled = disabled;
		return select;
	}
	if (field.type === "int") {
		const input = document.createElement("input");
		input.type = "number";
		input.inputMode = "numeric";
		input.step = "1";
		input.min = String(info.min ?? 0);
		if (info.max !== undefined) {
			input.max = String(info.max);
		}
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
	return textElement("p", String(field.value ?? "") || "Not set", "setting-text");
}

// drawSettings renders the grouped form and returns helpers so a failed save
// can mark fields and put back what the admin typed.
function drawSettings(root, chatID, data) {
	clear(root);
	// A focused input removed with the screen still fires change on blur, and
	// a save can answer after the user left; once the screen is left, neither
	// counts.
	let active = true;
	onLeave(() => {
		active = false;
		setClosingConfirmation(false);
		document.body.classList.remove("has-save-bar");
	});

	const readOnly = Boolean(data.read_only);
	if (readOnly) {
		root.append(textElement("p", readOnlyMessages[data.read_only_reason] ?? "Settings are read-only.", "callout warn"));
	}

	const form = document.createElement("form");
	form.className = "settings-form";
	form.noValidate = true;
	// Disabling the fieldset locks every control while a save is in flight.
	const fields = document.createElement("fieldset");
	fields.className = "settings-fields";

	const changes = new Map();
	const errors = new Map();
	// restores puts a field back to a value, by default the saved one.
	const restores = new Map();

	const bar = document.createElement("div");
	bar.className = "save-bar";
	bar.hidden = true;
	const discard = textElement("button", "Discard", "secondary");
	discard.type = "button";
	const save = textElement("button", "Save", "primary settings-save");
	save.type = "submit";
	bar.append(discard, save);

	const showErrors = (fieldErrors) => {
		for (const [key, element] of errors) {
			element.textContent = fieldErrors[key] ?? "";
		}
	};

	const syncBar = () => {
		if (!active) {
			return;
		}
		const count = changes.size;
		bar.hidden = count === 0;
		document.body.classList.toggle("has-save-bar", count > 0);
		save.textContent = count === 1 ? "Save 1 change" : `Save ${count} changes`;
		setClosingConfirmation(count > 0);
	};
	setLeaveGuard(() => changes.size > 0);

	for (const module of data.modules) {
		const section = document.createElement("section");
		section.className = "settings-module";
		section.append(textElement("h3", module.label));
		const group = document.createElement("div");
		group.className = "group-list";

		for (const field of module.fields) {
			const key = `${module.key}.${field.key}`;
			const info = infoFor(module.key, field);
			const writable = field.writable && !readOnly;
			const row = document.createElement("div");
			row.className = "setting-row";
			if (field.type === "list" || (!field.writable && field.type === "string")) {
				row.classList.add("stacked");
			}

			const control = buildControl(module.key, field, info, !writable);
			const isInput = control.tagName === "INPUT" || control.tagName === "SELECT";
			const label = textElement(isInput ? "label" : "span", info.label, "setting-name");
			if (isInput) {
				control.id = `setting-${key}`;
				label.htmlFor = control.id;
			}

			const text = document.createElement("div");
			text.className = "setting-text-block";
			text.append(label);
			if (info.hint) {
				text.append(textElement("span", info.hint, "setting-hint"));
			}

			const value = document.createElement("div");
			value.className = "setting-value";
			value.append(control);
			if (info.unit && field.type === "int") {
				value.append(textElement("span", info.unit, "muted"));
			}

			const head = document.createElement("div");
			head.className = "setting-head";
			head.append(text, value);
			row.append(head);

			const meta = document.createElement("div");
			meta.className = "setting-meta";
			const live = textElement("span", "");
			const reset = textElement("button", "Reset to default", "link-button");
			reset.type = "button";
			meta.append(live, reset);
			row.append(meta);

			const error = textElement("p", "", "field-error");
			row.append(error);
			errors.set(key, error);

			const syncMeta = () => {
				const current = isInput ? readControl(field, control) : field.value;
				const parts = [];
				if (info.duration && Number.isInteger(current)) {
					parts.push(`= ${duration(current)}`);
				}
				const atDefault = sameValue(current, field.default);
				if (!atDefault && field.writable) {
					parts.push(`Default: ${formatValue(module.key, field, field.default)}`);
				}
				live.textContent = parts.join(" · ");
				reset.hidden = !writable || atDefault;
				meta.hidden = live.textContent === "" && reset.hidden;
			};

			const noteDirty = () => {
				if (!active) {
					return;
				}
				const current = readControl(field, control);
				const dirty = !sameValue(current, field.value);
				if (dirty) {
					changes.set(key, current);
				} else {
					changes.delete(key);
				}
				row.classList.toggle("dirty", dirty);
				error.textContent = "";
				syncMeta();
				syncBar();
			};

			if (isInput && writable) {
				control.addEventListener("input", noteDirty);
				control.addEventListener("change", () => {
					if (field.type === "bool" || control.tagName === "SELECT") {
						haptic("selection");
					}
					noteDirty();
				});
				reset.addEventListener("click", () => {
					writeControl(field, control, field.default);
					haptic("selection");
					noteDirty();
				});
				restores.set(key, (next = field.value) => {
					writeControl(field, control, next);
					noteDirty();
				});
			}
			syncMeta();
			group.append(row);
		}

		section.append(group);
		fields.append(section);
	}

	form.append(fields, bar);
	discard.addEventListener("click", () => {
		for (const restore of restores.values()) {
			restore();
		}
		showErrors({});
		haptic("light");
	});

	form.addEventListener("submit", async (event) => {
		event.preventDefault();
		if (changes.size === 0) {
			return;
		}
		const payload = Object.fromEntries(changes);
		fields.disabled = true;
		save.disabled = true;
		discard.disabled = true;
		save.textContent = "Saving…";
		showErrors({});

		let result;
		try {
			result = await api(`chats/${encodeURIComponent(chatID)}/settings`, {
				method: "PATCH",
				body: { changes: payload },
			});
		} catch (error) {
			if (!active) {
				return;
			}
			const detail = error.payload ?? {};
			if (detail.failed && detail.modules) {
				haptic("warning");
				const next = redraw(detail);
				next.restore(Object.fromEntries(Object.keys(detail.failed).map((key) => [key, payload[key]])));
				next.showErrors(detail.failed);
				toast(`Saved ${detail.saved.length}; ${Object.keys(detail.failed).length} failed. The failed ones are marked.`, "error");
				return;
			}
			fields.disabled = false;
			save.disabled = false;
			discard.disabled = false;
			syncBar();
			haptic("error");
			const count = Object.keys(detail.errors ?? {}).length;
			if (count === 0) {
				toast(errorMessage(error, "Could not save the settings."), "error");
				return;
			}
			showErrors(detail.errors);
			errors.get(Object.keys(detail.errors)[0])?.closest(".setting-row")?.scrollIntoView({ block: "center", behavior: "smooth" });
			toast(count === 1 ? "Fix the marked setting and save again." : `Fix the ${count} marked settings and save again.`, "error");
			return;
		}
		if (!active) {
			return;
		}
		haptic("success");
		toast("Settings saved");
		if (result.modules) {
			redraw(result);
		}
	});

	// redraw retires this form and draws the server's new state.
	function redraw(result) {
		leaveScreen();
		return drawSettings(root, chatID, {
			modules: result.modules,
			read_only: result.read_only,
			read_only_reason: result.read_only_reason,
		});
	}

	root.append(form);
	syncBar();
	return {
		showErrors,
		restore(values) {
			for (const [key, next] of Object.entries(values)) {
				restores.get(key)?.(next);
			}
		},
	};
}

// setClosingConfirmation makes Telegram ask before closing the Mini App while
// changes are unsaved.
let closingConfirmation = false;
function setClosingConfirmation(enabled) {
	if (!supports("6.2") || enabled === closingConfirmation) {
		return;
	}
	closingConfirmation = enabled;
	if (enabled) {
		telegram.enableClosingConfirmation();
	} else {
		telegram.disableClosingConfirmation();
	}
}

export async function renderSettings(root, chatID) {
	clear(root);
	root.append(loading("Loading settings…"));
	const route = window.location.hash;

	let data;
	try {
		data = await api(`chats/${encodeURIComponent(chatID)}/settings`);
	} catch (error) {
		if (window.location.hash === route) {
			showError(root, error, () => renderSettings(root, chatID));
		}
		return;
	}
	// The user may have moved on while this loaded; drawing now would put a
	// leave guard on another screen.
	if (window.location.hash !== route) {
		return;
	}
	drawSettings(root, chatID, data);
}
