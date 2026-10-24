# dotReg documentation website

The documentation site uses [Astro Starlight](https://starlight.astro.build/) and
builds static files for `https://libnumafly.github.io/dotReg/`.

## Local development

Use Node.js 24.15 or later and npm. Run from `pages/`:

```sh
npm ci
npm run dev
```

Open the URL printed by Astro, normally `http://127.0.0.1:4321/dotReg/`.
Changes to the original Markdown files, the example configuration and the landing
page are picked up by the development server.

For a production build and local preview:

```sh
npm run build
npm run preview
```

The build includes local link and heading-anchor checks, verifies that code
examples are unchanged during preparation, compares the downloadable example and
licence with the original files byte for byte, and checks for the search index.
It does not validate the contents or availability of external websites.

## Edit the sources

The site imports these repository files rather than maintaining separate copies:

| Source | Site path beneath `/dotReg/` |
|---|---|
| `../README.md` | `overview/` |
| `../docs/README.md` | `documentation/` |
| `../docs/getting-started.md` | `getting-started/` |
| `../docs/config-format.md` | `specification/` |
| `../docs/type-profile.md` | `type-profile/` |
| `../docs/open-issues.md` | `open-issues/` |
| `../docs/conformance.md` | `conformance/` |
| `../implementations/README.md` | `implementations/` |
| `../implementations/go/README.md` | `implementations/go/` |
| `../implementations/python/README.md` | `implementations/python/` |
| `../implementations/javascript/README.md` | `implementations/javascript/` |
| `../implementations/typescript/README.md` | `implementations/typescript/` |
| `../implementations/java/README.md` | `implementations/java/` |
| `../implementations/csharp/README.md` | `implementations/csharp/` |
| `../implementations/rust/README.md` | `implementations/rust/` |
| `../implementations/cpp/README.md` | `implementations/cpp/` |
| `../implementations/benchmarks/README.md` | `implementations/benchmarks/` |
| `../implementations/benchmarks/public-rollin-2026-10-24/README.md` | `implementations/benchmarks/public-rollin-2026-10-24/` |
| `../implementations/benchmarks/multiline-allocation-2026-10-24/README.md` | `implementations/benchmarks/multiline-allocation-2026-10-24/` |
| `../implementations/benchmarks/comparison/README.md` | `implementations/benchmarks/comparison/` |
| `../implementations/benchmarks/native-2026-10-24/README.md` | `implementations/benchmarks/native-2026-10-24/` |
| `../examples/sample.txt` | `examples/` and the original `.txt` download |
| `../LICENSE` | `license/` and the original text download |

Edit the landing page in `content/index.mdx`, the sidebar and site metadata in
`astro.config.mjs`, and presentation rules in `src/styles/custom.css`.

`scripts/prepare-content.mjs` adds Starlight frontmatter to generated pages under
`src/content/docs/`. It removes each document's first title and manual contents
list because Starlight renders these automatically. The original source files,
section numbers and code examples retain their existing form. Generated pages
and copied downloads are ignored by Git; do not edit them directly.

`scripts/remark-repository-links.mjs` rewrites links at render time. Links to
published documents point to their site routes, including the existing heading
anchors. Links to implementation source and benchmark reports
point to the corresponding files on GitHub. The source Markdown continues to
work when read on GitHub.

When adding a document, register its source and route in `scripts/content.mjs`
and add its sidebar entry in `astro.config.mjs`.

## Publish with GitHub Pages

1. In `libnumafly/dotReg`, open **Settings → Pages** and choose **GitHub Actions**
   as the build and deployment source. This is a one-time repository setting.
2. Push the website configuration to `main`, or run the **Documentation** workflow
   manually from the Actions tab on `main`.
3. Check that both the build and deployment jobs succeed. The deployment job
   reports the public URL in the `github-pages` environment.

`.github/workflows/docs.yml` builds pull requests without deploying them. Pushes
to `main` and manual runs build and publish `pages/dist/`. The [Go workflow](../.github/workflows/ci.yml)
checks the existing Go implementation; the [parser samples workflow](../.github/workflows/parsers.yml)
checks all eight language entries and the comparison harness's semantic equivalence
and smoke workloads, and verifies the published tuning and public sample adoption
tables against their recorded observations. The Go table check is
`python implementations/benchmarks/multiline-allocation-2026-10-24/render.py --check`;
it checks all three paired campaigns and their separately retained raw output.
The adoption table check is
`python implementations/benchmarks/public-rollin-2026-10-24/render.py --check`;
it validates the recorded tables without running measurements.
Dependencies are pinned in `package-lock.json`,
and Dependabot checks the website's npm dependencies weekly.

The site is configured with `site: https://libnumafly.github.io` and
`base: /dotReg/`. If the repository name, owner or public domain changes, update
the constants in `scripts/content.mjs` and the references in this guide before
publishing.
