import { telegram } from "./api.js";
import { renderChat, renderSection } from "./screens/chat.js";
import { renderChats } from "./screens/chats.js";

telegram?.ready();
telegram?.expand();

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

function render() {
	const match = /^#\/chats\/(-?\d+)(?:\/([a-z-]+))?$/.exec(window.location.hash);
	if (match && match[2]) {
		return renderSection(root, match[1], match[2]);
	}
	if (match) {
		return renderChat(root, match[1]);
	}
	return renderChats(root);
}

if (root) {
	if (!window.location.hash) {
		const chatID = launchChatID();
		if (chatID) {
			history.replaceState(null, "", `#/chats/${chatID}`);
		}
	}
	window.addEventListener("hashchange", render);
	render();
}
