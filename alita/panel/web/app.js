import { telegram } from "./api.js";
import { renderChat, renderSection } from "./screens/chat.js";
import { renderChats } from "./screens/chats.js";
import { blocksLeave, confirmDestructive, haptic, leaveScreen, setLeaveGuard, supports } from "./ui.js";

telegram?.ready();
telegram?.expand();

// The page ground is the secondary background, with grouped sections on the
// primary one, the layout of Telegram's own settings screens. The header and
// page colours follow it so the Mini App reads as part of the client.
if (supports("6.1")) {
	telegram.setHeaderColor("secondary_bg_color");
	telegram.setBackgroundColor("secondary_bg_color");
}

const root = document.getElementById("app");

// launchChatID reads the chat this Mini App was opened for: ?chat=<id> from
// the PM web_app button (panel.WebAppURL), or the chat_<id> start_param of a
// direct Mini App link (panel.EntryURL).
function launchChatID() {
	const queryChat = new URLSearchParams(window.location.search).get("chat");
	const startParamChat = /^chat_(-?\d+)$/.exec(telegram?.initDataUnsafe?.start_param ?? "")?.[1];
	const value = queryChat || startParamChat || "";
	return /^-?\d+$/.test(value) && value !== "0" ? value : null;
}

const routePattern = /^#\/chats\/(-?\d+)(?:\/([a-z-]+))?$/;

// parentRoute is where Back leads: a section returns to its chat, a chat to
// the chat list, and the chat list has no parent.
function parentRoute(hash) {
	const match = routePattern.exec(hash);
	if (!match) {
		return null;
	}
	return match[2] ? `#/chats/${match[1]}` : "#/";
}

function syncBackButton() {
	if (!supports("6.1")) {
		return;
	}
	if (parentRoute(window.location.hash)) {
		telegram.BackButton.show();
	} else {
		telegram.BackButton.hide();
	}
}

function render() {
	leaveScreen();
	syncBackButton();
	window.scrollTo(0, 0);
	const match = routePattern.exec(window.location.hash);
	if (match && match[2]) {
		return renderSection(root, match[1], match[2]);
	}
	if (match) {
		return renderChat(root, match[1]);
	}
	return renderChats(root);
}

// currentHash is the route on screen. A hash change away from a screen with
// unsaved work puts it back and asks first; leaving then follows the user's
// original target. Putting it back replaces the history entry, which only a
// browser's own Back can notice: Telegram's Back button pushes a new hash.
let currentHash = "";
let confirming = false;

async function navigate() {
	const target = window.location.hash;
	if (target === currentHash) {
		return;
	}
	if (blocksLeave()) {
		history.replaceState(null, "", currentHash || `${window.location.pathname}${window.location.search}`);
		// Telegram allows one popup at a time; a second Back while it is open
		// is dropped.
		if (confirming) {
			return;
		}
		confirming = true;
		const leave = await confirmDestructive({
			title: "Discard changes?",
			message: "You have unsaved changes on this screen. Leaving now loses them.",
			action: "Discard",
		}).finally(() => {
			confirming = false;
		});
		if (!leave) {
			return;
		}
		setLeaveGuard(null);
		window.location.hash = target;
		return;
	}
	currentHash = target;
	render();
}

if (root) {
	if (!window.location.hash) {
		const chatID = launchChatID();
		if (chatID) {
			history.replaceState(null, "", `#/chats/${chatID}`);
		}
	}
	if (supports("6.1")) {
		telegram.BackButton.onClick(() => {
			const parent = parentRoute(window.location.hash);
			if (parent) {
				haptic("light");
				window.location.hash = parent;
			}
		});
	}
	window.addEventListener("hashchange", navigate);
	currentHash = window.location.hash;
	render();
}
