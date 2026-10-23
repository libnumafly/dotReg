import assert from 'node:assert/strict';
import { readFile, readdir, stat } from 'node:fs/promises';
import path from 'node:path';
import { parse } from 'parse5';
import { pages, repositoryRoot, siteBase, siteUrl, pagesRoot } from './content.mjs';

const outputRoot = path.join(pagesRoot, 'dist');
const documents = new Map();
const failures = [];
let checkedLinks = 0;
let checkedAnchors = 0;

async function htmlFiles(directory) {
  const result = [];
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const filename = path.join(directory, entry.name);
    if (entry.isDirectory()) result.push(...await htmlFiles(filename));
    else if (entry.name.endsWith('.html')) result.push(filename);
  }
  return result;
}

function walk(node, callback) {
  callback(node);
  for (const child of node.childNodes ?? []) walk(child, callback);
}

function textContent(node) {
  return node.nodeName === '#text' ? node.value : (node.childNodes ?? []).map(textContent).join('');
}

function renderedCodeBlocks(document) {
  const blocks = [];
  walk(document.ast, (node) => {
    if (node.tagName !== 'pre') return;
    const lines = [];
    walk(node, (child) => {
      if (child.attrs?.some(({ name, value }) => name === 'class' && value.split(' ').includes('code'))) {
        const text = textContent(child);
        lines.push(text === '\n' ? '' : text);
      }
    });
    if (lines.length) blocks.push(lines.join('\n'));
  });
  return blocks;
}

for (const filename of await htmlFiles(outputRoot)) {
  const document = { ids: new Set(), links: [], ast: parse(await readFile(filename, 'utf8')) };
  walk(document.ast, (node) => {
    for (const { name, value } of node.attrs ?? []) {
      if (name === 'id') document.ids.add(value);
      if (['href', 'src'].includes(name)) document.links.push(value);
    }
  });
  documents.set(filename, document);
}

assert(documents.size >= pages.length + 3, 'Expected all documentation pages in dist; run npm run build first.');
for (const [filename, document] of documents) {
  const relativePath = path.relative(outputRoot, filename).replaceAll('\\', '/');
  const pageUrl = new URL(siteBase + relativePath.replace(/index\.html$/, ''), siteUrl);
  for (const link of document.links) {
    const target = new URL(link, pageUrl);
    if (target.origin !== siteUrl) continue;
    checkedLinks++;
    if (!target.pathname.startsWith(siteBase)) {
      failures.push(`${relativePath}: link is outside ${siteBase}: ${link}`);
      continue;
    }
    const targetPath = decodeURIComponent(target.pathname.slice(siteBase.length));
    const targetFile = path.join(outputRoot, targetPath, target.pathname.endsWith('/') ? 'index.html' : '');
    if (!targetFile.startsWith(outputRoot + path.sep)) {
      failures.push(`${relativePath}: invalid path: ${link}`);
      continue;
    }
    try {
      if (!(await stat(targetFile)).isFile()) throw new Error('Not a file');
    } catch {
      failures.push(`${relativePath}: missing local file: ${link}`);
      continue;
    }
    if (target.hash && documents.has(targetFile)) {
      checkedAnchors++;
      if (!documents.get(targetFile).ids.has(decodeURIComponent(target.hash.slice(1)))) {
        failures.push(`${relativePath}: missing heading anchor: ${link}`);
      }
    }
  }
}

function fencedBlocks(markdown) {
  const result = [];
  let fence;
  let lines = [];
  for (const line of markdown.replaceAll('\r\n', '\n').split('\n')) {
    if (!fence) {
      const opening = /^(\x60{3,}|~{3,})/.exec(line);
      if (opening) { fence = opening[1]; lines = []; }
    } else if (new RegExp(`^${fence[0]}{${fence.length},}\\s*$`).test(line)) {
      result.push(lines.join('\n'));
      fence = undefined;
    } else {
      lines.push(line);
    }
  }
  return result;
}

let checkedBlocks = 0;
for (const page of pages) {
  const source = await readFile(path.join(repositoryRoot, page.source), 'utf8');
  const staged = await readFile(path.join(pagesRoot, 'src/content/docs', `${page.slug}.md`), 'utf8');
  const originalBlocks = fencedBlocks(source);
  assert.deepEqual(fencedBlocks(staged), originalBlocks, `${page.source}: a code example changed during preparation`);
  assert.deepEqual(
    renderedCodeBlocks(documents.get(path.join(outputRoot, page.slug, 'index.html'))),
    originalBlocks,
    `${page.source}: a rendered code example differs from its source`,
  );
  checkedBlocks += originalBlocks.length;
}

assert.deepEqual(
  await readFile(path.join(outputRoot, 'examples/sample.txt')),
  await readFile(path.join(repositoryRoot, 'examples/sample.txt')),
  'The downloadable sample must match its source byte for byte',
);
assert.deepEqual(
  await readFile(path.join(outputRoot, 'LICENSE.txt')),
  await readFile(path.join(repositoryRoot, 'LICENSE')),
  'The downloadable licence must match its source byte for byte',
);
assert(documents.get(path.join(outputRoot, 'specification/index.html'))?.ids.has('12-undecided-matters'), 'The undecided matters section must remain reachable');
assert(documents.has(path.join(outputRoot, 'open-issues/index.html')), 'The undecided matters page must remain reachable');
const typeProfile = documents.get(path.join(outputRoot, 'type-profile/index.html'));
assert(typeProfile?.ids.has('established-base64-representation'), 'The established base64 representation must remain reachable');
assert(typeProfile?.ids.has('candidate-type-names-and-sample-values'), 'The candidate type catalogue must remain reachable');
assert((await readdir(path.join(outputRoot, 'pagefind'))).some((name) => name.endsWith('.js')), 'The search index is missing');

if (failures.length) {
  console.error(failures.join('\n'));
  throw new Error(`${failures.length} broken local links or heading anchors`);
}
console.log(`Checked ${documents.size} HTML pages, ${checkedLinks} local links, ${checkedAnchors} anchors and ${checkedBlocks} unchanged source code blocks; downloads and search index match.`);
