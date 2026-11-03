// A test extension that touches the API surface the terminal app maps.
const vscode = require('vscode');

exports.activate = function (context) {
  console.log('sample activating');
  const diags = vscode.languages.createDiagnosticCollection('sample');
  const lint = doc => {
    if (doc.languageId !== 'plaintext') return;
    const out = [];
    for (let i = 0; i < doc.lineCount; i++) {
      const t = doc.lineAt(i).text, c = t.indexOf('TODO');
      if (c >= 0) out.push(new vscode.Diagnostic(new vscode.Range(i, c, i, c + 4), 'a TODO is left', vscode.DiagnosticSeverity.Warning));
    }
    diags.set(doc.uri, out);
  };
  vscode.workspace.textDocuments.forEach(lint);
  context.subscriptions.push(
    vscode.workspace.onDidOpenTextDocument(lint),
    vscode.workspace.onDidChangeTextDocument(e => lint(e.document)),
    vscode.commands.registerCommand('sample.hello', async () => {
      const greeting = vscode.workspace.getConfiguration('sample').get('greeting');
      const pick = await vscode.window.showInformationMessage(greeting + ' from sample', 'OK', 'Cancel');
      return pick;
    }),
    vscode.languages.registerCompletionItemProvider('plaintext', {
      provideCompletionItems(doc, pos) {
        const a = new vscode.CompletionItem('alpha', vscode.CompletionItemKind.Keyword);
        const b = new vscode.CompletionItem('betaFunc', vscode.CompletionItemKind.Function);
        b.insertText = new vscode.SnippetString('betaFunc(${1:x})');
        return [a, b];
      },
    }, '.'),
    vscode.languages.registerHoverProvider('plaintext', {
      provideHover(doc, pos) {
        const w = doc.getWordRangeAtPosition(pos);
        return w ? new vscode.Hover(new vscode.MarkdownString('**' + doc.getText(w) + '**')) : undefined;
      },
    }),
    vscode.languages.registerDocumentFormattingEditProvider('plaintext', {
      provideDocumentFormattingEdits(doc) {
        const edits = [];
        for (let i = 0; i < doc.lineCount; i++) {
          const l = doc.lineAt(i), t = l.text.replace(/\s+$/, '');
          if (t !== l.text) edits.push(vscode.TextEdit.replace(new vscode.Range(i, t.length, i, l.text.length), ''));
        }
        return edits;
      },
    }),
  );
  context.subscriptions.push(vscode.commands.registerCommand('sample.panel', () => {
    const panel = vscode.window.createWebviewPanel('sample.view', 'Sample Panel', vscode.ViewColumn.One, { enableScripts: true });
    panel.webview.onDidReceiveMessage(m => panel.webview.postMessage({ echo: m }));
    panel.webview.html = '<html><head><meta http-equiv="Content-Security-Policy" content="default-src \'none\'; script-src ' +
      panel.webview.cspSource + '"></head><body><img src="' + panel.webview.asWebviewUri(vscode.Uri.joinPath(context.extensionUri, 'package.json')) + '"></body></html>';
    return 'panel';
  }));
  const sb = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 10);
  sb.text = '$(check) sample ready';
  sb.show();
  return { answer: 42 };
};
