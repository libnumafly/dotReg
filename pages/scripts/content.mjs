import { fileURLToPath } from 'node:url';

export const pagesRoot = fileURLToPath(new URL('../', import.meta.url));
export const repositoryRoot = fileURLToPath(new URL('../../', import.meta.url));
export const repositoryUrl = 'https://github.com/libnumafly/dotReg';
export const siteUrl = 'https://libnumafly.github.io';
export const siteBase = '/dotReg/';

// These files remain the single source for the published documentation.
export const pages = [
  {
    source: 'README.md',
    slug: 'overview',
    title: 'dotReg at a glance',
    description: 'A short introduction to dotted keys, repeated values and literal text.',
  },
  {
    source: 'docs/README.md',
    slug: 'documentation',
    title: 'Documentation',
    description: 'Find the guides, reference specification and sample implementation.',
  },
  {
    source: 'docs/getting-started.md',
    slug: 'getting-started',
    title: 'Getting started',
    description: 'Learn dotReg through small examples, from full keys to multiline values.',
  },
  {
    source: 'docs/config-format.md',
    slug: 'specification',
    title: 'Configuration file format',
    description: 'The dotReg reference specification, including established rules and undecided matters.',
  },
  {
    source: 'docs/conformance.md',
    slug: 'conformance',
    title: 'Conformance checklist',
    description: 'Check an implementation against the established dotReg format rules.',
  },
  {
    source: 'implementations/go/README.md',
    slug: 'implementations/go',
    title: 'Go sample implementation',
    description: 'Run the sample parser and CLI, use its API and understand provisional choices.',
  },
  {
    source: 'implementations/go/PERFORMANCE.md',
    slug: 'implementations/go/performance',
    title: 'Historical parser measurements',
    description: 'Historical Go parser measurements, their method and their limitations.',
  },
];
