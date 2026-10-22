import { renderActivity } from "./screens/activity.js";
import { renderBlacklists, renderFilters, renderNotes } from "./screens/lists.js";
import { renderMembers } from "./screens/members.js";
import { renderOverview } from "./screens/overview.js";
import { renderSettings } from "./screens/settings.js";
import { renderTimeline } from "./screens/timeline.js";
import { renderWarnings } from "./screens/warnings.js";

// Dashboard screens of one chat. Each screen module adds one entry
// ({ key, label, description, render(root, chatID, chat) }); the chat screen
// links to them and the router renders them at #/chats/<id>/<key>.
export const sections = [
	{ key: "overview", label: "Overview", description: "Activity, members and recent moderation", render: renderOverview },
	{ key: "activity", label: "Activity", description: "Busiest hours and member growth", render: renderActivity },
	{ key: "settings", label: "Settings", description: "Greetings, captcha, antiflood, locks and more", render: renderSettings },
	{ key: "filters", label: "Filters", description: "Automatic replies to keywords", render: renderFilters },
	{ key: "notes", label: "Notes", description: "Saved messages members can fetch", render: renderNotes },
	{ key: "blacklists", label: "Blacklists", description: "Words that trigger an action", render: renderBlacklists },
	{ key: "warnings", label: "Warnings", description: "Warned members and their warns", render: renderWarnings, adminOnly: true },
	{ key: "timeline", label: "Timeline", description: "Every ban, kick, mute and unban", render: renderTimeline, adminOnly: true },
	{ key: "members", label: "Top members", description: "Who sent the most messages", render: renderMembers, adminOnly: true },
];
