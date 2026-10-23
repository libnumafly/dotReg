import { copyFile, mkdir, readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import { pages, repositoryRoot, repositoryUrl, siteBase, siteUrl, pagesRoot } from './content.mjs';

const contentRoot = path.join(pagesRoot, 'src/content/docs');

async function writeIfChanged(destination, content) {
  await mkdir(path.dirname(destination), { recursive: true });
  let previous;
  try {
    previous = await readFile(destination, 'utf8');
  } catch (error) {
    if (error.code !== 'ENOENT') throw error;
  }
  if (previous !== content) await writeFile(destination, content, 'utf8');
}

export async function prepareContent() {
  for (const page of pages) {
    const source = await readFile(path.join(repositoryRoot, page.source), 'utf8');
    // Starlight supplies the page title and a generated, accessible contents menu.
    // All code blocks, including meaningful whitespace, stay in their original form.
    const body = source
      .replace(/^# [^\r\n]+\r?\n(?:\r?\n)?/, '')
      .replace(/^## Contents\r?\n[\s\S]*?(?=^## )/m, '');
    const metadata = [
      '---',
      `title: ${JSON.stringify(page.title)}`,
      `description: ${JSON.stringify(page.description)}`,
      `editUrl: ${JSON.stringify(`${repositoryUrl}/edit/main/${page.source}`)}`,
      '---',
      '',
    ].join('\n');
    await writeIfChanged(path.join(contentRoot, `${page.slug}.md`), metadata + body);
  }

  await writeIfChanged(
    path.join(contentRoot, 'index.mdx'),
    await readFile(path.join(pagesRoot, 'content/index.mdx'), 'utf8'),
  );

  await writeIfChanged(path.join(contentRoot, '404.md'), [
    '---',
    'title: Page not found',
    'template: splash',
    // The dedicated 404 route reads this entry directly. Exclude it from the
    // ordinary production content routes and from the search index.
    'draft: true',
    'pagefind: false',
    'editUrl: false',
    'hero:',
    '  title: This path does not lead to a page.',
    '  tagline: Find your way back to the dotReg documentation.',
    '  actions:',
    '    - text: Back to dotReg',
    `      link: ${siteBase}`,
    '      icon: right-arrow',
    'head:',
    '  - tag: meta',
    '    attrs: { name: robots, content: noindex }',
    '  - tag: link',
    `    attrs: { rel: canonical, href: '${siteUrl}${siteBase}404.html' }`,
    '---',
    '',
  ].join('\n'));

  const sample = await readFile(path.join(repositoryRoot, 'examples/sample.txt'), 'utf8');
  await writeIfChanged(path.join(contentRoot, 'examples.md'), [
    '---',
    'title: Example configuration',
    'description: A complete dotReg input with repeated keys, quoted names and multiline values.',
    `editUrl: ${repositoryUrl}/edit/main/examples/sample.txt`,
    '---',
    '',
    'This complete file starts with the required declaration and blank separator. It demonstrates repeated values, quoting, both multiline forms and unset values.',
    '',
    '**Experimental types.** The `string`, `int` and `bool` annotations belong to the Go sample’s demo profile. Standard type names and conversion rules remain undecided.',
    '',
    'See [type profiles and candidate types](../type-profile/) for type-specific representations and candidates, and [undecided matters](../open-issues/) for the remaining decisions.',
    '',
    '[Download sample.txt](../examples/sample.txt) · [Run the Go sample](../implementations/go/)',
    '',
    '```text',
    sample.trimEnd(),
    '```',
    '',
  ].join('\n'));

  const license = await readFile(path.join(repositoryRoot, 'LICENSE'), 'utf8');
  await writeIfChanged(path.join(contentRoot, 'license.md'), [
    '---',
    'title: Licence',
    'description: The MIT License for the dotReg specification, examples and sample implementation.',
    'editUrl: false',
    '---',
    '',
    'The specification, examples and sample implementation are available under the MIT License. You may choose your own licence for configuration files you author and independently written implementations.',
    '',
    '[Download the licence](../LICENSE.txt)',
    '',
    '```text',
    license.trimEnd(),
    '```',
    '',
  ].join('\n'));

  await mkdir(path.join(pagesRoot, 'public/examples'), { recursive: true });
  await copyFile(path.join(repositoryRoot, 'examples/sample.txt'), path.join(pagesRoot, 'public/examples/sample.txt'));
  await copyFile(path.join(repositoryRoot, 'LICENSE'), path.join(pagesRoot, 'public/LICENSE.txt'));
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  await prepareContent();
  console.log(`Prepared ${pages.length + 4} pages from the repository sources.`);
}
