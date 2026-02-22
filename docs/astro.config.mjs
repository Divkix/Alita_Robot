// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import starlightLlmsTxt from 'starlight-llms-txt';
import starlightThemeBlack from 'starlight-theme-black';

// https://astro.build/config
export default defineConfig({
  site: 'https://alita-docs.divkix.me',
  integrations: [
    starlight({
      title: 'Alita Robot',
      logo: {
        dark: './src/assets/logo-dark.svg',
        light: './src/assets/logo-light.svg',
      },
      plugins: [starlightThemeBlack({}), starlightLlmsTxt()],
      social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/divkix/Alita_Robot' }],
      customCss: [
        './src/styles/custom.css',
      ],
      sidebar: [
        {
          label: 'Getting Started',
          items: [
            { label: 'Introduction', slug: 'getting-started/introduction' },
            { label: 'Quick Start', slug: 'getting-started/quick-start' },
          ],
        },
        {
          label: 'Commands',
          collapsed: false,
          autogenerate: { directory: 'commands' },
        },
        {
          label: 'Self-Hosting',
          collapsed: true,
          autogenerate: { directory: 'self-hosting' },
        },
        {
          label: 'Architecture',
          collapsed: true,
          autogenerate: { directory: 'architecture' },
        },
        {
          label: 'API Reference',
          collapsed: true,
          autogenerate: { directory: 'api-reference' },
        },
        {
          label: 'Contributing',
          collapsed: true,
          autogenerate: { directory: 'contributing' },
        },
      ],
    }),
  ],
});
