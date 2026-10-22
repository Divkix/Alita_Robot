import { renderBlacklists, renderFilters, renderNotes } from "./screens/lists.js";
import { renderOverview } from "./screens/overview.js";
import { renderSettings } from "./screens/settings.js";
import { renderTimeline } from "./screens/timeline.js";
import { renderWarnings } from "./screens/warnings.js";

// Dashboard screens of one chat. Each screen module adds one entry
// ({ key, label, render(root, chatID, chat) }); the chat screen links to them
// and the router renders them at #/chats/<id>/<key>.
export const sections = [
	{ key: "overview", label: "Overview", render: renderOverview },
	{ key: "settings", label: "Settings", render: renderSettings },
	{ key: "filters", label: "Filters", render: renderFilters },
	{ key: "notes", label: "Notes", render: renderNotes },
	{ key: "blacklists", label: "Blacklists", render: renderBlacklists },
	{ key: "warnings", label: "Warnings", render: renderWarnings, adminOnly: true },
	{ key: "timeline", label: "Timeline", render: renderTimeline, adminOnly: true },
];
