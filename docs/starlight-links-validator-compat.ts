import type { StarlightPlugin } from '@astrojs/starlight/types';
import type { IntegrationResolvedRoute } from 'astro';
import { AstroError } from 'astro/errors';
import { z } from 'astro/zod';

import { clearContentLayerCache } from './node_modules/starlight-links-validator/libs/astro.ts';
import { pathnameToSlug, stripTrailingSlash } from './node_modules/starlight-links-validator/libs/path.ts';
import {
  remarkStarlightLinksValidator,
  type RemarkStarlightLinksValidatorConfig,
} from './node_modules/starlight-links-validator/libs/remark.ts';
import { logErrors, validateLinks } from './node_modules/starlight-links-validator/libs/validation.ts';

// Temporary Astro 6 / Zod 4 compatibility wrapper until the upstream package
// replaces the deprecated Zod 3 `.args()` / `.returns()` function schema API.
const StarlightLinksValidatorOptionsSchema = z
  .object({
    components: z.tuple([z.string(), z.string()]).array().default([]),
    errorOnFallbackPages: z.boolean().default(true),
    errorOnInconsistentLocale: z.boolean().default(false),
    errorOnRelativeLinks: z.boolean().default(true),
    errorOnInvalidHashes: z.boolean().default(true),
    errorOnLocalLinks: z.boolean().default(true),
    exclude: z
      .union([
        z.array(z.string()),
        z.function({
          input: [
            z.object({
              file: z.string(),
              link: z.string(),
              slug: z.string(),
            }),
          ],
          output: z.boolean(),
        }),
      ])
      .default([]),
    sameSitePolicy: z.enum(['error', 'ignore', 'validate']).default('ignore'),
  });

type StarlightLinksValidatorUserOptions = z.input<typeof StarlightLinksValidatorOptionsSchema>;

export default function starlightLinksValidatorPlugin(
  userOptions?: StarlightLinksValidatorUserOptions,
): StarlightPlugin {
  const options = StarlightLinksValidatorOptionsSchema.safeParse(userOptions ?? {});

  if (!options.success) {
    throwPluginError('Invalid options passed to the starlight-links-validator plugin.');
  }

  return {
    name: 'starlight-links-validator-plugin',
    hooks: {
      'config:setup'({ addIntegration, astroConfig, config: starlightConfig, logger }) {
        let routes: IntegrationResolvedRoute[] = [];
        const site = astroConfig.site ? stripTrailingSlash(astroConfig.site) : undefined;

        addIntegration({
          name: 'starlight-links-validator-integration',
          hooks: {
            'astro:config:setup': async ({ command, updateConfig }) => {
              if (command !== 'build') {
                return;
              }

              await clearContentLayerCache(astroConfig, logger);

              updateConfig({
                markdown: {
                  remarkPlugins: [
                    [
                      remarkStarlightLinksValidator,
                      {
                        base: astroConfig.base,
                        options: options.data,
                        site,
                        srcDir: astroConfig.srcDir,
                      } satisfies RemarkStarlightLinksValidatorConfig,
                    ],
                  ],
                },
              });
            },
            'astro:routes:resolved': (params) => {
              routes = params.routes;
            },
            'astro:build:done': ({ dir, pages, assets }) => {
              const customPages = new Set<string>();

              for (const [pattern, urls] of assets) {
                const route = routes.find((route) => route.pattern === pattern);
                if (!route || route.origin !== 'project') continue;

                for (const url of urls) {
                  customPages.add(pathnameToSlug(url.pathname.replace(astroConfig.outDir.pathname, '')));
                }
              }

              const errors = validateLinks(pages, customPages, dir, astroConfig, starlightConfig, options.data);

              const hasInvalidLinkToCustomPage = logErrors(logger, errors, site);

              if (errors.size > 0) {
                throwPluginError(
                  'Links validation failed.',
                  hasInvalidLinkToCustomPage
                    ? 'Some invalid links point to custom pages which cannot be validated, see the `exclude` option for more informations at https://starlight-links-validator.vercel.app/configuration#exclude'
                    : undefined,
                );
              }
            },
          },
        });
      },
    },
  };
}

function throwPluginError(message: string, additionalHint?: string): never {
  let hint = 'See the error report above for more informations.\n\n';
  if (additionalHint) hint += `${additionalHint}\n\n`;
  hint +=
    'If you believe this is a bug, please file an issue at https://github.com/HiDeoo/starlight-links-validator/issues/new/choose';

  throw new AstroError(message, hint);
}
