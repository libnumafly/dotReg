'use strict';

const vscode = require('vscode');

let support;
const debounceTimers = new Map();
// Suggestions are examples for editing only; the syntax layer does not validate
// type names or perform their conversions.
const TYPE_CANDIDATES = [
  ['string', 'Text'],
  ['bool', 'Boolean'],
  ['int32', 'Signed 32-bit integer'],
  ['int64', 'Signed 64-bit integer'],
  ['uint32', 'Unsigned 32-bit integer'],
  ['uint64', 'Unsigned 64-bit integer'],
  ['decimal', 'Decimal number'],
  ['float64', '64-bit floating-point number'],
  ['base64', 'Octets encoded as standard Base64'],
  ['duration', 'Duration'],
  ['date', 'Calendar date'],
  ['time', 'Time of day'],
  ['datetime', 'Date and time'],
  ['uuid', 'UUID'],
  ['ip4', 'IPv4 address'],
  ['ip6', 'IPv6 address'],
  ['cidr4', 'IPv4 network prefix'],
  ['cidr6', 'IPv6 network prefix'],
  ['uri', 'URI'],
  ['int', 'Type used by the Go sample DemoProfile', 'Go sample only; not a standard candidate.'],
];

async function getSupport() {
  support ??= import('./formatter.mjs');
  return support;
}

function diagnosticMessage(item) {
  if (item.code === 'unsupported') return 'This syntax is not supported by the dotReg 0.1 sample implementation.';
  if (item.code === 'duplicate') return 'The @dotReg declaration is duplicated.';
  if (item.code === 'input') return 'The file text could not be read.';
  if (item.severity === 'warning') return item.message;
  return item.message;
}

function asDiagnostic(document, item) {
  const line = Math.min(Math.max(item.line - 1, 0), document.lineCount - 1);
  const range = document.lineAt(line).range;
  const severity = item.severity === 'warning'
    ? vscode.DiagnosticSeverity.Warning
    : vscode.DiagnosticSeverity.Error;
  const diagnostic = new vscode.Diagnostic(range, diagnosticMessage(item), severity);
  diagnostic.source = 'dotReg';
  diagnostic.code = `dotreg.${item.code}`;
  return diagnostic;
}

function formatPath(path) {
  return `[${path.map(element => JSON.stringify(element)).join(', ')}]`;
}

