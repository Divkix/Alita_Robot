import { renderBlacklists, renderFilters, renderNotes } from "./screens/lists.js";
import { renderSettings } from "./screens/settings.js";

// Dashboard screens of one chat. Each screen module adds one entry
// ({ key, label, render(root, chatID, chat) }); the chat screen links to them
// and the router renders them at #/chats/<id>/<key>.
export const sections = [
	{ key: "settings", label: "Settings", render: renderSettings },
	{ key: "filters", label: "Filters", render: renderFilters },
	{ key: "notes", label: "Notes", render: renderNotes },
	{ key: "blacklists", label: "Blacklists", render: renderBlacklists },
];
