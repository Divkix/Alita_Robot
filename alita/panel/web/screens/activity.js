// Activity: the chat's messages by weekday and hour as a heatmap, and its
// Telegram member count over the window. Both are counts only, so every
// member with access sees this screen. Drawn as SVG like the overview (the
// CSP blocks chart libraries); server data reaches the DOM only through
// textContent and SVG attributes.
import { api } from "../api.js";
import { collectionNote, finite, svgNode, svgText, windowedScreen } from "../charts.js";
import { textElement } from "../ui.js";

const WEEKDAYS = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];

function hourLabel(hour) {
	return `${String(hour).padStart(2, "0")}:00`;
}

// heatmap draws one cell per weekday and hour. A cell's opacity is its
// average per day (its total over how often that weekday occurs in the
// window) as a share of the busiest cell's, so a weekday that occurs five
// times in 30 days does not look busier for it. An hour without messages stays
// a faint outline.
function heatmap(grid, weekdays, timezone) {
	const width = 360;
	const left = 32;
	const right = 4;
	const top = 16;
	const gap = 2;
	const cellWidth = (width - left - right) / 24;
	const cellHeight = 16;
	const height = top + 7 * (cellHeight + gap);

	const chart = svgNode("svg", {
		viewBox: `0 0 ${width} ${height}`,
		class: "activity-chart heatmap",
		role: "img",
		"aria-label": `Messages by weekday and hour (${timezone})`,
	});

	const occurrences = (row) => Math.max(1, finite(weekdays?.[row]));
	const average = (row, hour) => finite(grid[row]?.[hour]) / occurrences(row);
	const peak = Math.max(
		Number.EPSILON,
		...WEEKDAYS.flatMap((_, row) => Array.from({ length: 24 }, (_, hour) => average(row, hour))),
	);

	for (const hour of [0, 6, 12, 18]) {
		chart.append(svgText(String(hour).padStart(2, "0"), { x: left + hour * cellWidth, y: 11, class: "chart-label" }));
	}

	WEEKDAYS.forEach((weekday, row) => {
		const y = top + row * (cellHeight + gap);
		chart.append(svgText(weekday, { x: 0, y: y + cellHeight - 4, class: "chart-label" }));
		for (let hour = 0; hour < 24; hour++) {
			const value = finite(grid[row]?.[hour]);
			const share = average(row, hour) / peak;
			const cell = svgNode("rect", {
				x: (left + hour * cellWidth + gap / 2).toFixed(1),
				y,
				width: (cellWidth - gap).toFixed(1),
				height: cellHeight,
				rx: 2,
				class: value > 0 ? "heat-cell" : "heat-cell heat-empty",
				"fill-opacity": value > 0 ? (0.15 + 0.85 * share).toFixed(2) : 1,
			});
			const title = svgNode("title", {});
			title.textContent = `${weekday} ${hourLabel(hour)} ${timezone}: ${value} messages over ${occurrences(row)} ${weekday}s`;
			cell.append(title);
			chart.append(cell);
		}
	});

	return chart;
}

