// Shared pieces of the analytics screens: SVG nodes and the 7/30/90-day
// window switch. Like the overview, nothing here builds markup from strings:
// server data reaches the DOM only through textContent and SVG attributes.
import { clear, haptic, loading, showError, textElement } from "./ui.js";

export const WINDOWS = [7, 30, 90];
const SVG_NS = "http://www.w3.org/2000/svg";

export function svgNode(tag, attributes) {
	const node = document.createElementNS(SVG_NS, tag);
	for (const [name, value] of Object.entries(attributes)) {
		node.setAttribute(name, String(value));
	}
	return node;
}

// svgText is an SVG <text> whose content is set as text, never parsed.
export function svgText(text, attributes) {
	const node = svgNode("text", attributes);
	node.textContent = text;
	return node;
}

// finite returns a server number, or 0 when it is missing or not finite, so a
// partial response draws an empty chart instead of NaN.
export function finite(value) {
	return Number.isFinite(value) ? value : 0;
}

// collectionNote is the folded "How this is collected" paragraph under a chart.
export function collectionNote(text) {
	const details = document.createElement("details");
	details.className = "collection-note";
	details.append(textElement("summary", "How this is collected"), textElement("p", text));
	return details;
}

// reuseFor is how long a window's answer is redrawn without a new request. The
// counters behind every chart are flushed once a minute, so switching back to
// a window inside that minute would fetch the same numbers.
const reuseFor = 60_000;

// windowedScreen renders the window switch and fetches the selected window with
// fetchData(days) and draws it with draw(content, data). A slow answer for an
// old window never overwrites a newer one: draw runs only for the latest request.
export async function windowedScreen(root, label, loadingText, fetchData, draw) {
	clear(root);

	const controls = document.createElement("div");
	controls.className = "window-switch";
	controls.setAttribute("role", "group");
	controls.setAttribute("aria-label", label);
	const content = document.createElement("div");
	content.className = "window-content";
	root.append(controls, content);

	let days = WINDOWS[0];
	let request = 0;
	const buttons = [];
	const answers = new Map();

	function syncButtons() {
		for (const entry of buttons) {
			const active = entry.value === days;
			entry.button.classList.toggle("active", active);
			entry.button.setAttribute("aria-pressed", String(active));
		}
	}

	async function load() {
		const current = ++request;
		const cached = answers.get(days);
		if (cached && Date.now() - cached.at < reuseFor) {
			clear(content);
			draw(content, cached.data);
			return;
		}
		clear(content);
		content.append(loading(loadingText));
		let data;
		try {
			data = await fetchData(days);
		} catch (error) {
			if (current === request) {
				showError(content, error, () => void load());
			}
			return;
		}
		if (current !== request) {
			return;
		}
		answers.set(days, { data, at: Date.now() });
		clear(content);
		draw(content, data);
	}

	for (const value of WINDOWS) {
		const button = textElement("button", `${value} days`);
		button.type = "button";
		button.addEventListener("click", () => {
			if (value === days) {
				return;
			}
			days = value;
			haptic("selection");
			syncButtons();
			void load();
		});
		controls.append(button);
		buttons.push({ value, button });
	}

	syncButtons();
	await load();
}
