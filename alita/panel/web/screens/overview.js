// Overview: the chat's Telegram member count, a daily messages/joins/leaves
// chart for a 7, 30 or 90 day window, and the most recent moderation actions.
// Server data reaches the DOM only through textContent and SVG attributes, so
// a chat title or an action value can never become markup.
import { api } from "../api.js";
import { clear, showError, textElement, userLabel } from "../ui.js";

const WINDOWS = [7, 30, 90];
const SVG_NS = "http://www.w3.org/2000/svg";

const SERIES = [
	{ key: "messages", label: "Messages", className: "series-messages" },
	{ key: "joins", label: "Joins", className: "series-joins" },
	{ key: "leaves", label: "Leaves", className: "series-leaves" },
];

// number renders a server count. A missing or non-finite value becomes 0 so a
// partial response shows an empty chart instead of NaN.
function number(value) {
	return Number.isFinite(value) ? String(value) : "0";
}

function count(point, key) {
	const value = point?.[key];
	return Number.isFinite(value) ? value : 0;
}

function svgNode(tag, attributes) {
	const node = document.createElementNS(SVG_NS, tag);
	for (const [name, value] of Object.entries(attributes)) {
		node.setAttribute(name, String(value));
	}
	return node;
}

// activityChart draws one line per counter over the zero-filled series: no
// chart library, no innerHTML, and the scale starts at a real zero baseline.
function activityChart(series) {
	const width = 640;
	const height = 180;
	const left = 40;
	const right = 12;
	const top = 16;
	const bottom = 28;
	const plotWidth = width - left - right;
	const plotHeight = height - top - bottom;

	const chart = svgNode("svg", {
		viewBox: `0 0 ${width} ${height}`,
		class: "activity-chart",
		role: "img",
		"aria-label": "Messages, joins and leaves per day",
	});

	const peak = Math.max(1, ...series.map((point) => Math.max(count(point, "messages"), count(point, "joins"), count(point, "leaves"))));
	const x = (index) => left + (series.length > 1 ? (index / (series.length - 1)) * plotWidth : plotWidth / 2);
	const y = (value) => top + plotHeight - (value / peak) * plotHeight;

	chart.append(
		svgNode("line", {
			x1: left,
			y1: top + plotHeight,
			x2: width - right,
			y2: top + plotHeight,
			class: "chart-axis",
		}),
	);

	const peakLabel = svgNode("text", { x: 4, y: top + 8, class: "chart-label" });
	peakLabel.textContent = number(peak);
	chart.append(peakLabel);

	for (const counter of SERIES) {
		const points = series
			.map((point, index) => `${x(index).toFixed(1)},${y(count(point, counter.key)).toFixed(1)}`)
			.join(" ");
		chart.append(svgNode("polyline", { points, class: `series ${counter.className}` }));
	}

	const firstLabel = svgNode("text", { x: left, y: height - 8, class: "chart-label" });
	firstLabel.textContent = series[0]?.day ?? "";
	chart.append(firstLabel);

	const lastLabel = svgNode("text", {
		x: width - right,
		y: height - 8,
		class: "chart-label",
		"text-anchor": "end",
	});
	lastLabel.textContent = series[series.length - 1]?.day ?? "";
	chart.append(lastLabel);

	return chart;
}

function chartLegend() {
	const legend = document.createElement("ul");
	legend.className = "chart-legend";
	for (const counter of SERIES) {
		const item = document.createElement("li");
		const swatch = document.createElement("span");
		swatch.className = `swatch ${counter.className}`;
		item.append(swatch, document.createTextNode(counter.label));
		legend.append(item);
	}
	return legend;
}

function summaryItem(label, value) {
	const item = document.createElement("li");
	item.append(textElement("span", label, "summary-label"), textElement("strong", number(value)));
	return item;
}

function totalsBlock(totals) {
	const row = document.createElement("ul");
	row.className = "overview-totals";
	for (const counter of SERIES) {
		row.append(summaryItem(counter.label, totals[counter.key]));
	}
	return row;
}

