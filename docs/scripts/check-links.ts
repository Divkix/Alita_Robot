// Link check for hand-written docs pages: fails when a relative link escapes the
// content tree or points at a route/asset that does not exist.
//
// Blume has no link validator, so this script covers the pages the docs generator
// does not own. Run from docs/: `bun run check:links` (or pass page paths).
import { existsSync, readFileSync, statSync } from "node:fs";
import { dirname, join, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

const docsRoot = resolve(fileURLToPath(new URL("..", import.meta.url)));
const contentRoot = join(docsRoot, "src", "content", "docs");
const publicRoot = join(docsRoot, "public");
const pagesRoot = join(docsRoot, "pages");

const defaultPages = [
	"src/content/docs/add-to-group.mdx",
	"src/content/docs/getting-started/quick-start.mdx",
];
const pages = process.argv.length > 2 ? process.argv.slice(2) : defaultPages;

const schemes = /^(?:[a-z][a-z0-9+.-]*:|\/\/)/i;

/** Every link a page can emit: Markdown inline, Markdown reference, and HTML. */
const extractLinks = (source) => {
	const links = [];
	for (const match of source.matchAll(/\]\(([^)\s]+)\)/g)) {
		links.push(match[1]);
	}
	for (const match of source.matchAll(/^\[[^\]]+\]:\s*(\S+)/gm)) {
		links.push(match[1]);
	}
	for (const match of source.matchAll(/\b(?:href|src)="([^"]+)"/g)) {
		links.push(match[1]);
	}
	for (const match of source.matchAll(/\bsrcSet="([^"]+)"/g)) {
		links.push(match[1]);
	}
	for (const match of source.matchAll(/\bsrcset="([^"]+)"/g)) {
		links.push(match[1]);
	}
	return links;
};

const isFile = (path) => existsSync(path) && statSync(path).isFile();

/** Route files a page URL can map to under a content or pages root. */
const resolvesToPage = (root, route) => {
	const base = join(root, route.replace(/\/+$/, ""));
	return [
		base,
		`${base}.md`,
		`${base}.mdx`,
		join(base, "index.md"),
		join(base, "index.mdx"),
		`${base}.astro`,
		join(base, "index.astro"),
	].some((candidate) => isFile(candidate));
};

/** A root-relative path is served from public/ or is a docs route. */
const checkRootRelative = (url) => {
	const route = url.replace(/^\/+/, "");
	if (isFile(join(publicRoot, route))) {
		return null;
	}
	if (resolvesToPage(contentRoot, route) || resolvesToPage(pagesRoot, route)) {
		return null;
	}
	return `/${route} matches no file in public/ or src/content/docs/`;
};

/** A relative path must stay inside the content tree and resolve to a page. */
const checkRelative = (pageDir, url) => {
	const target = resolve(pageDir, url);
	const inside = relative(contentRoot, target);
	if (inside === "" || inside.startsWith(`..${sep}`) || inside === ".." || resolve(contentRoot, inside) !== target) {
		return `${url} resolves outside src/content/docs/ (${target})`;
	}
	if (resolvesToPage(contentRoot, inside)) {
		return null;
	}
	return `${url} matches no page in src/content/docs/`;
};

let checked = 0;
const problems = [];

for (const page of pages) {
	const pagePath = resolve(docsRoot, page);
	if (!isFile(pagePath)) {
		problems.push(`${page}: page not found`);
		continue;
	}
	const source = readFileSync(pagePath, "utf8");
	for (const raw of extractLinks(source)) {
		const url = raw.trim();
		if (url === "" || url.startsWith("#") || schemes.test(url)) {
			continue;
		}
		checked++;
		const clean = url.split(/[?#]/, 1)[0];
		if (clean === "") {
			continue;
		}
		const problem = clean.startsWith("/")
			? checkRootRelative(clean)
			: checkRelative(dirname(pagePath), clean);
		if (problem) {
			problems.push(`${page}: ${problem}`);
		}
	}
}

if (problems.length > 0) {
	for (const problem of problems) {
		console.error(problem);
	}
	process.exit(1);
}

console.log(`check:links — ${checked} relative links in ${pages.length} page(s) resolve inside the content tree`);
