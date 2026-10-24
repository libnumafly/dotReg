import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import { unified } from '@astrojs/markdown-remark';
import path from 'node:path';
import { pages, repositoryRoot, repositoryUrl, siteBase, siteUrl, pagesRoot } from './scripts/content.mjs';
import { prepareContent } from './scripts/prepare-content.mjs';
import repositoryLinks from './scripts/remark-repository-links.mjs';

export default defineConfig({
  site: siteUrl,
  base: siteBase,
  trailingSlash: 'always',
  output: 'static',
  markdown: { processor: unified({ remarkPlugins: [repositoryLinks] }) },
  integrations: [
    starlight({
      title: 'dotReg',
      description: 'Readable configuration, built on dotted keys.',
      logo: { src: './src/assets/mark.svg', alt: '' },
      favicon: '/favicon.svg',
      customCss: ['./src/styles/custom.css'],
      social: [{ icon: 'github', label: 'GitHub', href: repositoryUrl }],
      tableOfContents: { minHeadingLevel: 2, maxHeadingLevel: 3 },
      sidebar: [
        { label: 'Start here', items: [
          { label: 'Introduction', slug: 'overview' },
          { label: 'Getting started', slug: 'getting-started' },
          { label: 'Documentation guide', slug: 'documentation' },
        ] },
        { label: 'The notation', items: [
          { label: 'Specification', slug: 'specification' },
          { label: 'Type profiles and candidate types', slug: 'type-profile' },
          { label: 'Conformance checklist', slug: 'conformance' },
          { label: 'Example configuration', slug: 'examples' },
          { label: 'Undecided matters', slug: 'open-issues' },
        ] },
        { label: 'Implementations', items: [
          { label: 'Parser set and common contract', slug: 'implementations' },
          { label: 'Go sample', slug: 'implementations/go' },
          { label: 'Python sample', slug: 'implementations/python' },
          { label: 'JavaScript sample', slug: 'implementations/javascript' },
          { label: 'TypeScript sample', slug: 'implementations/typescript' },
          { label: 'Java sample', slug: 'implementations/java' },
          { label: 'C# sample', slug: 'implementations/csharp' },
          { label: 'Rust sample', slug: 'implementations/rust' },
          { label: 'C++ sample', slug: 'implementations/cpp' },
          { label: 'Time and memory measurements', slug: 'implementations/benchmarks' },
          { label: 'Public sample parser adoption', slug: 'implementations/benchmarks/public-rollin-2026-10-24' },
          { label: 'Go parser tuning results', slug: 'implementations/benchmarks/tuning-2026-10-24' },
          { label: 'Configuration format comparison', slug: 'implementations/benchmarks/comparison' },
          { label: 'Native parser workload matrix', slug: 'implementations/benchmarks/native-2026-10-24' },
          { label: 'Historical measurements', slug: 'implementations/go/performance' },
        ] },
        { label: 'Licence', slug: 'license' },
      ],
    }),
    {
      name: 'dotreg-source-watch',
      hooks: {
        'astro:server:setup': ({ server }) => {
          const inputs = [
            ...pages.map((page) => path.join(repositoryRoot, page.source)),
            path.join(repositoryRoot, 'examples/sample.txt'),
            path.join(repositoryRoot, 'LICENSE'),
            path.join(pagesRoot, 'content/index.mdx'),
          ];
          const normalized = new Set(inputs.map((input) => path.resolve(input)));
          server.watcher.add(inputs);
          server.watcher.on('change', async (changed) => {
            if (normalized.has(path.resolve(changed))) {
              try { await prepareContent(); }
              catch (error) { server.config.logger.error(error.stack ?? String(error)); }
            }
          });
        },
      },
    },
  ],
});