function formatTime(value) {
	const date = new Date(value);
	if (Number.isNaN(date.getTime())) {
		return value == null ? "" : String(value);
	}
	return date.toLocaleString();
}

// recentSection lists action, actor, target and time. The API also carries a
// reason; the overview deliberately shows only the four fields the screen
// promises, and no message text exists to show.
function recentSection(events, names) {
	const section = document.createElement("section");
	section.className = "overview-recent";
	section.append(textElement("h3", "Recent moderation"));
	if (events.length === 0) {
		section.append(textElement("p", "No moderation actions recorded yet.", "empty"));
		return section;
	}

	const list = document.createElement("ul");
	list.className = "event-list";
	for (const event of events) {
		const item = document.createElement("li");
		item.className = "event-row";
		item.append(
			textElement("span", String(event.action ?? ""), "event-action"),
			textElement("span", `${partyLabel(names, event.actor_id)} → ${partyLabel(names, event.target_user_id)}`, "event-parties"),
			textElement("span", formatTime(event.created_at), "event-time"),
		);
		list.append(item);
	}
	section.append(list);
	return section;
}

// partyLabel names an actor or target. Names reach admins only; a member sees
// IDs, and a non-positive actor is the bot's own automated action.
function partyLabel(names, id) {
	if (!(Number(id) > 0)) {
		return "automatic";
	}
	return userLabel(names, id);
}

function collectionNote() {
	const details = document.createElement("details");
	details.className = "collection-note";
	details.append(textElement("summary", "How this is collected"));
	details.append(
		textElement(
			"p",
			"Message, join and leave counts are kept in memory by the bot and written to the database once a minute, so a restart can lose up to a minute of counts. The member count comes from Telegram and is refreshed at most once a minute. Moderation history is taken from the actions the bot recorded; message text is never stored.",
		),
	);
	return details;
}

function drawOverview(root, data) {
	const series = Array.isArray(data.days) ? data.days : [];
	const totals = data.totals ?? {};
	const recent = Array.isArray(data.recent) ? data.recent : [];

	const summary = document.createElement("ul");
	summary.className = "overview-summary";
	summary.append(
		summaryItem("Members", data.member_count),
		summaryItem("Moderation actions", data.moderation_count),
	);
	root.append(summary);

	const hasActivity = series.some(
		(point) => count(point, "messages") > 0 || count(point, "joins") > 0 || count(point, "leaves") > 0,
	);
	if (hasActivity) {
		root.append(activityChart(series), chartLegend());
	} else {
		root.append(
			textElement(
				"p",
				"No messages, joins or leaves have been recorded in this window yet.",
				"empty",
			),
		);
	}

	root.append(totalsBlock(totals), collectionNote(), recentSection(recent, data.names));
}

export async function renderOverview(root, chatID) {
	clear(root);

	const controls = document.createElement("div");
	controls.className = "window-switch";
	const content = document.createElement("div");
	content.className = "overview-content";
	root.append(controls, content);

	let days = WINDOWS[0];
	// request orders the fetches: a slow answer for the old window must not
	// overwrite a newer one.
	let request = 0;
	const buttons = [];

	function syncButtons() {
		for (const entry of buttons) {
			const active = entry.value === days;
			entry.button.classList.toggle("active", active);
			entry.button.setAttribute("aria-pressed", String(active));
		}
	}

	async function load() {
		const current = ++request;
		clear(content);
		content.append(textElement("p", "Loading activity…", "status"));
		let data;
		try {
			data = await api(`chats/${encodeURIComponent(chatID)}/overview?days=${days}`);
		} catch (error) {
			if (current === request) {
				showError(content, error);
			}
			return;
		}
		if (current !== request) {
			return;
		}
		clear(content);
		drawOverview(content, data);
	}

	for (const value of WINDOWS) {
		const button = textElement("button", `${value} days`);
		button.type = "button";
		button.addEventListener("click", () => {
			if (value === days) {
				return;
			}
			days = value;
			syncButtons();
			void load();
		});
		controls.append(button);
		buttons.push({ value, button });
	}

	syncButtons();
	await load();
}
