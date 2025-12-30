# Alita Robot Documentation

This is the documentation website for [Alita Robot](https://github.com/divkix/Alita_Robot), a modern Telegram group management bot built with Go.

Built with [Starlight](https://starlight.astro.build/) on [Astro](https://astro.build/).

## Project Structure

```
docs/
├── public/           # Static assets (favicons, images)
├── src/
│   ├── assets/       # Optimized images for docs
│   ├── content/
│   │   └── docs/     # Documentation pages (.md/.mdx)
│   └── content.config.ts
├── astro.config.mjs  # Astro configuration
├── package.json
├── tsconfig.json
└── wrangler.jsonc    # Cloudflare Pages deployment config
```

Documentation pages live in `src/content/docs/`. Each file becomes a route based on its filename.

## Development

All commands run from the `docs/` directory:

| Command         | Action                                       |
| :-------------- | :------------------------------------------- |
| `bun install`   | Install dependencies                         |
| `bun dev`       | Start dev server at `localhost:4321`         |
| `bun build`     | Build production site to `./dist/`           |
| `bun preview`   | Preview production build locally             |
| `bun astro ...` | Run Astro CLI commands (`add`, `check`, etc) |

## Deployment

The documentation is deployed to Cloudflare Pages. Configuration is in `wrangler.jsonc`.

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
