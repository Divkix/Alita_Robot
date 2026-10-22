// Small DOM helpers. Everything renders with textContent so chat titles,
// usernames and filter text can never become markup.
import { telegram } from "./api.js";

// supports reports whether the Telegram client implements a Bot API version;
// outside Telegram nothing is supported and the page falls back to plain DOM.
export function supports(version) {
	return Boolean(telegram?.isVersionAtLeast?.(version));
}

// nativeBack is true when Telegram draws the back button in its own header,
// so screens skip their in-page back links.
export const nativeBack = supports("6.1");

export function clear(node) {
	while (node.firstChild) {
		node.removeChild(node.firstChild);
	}
}

export function textElement(tag, text, className) {
	const element = document.createElement(tag);
	element.textContent = text;
	if (className) {
		element.className = className;
	}
	return element;
}

// haptic plays Telegram's feedback for an outcome: "success", "error" and
// "warning" are notifications, "selection" a picker tick, anything else a
// light impact. It does nothing outside Telegram.
export function haptic(kind) {
	if (!supports("6.1")) {
		return;
	}
	const feedback = telegram.HapticFeedback;
	if (kind === "success" || kind === "error" || kind === "warning") {
		feedback.notificationOccurred(kind);
	} else if (kind === "selection") {
		feedback.selectionChanged();
	} else {
		feedback.impactOccurred("light");
	}
}

// clip shortens text to Telegram's popup limits, in code points so an emoji
// is never split.
function clip(text, limit) {
	const chars = Array.from(String(text));
	return chars.length <= limit ? chars.join("") : `${chars.slice(0, limit - 1).join("")}…`;
}

// confirmDestructive asks before an action that cannot be undone and resolves
// true only when the user picks the action. Telegram draws its own popup with
// a red action button; elsewhere the browser's confirm dialog does.
export function confirmDestructive({ title, message, action }) {
	haptic("warning");
	if (!supports("6.2")) {
		return Promise.resolve(window.confirm(`${title}\n\n${message}`));
	}
	return new Promise((resolve) => {
		telegram.showPopup(
			{
				title: clip(title, 64),
				message: clip(message, 256),
				buttons: [
					{ id: "confirm", type: "destructive", text: clip(action, 64) },
					{ type: "cancel" },
				],
			},
			(buttonID) => resolve(buttonID === "confirm"),
		);
	});
}

// setHeader shows where the user is: the screen title and, above it, the chat
// it belongs to.
export function setHeader(title, context) {
	const heading = document.getElementById("title");
	const eyebrow = document.getElementById("context");
	if (heading) {
		heading.textContent = title;
	}
	if (eyebrow) {
		eyebrow.textContent = context ?? "";
		eyebrow.hidden = !context;
	}
	document.title = context ? `${title} · ${context}` : title;
}

// backLink is the in-page way back for clients without Telegram's header
// button; with it, the header button does the job and this returns nothing.
export function backLink(href, label) {
	if (nativeBack) {
		return null;
	}
	const link = textElement("a", `‹ ${label}`, "back-link");
	link.href = href;
	return link;
}

export function loading(text) {
	const status = textElement("p", text, "status loading");
	status.setAttribute("role", "status");
	return status;
}

// friendlyMessages replaces API error codes whose server message assumes
// context the user does not have.
const friendlyMessages = {
	missing_init_data: "Open this panel from Telegram: it signs you in with your Telegram account.",
	stale_auth_date: "This session has expired. Close the panel and open it again from Telegram.",
	bad_hash: "Telegram could not confirm this session. Close the panel and open it again.",
	not_member: "You are not a member of this group, so its panel is closed to you.",
	chat_not_found: "This chat is not available. The bot may have been removed from it.",
	rate_limited: "Too many requests at once. Wait a moment and try again.",
	timeout: "Telegram took too long to answer. Try again.",
	telegram_error: "Telegram did not answer. Try again in a moment.",
};

export function errorMessage(error, fallback = "Something went wrong.") {
	return friendlyMessages[error?.code] ?? error?.message ?? fallback;
}

// showError replaces root with the error and, when retry is given, a button
// that runs it again.
export function showError(root, error, retry) {
	clear(root);
	const box = document.createElement("div");
	box.className = "error-state";
	box.setAttribute("role", "alert");
	box.append(textElement("p", errorMessage(error), "error"));
	if (retry) {
		const again = textElement("button", "Try again", "secondary");
		again.type = "button";
		again.addEventListener("click", () => retry());
		box.append(again);
	}
	root.append(box);
	haptic("error");
}

// userLabel names a user from a response's names map (keyed by decimal ID),
// falling back to the ID when the bot never stored the user.
export function userLabel(names, id) {
	const entry = names?.[String(id)];
	if (!entry) {
		return `User ${id}`;
	}
	if (entry.name && entry.username) {
		return `${entry.name} (@${entry.username})`;
	}
	return entry.name || `@${entry.username}`;
}

// initials is the avatar text for a title: the first letter of its first two
// words, or "?" when it has no letters.
export function initials(title) {
	const letters = String(title ?? "")
		.split(/\s+/)
		.map((word) => Array.from(word).find((char) => /\p{L}|\p{N}/u.test(char)))
		.filter(Boolean)
		.slice(0, 2);
	return letters.length > 0 ? letters.join("").toUpperCase() : "?";
}

// relativeTime renders a past timestamp as "5 min ago", "yesterday" or a
// date, in the viewer's locale.
export function relativeTime(value, now = Date.now()) {
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) {
		return "";
	}
	const seconds = Math.round((date.getTime() - now) / 1000);
	const format = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" });
	const steps = [
		["second", 60],
		["minute", 60],
		["hour", 24],
		["day", 7],
	];
	let amount = seconds;
	for (const [unit, size] of steps) {
		if (Math.abs(amount) < size) {
			return unit === "second" ? format.format(0, "second") : format.format(amount, unit);
		}
		amount = Math.round(amount / size);
	}
	return date.toLocaleDateString();
}

// Screen lifecycle. A screen registers cleanup to run when the user leaves
// it, and a guard that reports unsaved work; the router asks before leaving
// while the guard returns true.
let leaveHandlers = [];
let leaveGuard = null;

export function onLeave(handler) {
	leaveHandlers.push(handler);
}

export function setLeaveGuard(guard) {
	leaveGuard = guard;
}

export function blocksLeave() {
	return Boolean(leaveGuard?.());
}

export function leaveScreen() {
	const handlers = leaveHandlers;
	leaveHandlers = [];
	leaveGuard = null;
	for (const handler of handlers) {
		handler();
	}
}

// toast shows a short outcome near the bottom of the screen for a few
// seconds; kind "error" colours it as a failure.
let toastTimer = 0;
export function toast(text, kind) {
	let node = document.getElementById("toast");
	if (!node) {
		node = document.createElement("div");
		node.id = "toast";
		node.setAttribute("role", "status");
		document.body.append(node);
	}
	node.textContent = text;
	node.className = kind === "error" ? "toast error-toast" : "toast";
	node.hidden = false;
	clearTimeout(toastTimer);
	toastTimer = setTimeout(() => {
		node.hidden = true;
	}, 2600);
}