// growthChart draws the measured member counts. The scale spans the window's
// lowest to highest count, so a change of a few members in a large group is
// still visible; the two labels say what the ends of the scale are. A count
// that never changed is a flat line at mid-height with one label.
function growthChart(points) {
	const width = 360;
	const height = 150;
	const left = 44;
	const right = 12;
	const top = 16;
	const bottom = 28;
	const plotWidth = width - left - right;
	const plotHeight = height - top - bottom;

	const chart = svgNode("svg", {
		viewBox: `0 0 ${width} ${height}`,
		class: "activity-chart",
		role: "img",
		"aria-label": "Members per day",
	});

	const counts = points.map((point) => finite(point.count));
	const low = Math.min(...counts);
	const high = Math.max(...counts);
	const span = Math.max(1, high - low);
	// Points are placed by date, so a day without a reading stays a gap in
	// time instead of pulling its neighbours together. A malformed day falls
	// back to even spacing.
	const dates = points.map((point) => Date.parse(`${point.day}T00:00:00Z`));
	const times = dates.every(Number.isFinite) ? dates : points.map((_, index) => index);
	const first = times[0];
	const range = times[times.length - 1] - first;
	const x = (index) => left + (range > 0 ? ((times[index] - first) / range) * plotWidth : plotWidth / 2);
	const flat = high === low;
	const y = (value) => (flat ? top + plotHeight / 2 : top + plotHeight - ((value - low) / span) * plotHeight);

	chart.append(
		svgNode("line", { x1: left, y1: top + plotHeight, x2: width - right, y2: top + plotHeight, class: "chart-axis" }),
	);
	if (flat) {
		chart.append(svgText(String(high), { x: 4, y: top + plotHeight / 2 + 4, class: "chart-label" }));
	} else {
		chart.append(
			svgNode("line", { x1: left, y1: top, x2: width - right, y2: top, class: "chart-grid" }),
			svgText(String(high), { x: 4, y: top + 4, class: "chart-label" }),
			svgText(String(low), { x: 4, y: top + plotHeight, class: "chart-label" }),
		);
	}

	if (points.length > 1) {
		const line = counts.map((value, index) => `${x(index).toFixed(1)},${y(value).toFixed(1)}`).join(" ");
		chart.append(svgNode("polyline", { points: line, class: "series series-members" }));
	} else {
		chart.append(svgNode("circle", { cx: x(0), cy: y(counts[0]), r: 3, class: "series-dot" }));
	}

	chart.append(
		svgText(String(points[0]?.day ?? ""), { x: left, y: height - 8, class: "chart-label" }),
		svgText(String(points[points.length - 1]?.day ?? ""), {
			x: width - right,
			y: height - 8,
			class: "chart-label",
			"text-anchor": "end",
		}),
	);
	return chart;
}

function signed(value) {
	return value > 0 ? `+${value}` : String(value);
}

function heatmapSection(data, timezone) {
	const section = document.createElement("section");
	section.className = "analytics-section";
	section.append(textElement("h3", `Messages by hour (${timezone})`));

	const grid = Array.isArray(data.grid) ? data.grid : [];
	if (finite(data.total) === 0) {
		section.append(textElement("p", "No messages have been recorded in this window yet.", "empty"));
		return section;
	}

	const card = document.createElement("div");
	card.className = "chart-card";
	card.append(heatmap(grid, Array.isArray(data.weekdays) ? data.weekdays : [], timezone));
	section.append(card);

	if (data.busiest && Number.isInteger(data.busiest.hour)) {
		section.append(
			textElement(
				"p",
				`Busiest hour: ${hourLabel(data.busiest.hour)}–${hourLabel((data.busiest.hour + 1) % 24)} ${timezone}, ${finite(data.busiest.messages)} messages.`,
				"chart-caption",
			),
		);
	}
	return section;
}

function growthSection(data) {
	const section = document.createElement("section");
	section.className = "analytics-section";
	section.append(textElement("h3", "Member growth"));

	const points = Array.isArray(data.points) ? data.points : [];
	if (points.length === 0) {
		section.append(
			textElement(
				"p",
				"No member counts yet. The bot records the count once a day for chats with activity; the history starts when this feature was deployed.",
				"empty",
			),
		);
		return section;
	}

	const card = document.createElement("div");
	card.className = "chart-card";
	card.append(growthChart(points));
	section.append(card);
	if (Number.isFinite(data.change)) {
		section.append(
			textElement("p", `${signed(data.change)} members since ${String(points[0].day)}.`, "chart-caption"),
		);
	}
	return section;
}

function collectionText(timezone) {
	return `Messages are counted per hour in memory and written to the database once a minute. Weekdays and hours are in ${timezone}; a cell's shade is its average per day, since a 30 or 90 day window holds some weekdays once more than others. The member count is read from Telegram once a day per chat with activity; days without a reading are left out of the chart.`;
}

export async function renderActivity(root, chatID) {
	const base = `chats/${encodeURIComponent(chatID)}`;
	await windowedScreen(
		root,
		"Activity window",
		"Loading activity…",
		(days) => Promise.all([api(`${base}/heatmap?days=${days}`), api(`${base}/growth?days=${days}`)]),
		(content, [heat, growth]) => {
			const timezone = typeof heat?.timezone === "string" && heat.timezone !== "" ? heat.timezone : "UTC";
			content.append(
				heatmapSection(heat ?? {}, timezone),
				growthSection(growth ?? {}),
				collectionNote(collectionText(timezone)),
			);
		},
	);
}
