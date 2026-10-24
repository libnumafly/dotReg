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
    description: 'Find the guides, reference specification and sample parsers.',
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
    description: 'The dotReg core specification for syntax, string decoding and processing responsibilities.',
  },
  {
    source: 'docs/type-profile.md',
    slug: 'type-profile',
    title: 'Type profiles and candidate types',
    description: 'Established type-specific representations, candidate names and illustrative values.',
  },
  {
    source: 'docs/open-issues.md',
    slug: 'open-issues',
    title: 'Undecided matters',
    description: 'Open questions about the notation, type system, APIs and future properties.',
  },
  {
    source: 'docs/conformance.md',
    slug: 'conformance',
    title: 'Conformance checklist',
    description: 'Check an implementation against the established dotReg format rules.',
  },
  {
    source: 'implementations/README.md',
    slug: 'implementations',
    title: 'Sample parser set',
    description: 'Choose a language, use streaming APIs and check the common syntax contract.',
  },
  {
    source: 'implementations/python/README.md',
    slug: 'implementations/python',
    title: 'Python sample',
    description: 'Run the dependency-free Python parser, CLI, native tests and benchmark.',
  },
  {
    source: 'implementations/javascript/README.md',
    slug: 'implementations/javascript',
    title: 'JavaScript sample',
    description: 'Parse byte streams with an awaited visitor and decode common strings.',
  },
  {
    source: 'implementations/typescript/README.md',
    slug: 'implementations/typescript',
    title: 'TypeScript sample',
    description: 'Use static types with the shared JavaScript parser engine.',
  },
  {
    source: 'implementations/java/README.md',
    slug: 'implementations/java',
    title: 'Java sample',
    description: 'Run the standard-library Java parser and incremental API.',
  },
  {
    source: 'implementations/csharp/README.md',
    slug: 'implementations/csharp',
    title: 'C# sample',
    description: 'Use the .NET parser, atomic JSON CLI and framework-free tests.',
  },
  {
    source: 'implementations/rust/README.md',
    slug: 'implementations/rust',
    title: 'Rust sample',
    description: 'Try the dependency-free Rust parser and native benchmark.',
  },
  {
    source: 'implementations/cpp/README.md',
    slug: 'implementations/cpp',
    title: 'C++ sample',
    description: 'Build the C++17 parser, CLI, API tests and benchmark.',
  },
  {
    source: 'implementations/benchmarks/README.md',
    slug: 'implementations/benchmarks',
    title: 'Time and memory measurements',
    description: 'Current-specification parser, consumer and equivalent-format measurements, with recorded conditions and source hashes.',
  },
  {
    source: 'implementations/benchmarks/public-rollin-2026-10-24/README.md',
    slug: 'implementations/benchmarks/public-rollin-2026-10-24',
    title: 'Public sample parser adoption',
    description: 'Public parser changes, path ownership checks and balanced before-and-after measurements.',
  },
  {
    source: 'implementations/benchmarks/tuning-2026-10-24/README.md',
    slug: 'implementations/benchmarks/tuning-2026-10-24',
    title: 'Go parser tuning results',
    description: 'Balanced before-and-after measurements of the general Go parser against JSON, TOML, YAML and INI.',
  },
  {
    source: 'implementations/benchmarks/comparison/README.md',
    slug: 'implementations/benchmarks/comparison',
    title: 'Configuration comparison methodology',
    description: 'Equivalent decoded string maps, pinned libraries and reproduction tools for the Go format comparison.',
  },
  {
    source: 'implementations/benchmarks/native-2026-10-24/README.md',
    slug: 'implementations/benchmarks/native-2026-10-24',
    title: 'Native workload methodology',
    description: 'Shared dotReg fixtures, independent output validation and reproducible native time and process-memory measurements.',
  },
  {
    source: 'implementations/go/README.md',
    slug: 'implementations/go',
    title: 'Go sample implementation',
    description: 'Run the Go parser and CLI, retrieve values by key or prefix and understand provisional choices.',
  },
];