function formatPathElement(element) {
  return element !== '' && !/[.\p{White_Space}\p{Cc}:()#"\\]/u.test(element)
    ? element
    : JSON.stringify(element);
}

function outlineRange(document, startLine, endLine) {
  const first = Math.max(0, Math.min(startLine - 1, document.lineCount - 1));
  const last = Math.max(first, Math.min(endLine - 1, document.lineCount - 1));
  return new vscode.Range(
    new vscode.Position(first, 0),
    new vscode.Position(last, document.lineAt(last).text.length),
  );
}

function buildDocumentSymbols(document, entries) {
  const roots = new Map();
  for (const entry of entries) {
    let siblings = roots;
    let node;
    for (const element of entry.path) {
      if (!siblings.has(element)) {
        siblings.set(element, {
          element,
          children: new Map(),
          headings: [],
          values: [],
          firstLine: entry.line,
          lastLine: entry.line,
        });
      }
      node = siblings.get(element);
      node.firstLine = Math.min(node.firstLine, entry.line);
      node.lastLine = Math.max(node.lastLine, entry.line);
      siblings = node.children;
    }
    if (!node) continue;
    if (entry.kind === 'heading') node.headings.push(entry.line);
    else node.values.push(entry);
  }

  const makeSymbol = (name, detail, kind, startLine, endLine, selectionLine) => {
    const range = outlineRange(document, startLine, endLine);
    const selectionRange = outlineRange(document, selectionLine, selectionLine);
    return new vscode.DocumentSymbol(name, detail, kind, range, selectionRange);
  };

  const valueDetail = entry => {
    const parts = [];
    if (entry.has_type) parts.push(`type: ${entry.type}`);
    if (entry.unset) parts.push('unset');
    if (entry.form) parts.push(entry.form);
    return parts.join(' · ') || 'value';
  };

  const convert = node => {
    const children = [...node.children.values()]
      .map(child => ({ line: child.firstLine, symbol: convert(child) }));
    const hasChildren = children.length > 0;
    const isGroup = hasChildren || node.headings.length > 0 || node.values.length > 1;
    if (node.values.length === 1 && !isGroup) {
      const entry = node.values[0];
      return makeSymbol(
        formatPathElement(node.element),
        valueDetail(entry),
        vscode.SymbolKind.Property,
        entry.line,
        entry.line,
        entry.line,
      );
    }

    for (let i = 0; i < node.values.length; i++) {
      const entry = node.values[i];
      const occurrence = node.values.length > 1 ? ` #${i + 1}` : '';
      children.push({
        line: entry.line,
        symbol: makeSymbol(
          `value${occurrence}`,
          valueDetail(entry),
          vscode.SymbolKind.Field,
          entry.line,
          entry.line,
          entry.line,
        ),
      });
    }
    children.sort((left, right) => left.line - right.line);

    const selectionLine = node.headings.length > 0
      ? Math.min(...node.headings)
      : node.firstLine;
    const group = makeSymbol(
      formatPathElement(node.element),
      node.headings.length > 0 ? 'heading' : 'key group',
      vscode.SymbolKind.Namespace,
      node.firstLine,
      node.lastLine,
      selectionLine,
    );
    group.children = children.map(item => item.symbol);
    return group;
  };

  return [...roots.values()]
    .sort((left, right) => left.firstLine - right.firstLine)
    .map(convert);
}

function inlineCode(value) {
  const runs = value.match(/`+/g) ?? [];
  const delimiter = '`'.repeat(Math.max(1, ...runs.map(run => run.length + 1)));
  return `${delimiter}${value}${delimiter}`;
}

function typeCompletionContext(line, offset) {
  if (/^[ \t]*#/.test(line) || /^[ \t]*@/.test(line) || /^[ \t]*\|/.test(line)) return undefined;
  let quoted = false;
  let depth = 0;
  let typeStart = -1;
  for (let i = 0; i < offset; i++) {
    const char = line[i];
    if (quoted) {
      if (char === '\\') i++;
      else if (char === '"') quoted = false;
      continue;
    }
    if (char === '"') {
      quoted = true;
      continue;
    }
    if (char === ':' && depth === 0) return undefined;
    if (char === '(') {
      if (depth === 0) typeStart = i + 1;
      depth++;
    } else if (char === ')' && depth > 0) {
      depth--;
      if (depth === 0) typeStart = -1;
    }
  }
  if (depth !== 1 || typeStart < 0) return undefined;
  const prefix = line.slice(typeStart, offset);
  if (/[()\p{White_Space}]/u.test(prefix)) return undefined;
  const key = line.slice(0, typeStart - 1).trim();
  if (!key) return undefined;
  return { start: typeStart };
}

function valueColonIndex(line) {
  let quoted = false;
  for (let i = 0; i < line.length; i++) {
    if (quoted) {
      if (line[i] === '\\') i++;
      else if (line[i] === '"') quoted = false;
      continue;
    }
    if (line[i] === '"') quoted = true;
    else if (line[i] === ':') return i;
  }
  return -1;
}

function declarationQuickFix(document, diagnostic) {
  if (diagnostic.code !== 'dotreg.syntax' || diagnostic.range.start.line !== 0) return undefined;
  const first = document.lineAt(0).text.replace(/^\uFEFF/, '');
  if (first.trimStart().startsWith('@')) return undefined;
  const firstBlank = document.getText().split(/\r?\n/).findIndex(line => /^[ \t]*$/.test(line));
  const propertyRegion = firstBlank < 0 ? document.getText() : document.getText().split(/\r?\n/).slice(0, firstBlank).join('\n');
  if (/^[ \t]*@dotReg\b/m.test(propertyRegion)) return undefined;

  const action = new vscode.CodeAction('Insert @dotReg 0.1 declaration', vscode.CodeActionKind.QuickFix);
  action.diagnostics = [diagnostic];
  action.edit = new vscode.WorkspaceEdit();
  const newline = document.eol === vscode.EndOfLine.CRLF ? '\r\n' : '\n';
  action.edit.insert(document.uri, new vscode.Position(0, 0), `@dotReg 0.1${newline}${newline}`);
  return action;
}

async function addTypeAnnotation(uriString, lineNumber) {
  const uri = vscode.Uri.parse(uriString);
  const document = await vscode.workspace.openTextDocument(uri);
  const { parse } = await import('./dotreg.mjs');
  let entry;
  try {
    entry = (await parse(document.getText())).find(item => item.line === lineNumber + 1);
  } catch {
    return;
  }
  if (!entry || entry.kind !== 'value' || entry.has_type) return;

  const line = document.lineAt(lineNumber).text;
  const colon = valueColonIndex(line);
  if (colon < 0) return;
  const options = TYPE_CANDIDATES.map(([name, description, source]) => ({
    label: name,
    description: source ?? 'Illustrative candidate; type meaning and conversion remain undecided.',
    detail: description,
    typeName: name,
  }));
  options.push({
    label: 'Enter a custom type expression…',
    description: 'Type meaning and conversion are consumer-defined',
    typeName: '',
  });
  const choice = await vscode.window.showQuickPick(options, {
    placeHolder: 'Choose a candidate or Go sample type, or enter a custom expression',
    matchOnDetail: true,
  });
  if (!choice) return;

  let typeName = choice.typeName;
  if (!typeName) {
    typeName = await vscode.window.showInputBox({
      prompt: 'Enter a type expression without whitespace. Consumers define its meaning and conversion.',
      placeHolder: 'list(string)',
      validateInput(value) {
        if (!value) return 'Enter a type expression.';
        if (/\p{White_Space}/u.test(value)) return 'Type expressions cannot contain whitespace.';
        return undefined;
      },
    });
    if (typeName === undefined) return;
  }

  const key = line.slice(0, colon).replace(/[ \t]+$/g, '');
  const edit = new vscode.WorkspaceEdit();
  edit.replace(
    uri,
    new vscode.Range(new vscode.Position(lineNumber, 0), new vscode.Position(lineNumber, colon)),
    `${key} (${typeName})`,
  );
  await vscode.workspace.applyEdit(edit);
}

function activate(context) {
  const diagnostics = vscode.languages.createDiagnosticCollection('dotReg');
  context.subscriptions.push(diagnostics);

  const validate = async document => {
    if (document.languageId !== 'dotreg') return;
    const prior = debounceTimers.get(document.uri.toString());
    if (prior) clearTimeout(prior);
    const key = document.uri.toString();
    const timer = setTimeout(async () => {
      debounceTimers.delete(key);
      try {
        const { inspectText } = await getSupport();
        const items = await inspectText(document.getText());
        diagnostics.set(document.uri, items.map(item => asDiagnostic(document, item)));
      } catch {
        diagnostics.delete(document.uri);
      }
    }, 120);
    debounceTimers.set(key, timer);
  };

  context.subscriptions.push(
    vscode.workspace.onDidOpenTextDocument(validate),
    vscode.workspace.onDidChangeTextDocument(event => validate(event.document)),
    vscode.workspace.onDidCloseTextDocument(document => {
      const key = document.uri.toString();
      const timer = debounceTimers.get(key);
      if (timer) clearTimeout(timer);
      debounceTimers.delete(key);
      diagnostics.delete(document.uri);
    }),
  );
  for (const document of vscode.workspace.textDocuments) validate(document);

  context.subscriptions.push(vscode.languages.registerDocumentFormattingEditProvider('dotreg', {
    async provideDocumentFormattingEdits(document) {
      try {
        const { formatText } = await getSupport();
        const formatted = await formatText(document.getText());
        if (formatted === document.getText()) return [];
        const fullRange = new vscode.Range(document.positionAt(0), document.positionAt(document.getText().length));
        return [vscode.TextEdit.replace(fullRange, formatted)];
      } catch {
        // Keep invalid or incomplete buffers intact; diagnostics explain the issue.
        return [];
      }
    },
  }));

  context.subscriptions.push(vscode.languages.registerHoverProvider('dotreg', {
    async provideHover(document, position) {
      try {
        const { parse } = await import('./dotreg.mjs');
        const entries = await parse(document.getText());
        const entry = entries.find(item => item.line === position.line + 1);
        if (!entry) return undefined;
        const markdown = new vscode.MarkdownString();
        if (entry.kind === 'heading') {
          markdown.appendMarkdown(`**dotReg heading**\n\nResolved prefix: ${inlineCode(formatPath(entry.path))}`);
        } else {
          markdown.appendMarkdown(`**dotReg value**\n\nResolved key: ${inlineCode(formatPath(entry.path))}`);
          if (entry.has_type) markdown.appendMarkdown(`\n\nDeclared type: ${inlineCode(entry.type)}`);
          if (entry.has_type) markdown.appendMarkdown('\n\nThe consuming profile defines the type meaning and value conversion. This extension does not perform conversion.');
          if (entry.form) markdown.appendMarkdown(`\n\nValue form: ${inlineCode(entry.form)}`);
          if (entry.unset) markdown.appendMarkdown('\n\nThis occurrence is unset.');
        }
        return new vscode.Hover(markdown);
      } catch {
        return undefined;
      }
    },
  }));

  context.subscriptions.push(vscode.languages.registerDocumentSymbolProvider('dotreg', {
    async provideDocumentSymbols(document) {
      try {
        const { parse } = await import('./dotreg.mjs');
        return buildDocumentSymbols(document, await parse(document.getText()));
      } catch {
        return [];
      }
    },
  }));

  context.subscriptions.push(vscode.languages.registerCompletionItemProvider('dotreg', {
    provideCompletionItems(document, position) {
      const line = document.lineAt(position.line).text;
      const contextAtPosition = typeCompletionContext(line, position.character);
      if (contextAtPosition) {
        return TYPE_CANDIDATES.map(([name, description, source]) => {
          const item = new vscode.CompletionItem(name, vscode.CompletionItemKind.TypeParameter);
          item.insertText = name;
          item.range = new vscode.Range(
            new vscode.Position(position.line, contextAtPosition.start),
            position,
          );
          item.detail = source ?? 'Illustrative candidate; type meaning and conversion remain undecided.';
          item.documentation = new vscode.MarkdownString(`${description}\n\nIllustrative suggestion only. The syntax layer accepts type annotations but does not validate names or convert values. Consumers define type meaning and conversion rules.`);
          return item;
        });
      }

      if (position.line !== 0 || document.getText().trim() !== '') return undefined;
      const header = new vscode.CompletionItem('@dotReg 0.1', vscode.CompletionItemKind.Snippet);
      header.insertText = new vscode.SnippetString('@dotReg 0.1\n\n$0');
      header.detail = 'Required dotReg file declaration';
      return [header];
    },
  }, '('));

  context.subscriptions.push(vscode.languages.registerCodeActionsProvider('dotreg', {
    async provideCodeActions(document, range, actionContext) {
      const actions = actionContext.diagnostics
        .map(diagnostic => declarationQuickFix(document, diagnostic))
        .filter(Boolean);
      try {
        const { parse } = await import('./dotreg.mjs');
        const entry = (await parse(document.getText())).find(item => item.line === range.start.line + 1);
        if (entry?.kind === 'value' && !entry.has_type) {
          const action = new vscode.CodeAction('Add type annotation from provisional suggestions…', vscode.CodeActionKind.QuickFix);
          action.command = {
            command: 'dotreg.addTypeAnnotation',
            title: 'Add type annotation',
            arguments: [document.uri.toString(), range.start.line],
          };
          actions.push(action);
        }
      } catch {
        // Do not offer type edits while the document has structural errors.
      }
      return actions;
    },
  }, { providedCodeActionKinds: [vscode.CodeActionKind.QuickFix] }));

  context.subscriptions.push(vscode.commands.registerCommand('dotreg.addTypeAnnotation', addTypeAnnotation));
  context.subscriptions.push(vscode.commands.registerCommand('dotreg.formatDocument', () =>
    vscode.commands.executeCommand('editor.action.formatDocument')));
}

function deactivate() {
  for (const timer of debounceTimers.values()) clearTimeout(timer);
  debounceTimers.clear();
}

module.exports = { activate, deactivate };
