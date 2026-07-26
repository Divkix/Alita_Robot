# Alita Robot Documentation

This is the documentation website for [Alita Robot](https://github.com/divkix/Alita_Robot), a modern Telegram group management bot built with Go.

Built with [Blume](https://useblume.dev/) — a fast, AI-ready, markdown-first documentation framework.

## Project Structure

```
docs/
├── public/           # Static assets (favicon, robots.txt)
├── src/
│   ├── assets/       # Images for docs
│   └── content/
│       └── docs/     # Documentation pages (.md/.mdx) + per-folder meta.ts
├── blume.config.ts   # Blume configuration
├── package.json
├── tsconfig.json
└── wrangler.jsonc    # Cloudflare Workers deployment config
```

Documentation pages live in `src/content/docs/`. Each file becomes a route
based on its filename; sidebar groups are inferred from the folder tree and
refined with `meta.ts` files. The Go generator in `scripts/generate_docs`
still writes module and lock-type pages into this folder (files opt out of
regeneration with a `<!-- MANUALLY MAINTAINED: do not regenerate -->`
sentinel).

## Development

All commands run from the `docs/` directory:

Node.js `v22.12.0` or higher is required by Blume. Use an even-numbered
release line such as Node 22 or 24.

| Command         | Action                                       |
| :-------------- | :------------------------------------------- |
| `bun install`   | Install dependencies                         |
| `bun dev`       | Start dev server (hot reload)                |
| `bun build`     | Build production site to `./dist/`           |
| `bun preview`   | Preview production build locally             |

## AI features

Blume emits machine-readable docs by default on a static build:
`llms.txt` and `llms-full.txt` at the site root, plus a raw `.md`/`.mdx`
mirror of every page (append `.md` to any page URL). No API key is required.

## Deployment

The documentation is deployed to Cloudflare Workers as static assets.
Configuration is in `wrangler.jsonc` (output directory `./dist`).

## Contributing

Documentation improvements are welcome. Follow the same contribution guidelines as the main project:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a Pull Request

See the main [README](../README.md) for full contribution guidelines.

## Links

- [Alita Robot Repository](https://github.com/divkix/Alita_Robot)
- [Live Documentation](https://alita.divkix.me)
- [Support Group](https://t.me/DivideSupport)
