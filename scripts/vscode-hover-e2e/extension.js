const vscode = require('vscode');

/**
 * @param {vscode.ExtensionContext} context
 */
function activate(context) {
  context.subscriptions.push(
    vscode.commands.registerCommand('errsweep.runHoverE2E', async () => {
      const doc = await vscode.workspace.openTextDocument(
        vscode.Uri.file(process.env.ERRSWEEP_HOVER_FILE),
      );
      const editor = await vscode.window.showTextDocument(doc, { preview: false });
      const pos = new vscode.Position(
        Number(process.env.ERRSWEEP_HOVER_LINE || '0'),
        Number(process.env.ERRSWEEP_HOVER_CHAR || '0'),
      );
      editor.selection = new vscode.Selection(pos, pos);

      // Wait for LSP readiness in fresh profiles.
      await new Promise((r) => setTimeout(r, 2500));

      let lastText = '';
      for (let i = 0; i < 20; i++) {
        const hovers = await vscode.commands.executeCommand(
          'vscode.executeHoverProvider',
          doc.uri,
          pos,
        );
        const text = (hovers || [])
          .flatMap((h) => h.contents || [])
          .map((c) => {
            if (typeof c === 'string') return c;
            if (c && typeof c.value === 'string') return c.value;
            if (c && typeof c.language === 'string' && typeof c.value === 'string') return c.value;
            return '';
          })
          .join('\n');
        lastText = text;
        if (
          text.includes('Possible Sentinel Errors') &&
          text.includes(process.env.ERRSWEEP_EXPECT_SENTINEL || 'ErrNotFound')
        ) {
          await vscode.commands.executeCommand('workbench.action.closeWindow');
          return;
        }
        await new Promise((r) => setTimeout(r, 500));
      }

      vscode.window.showErrorMessage(`Hover E2E failed: ${lastText}`);
      await vscode.commands.executeCommand('workbench.action.closeWindow');
      process.exitCode = 1;
    }),
  );

  // Auto-run once VS Code starts.
  setTimeout(() => {
    vscode.commands.executeCommand('errsweep.runHoverE2E');
  }, 500);
}

function deactivate() {}

module.exports = {
  activate,
  deactivate,
};
