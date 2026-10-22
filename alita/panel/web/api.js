// Telegram Mini App bootstrap: initData auth header and JSON API helper.
// Data is rendered through DOM APIs only; never innerHTML.

export const telegram = window.Telegram?.WebApp ?? null;
export const initData = telegram?.initData ?? "";

export class ApiError extends Error {
	constructor(code, message, status) {
		super(message);
		this.name = "ApiError";
		this.code = code;
		this.status = status;
	}
}

export async function api(path, { method = "GET", body } = {}) {
	const headers = { "X-Telegram-Init-Data": initData };
	if (method !== "GET") {
		headers["Content-Type"] = "application/json";
	}

	const response = await fetch(`/panel/api/v1/${path.replace(/^\/+/, "")}`, {
		method,
		headers,
		body: body === undefined ? undefined : JSON.stringify(body),
		credentials: "omit",
		cache: "no-store",
	});

	let payload = null;
	try {
		payload = await response.json();
	} catch {
		payload = null;
	}

	if (!response.ok) {
		const detail = payload?.error ?? {};
		throw new ApiError(detail.code ?? "http_error", detail.message ?? `Request failed (${response.status})`, response.status);
	}
	return payload;
}
