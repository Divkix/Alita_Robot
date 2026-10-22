// Small DOM helpers. Everything renders with textContent so chat titles,
// usernames and filter text can never become markup.

export function clear(node) {
	while (node.firstChild) {
		node.removeChild(node.firstChild);
	}
}

export function textElement(tag, text, className) {
	const element = document.createElement(tag);
	element.textContent = text;
	if (className) {
		element.className = className;
	}
	return element;
}

export function showError(root, error) {
	clear(root);
	root.append(textElement("p", error?.message ?? "Something went wrong.", "error"));
}

// userLabel names a user from a response's names map (keyed by decimal ID),
// falling back to the ID when the bot never stored the user.
export function userLabel(names, id) {
	const entry = names?.[String(id)];
	if (!entry) {
		return `User ${id}`;
	}
	if (entry.name && entry.username) {
		return `${entry.name} (@${entry.username})`;
	}
	return entry.name || `@${entry.username}`;
}
