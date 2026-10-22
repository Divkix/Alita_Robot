// Top members: the users who sent the most messages in a 7, 30 or 90 day
// window. It names users, so like the timeline it is admin-only (the server
// refuses members too). Names reach the DOM only through textContent.
import { api } from "../api.js";
import { collectionNote, finite, svgNode, windowedScreen } from "../charts.js";
import { textElement, userLabel } from "../ui.js";

// bar is a fixed-height SVG bar whose length is the member's share of the
// first member's count.
function bar(value, peak) {
	const width = 100;
	const chart = svgNode("svg", {
		viewBox: `0 0 ${width} 6`,
		class: "member-bar",
		preserveAspectRatio: "none",
		"aria-hidden": "true",
	});
	chart.append(svgNode("rect", { x: 0, y: 0, width, height: 6, rx: 3, class: "member-bar-track" }));
	const filled = peak > 0 ? Math.max(2, (value / peak) * width) : 0;
	chart.append(svgNode("rect", { x: 0, y: 0, width: filled.toFixed(1), height: 6, rx: 3, class: "member-bar-fill" }));
	return chart;
}

function drawMembers(content, data) {
	const members = Array.isArray(data?.members) ? data.members : [];
	if (members.length === 0) {
		content.append(textElement("p", "No member messages have been recorded in this window yet.", "empty"));
	} else {
		const peak = Math.max(1, ...members.map((member) => finite(member.messages)));
		const list = document.createElement("ol");
		list.className = "member-list";
		for (const member of members) {
			const messages = finite(member.messages);
			const item = document.createElement("li");
			const head = document.createElement("div");
			head.className = "member-row";
			head.append(
				textElement("span", userLabel(data.names, member.user_id), "member-name"),
				textElement("span", `${messages} ${messages === 1 ? "message" : "messages"}`, "member-count"),
			);
			item.append(head, bar(messages, peak));
			list.append(item);
		}
		content.append(list);
	}

	content.append(
		collectionNote(
			"Messages are counted per user and UTC day in memory and written to the database once a minute; days are kept for 90 days. Channel posts and anonymous admins are not counted, and edits are not new messages. Message text is never stored.",
		),
	);
}

export async function renderMembers(root, chatID) {
	await windowedScreen(root, "Top members window", "Loading members…", (days) =>
		api(`chats/${encodeURIComponent(chatID)}/top-members?days=${days}`), drawMembers);
}
