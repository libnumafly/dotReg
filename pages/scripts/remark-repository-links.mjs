import path from 'node:path';
import { pages, repositoryUrl, siteBase, pagesRoot } from './content.mjs';

const sourceRoutes = new Map(pages.map((page) => [page.source, `${siteBase}${page.slug}/`]));
sourceRoutes.set('examples/sample.txt', `${siteBase}examples/`);
sourceRoutes.set('LICENSE', `${siteBase}license/`);

// Work on Markdown's link nodes so examples containing Markdown-like text are
// never rewritten. GitHub and the published site keep the same source files.
export default function repositoryLinks() {
  return (tree, file) => {
    const generatedPath = path.relative(path.join(pagesRoot, 'src/content/docs'), file.path).replaceAll('\\', '/');
    const page = pages.find((candidate) => generatedPath === `${candidate.slug}.md`);
    if (!page) return;

    function visit(node) {
      if (['link', 'definition', 'image'].includes(node.type) && node.url && !/^(?:[a-z][a-z\d+.-]*:|[/#?])/i.test(node.url)) {
        const match = /^([^?#]+)(.*)$/.exec(node.url);
        if (match) {
          const source = path.posix.normalize(path.posix.join(path.posix.dirname(page.source), decodeURIComponent(match[1])));
          const route = sourceRoutes.get(source);
          if (route) {
            node.url = route + match[2];
          } else {
            if (source.startsWith('../')) throw new Error(`Link escapes the repository: ${node.url} in ${page.source}`);
            node.url = `${repositoryUrl}/blob/main/${source.split('/').map(encodeURIComponent).join('/')}${match[2]}`;
          }
        }
      }
      for (const child of node.children ?? []) visit(child);
    }
    visit(tree);
  };
}
