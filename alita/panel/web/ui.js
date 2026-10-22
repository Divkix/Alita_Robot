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
