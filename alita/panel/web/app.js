import { telegram } from "./api.js";
import { renderChat, renderSection } from "./screens/chat.js";
import { renderChats } from "./screens/chats.js";

telegram?.ready();
telegram?.expand();

const root = document.getElementById("app");

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
	window.addEventListener("hashchange", render);
	render();
}
