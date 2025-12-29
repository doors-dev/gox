package lsp

type method string

// notfif client -> gopls

const (
	didOpen   method = "textDocument/didOpen"   // +
	didChange method = "textDocument/didChange" // +
	didSave   method = "textDocument/didSave"   // +
	didClose  method = "textDocument/didClose"  // +
)

// calls client -> gopls
const (
	prepareCallHierarchy method = "textDocument/prepareCallHierarchy" // +
	incomingCalls        method = "callHierarchy/incomingCalls"       // +
	outgoingCalls        method = "callHierarchy/outgoingCalls"       // +
	resolveCodeAction    method = "codeAction/resolve"                // +

	exit        method = "exit"        // -
	initialize  method = "initialize"  // + TODO: try to force utf-8, handle encoding in responce
	initialized method = "initialized" // -
	shutdown    method = "shutdown"    // -

	codeAction           method = "textDocument/codeAction"           // +
	codeLens             method = "textDocument/codeLens"             // +
	completion           method = "textDocument/completion"           // +
	definition           method = "textDocument/definition"           // +
	diagnostic           method = "textDocument/diagnostic"           // + TODO: unchecked
	documentHighlight    method = "textDocument/documentHighlight"    // + TODO: gox highlight?
	documentLink         method = "textDocument/documentLink"         // + TODO: unchecked
	documentSymbol       method = "textDocument/documentSymbol"       // +
	foldingRange         method = "textDocument/foldingRange"         // + TODO: gox
	formatting           method = "textDocument/formatting"           // +
	hover                method = "textDocument/hover"                // +
	implementation       method = "textDocument/implementation"       // +
	inlayHint            method = "textDocument/inlayHint"            // + TODO: unchecked
	prepareRename        method = "textDocument/prepareRename"        // +
	prepareTypeHierarchy method = "textDocument/prepareTypeHierarchy" // + TODO: unchecked
	references           method = "textDocument/references"           // +
	rename               method = "textDocument/rename"               // +
	selectionRange       method = "textDocument/selectionRange"
	semanticTokensFull   method = "textDocument/semanticTokens/full"  // + TODO: gox
	semanticTokensRange  method = "textDocument/semanticTokens/range" // + TODO: gox
	signatureHelp        method = "textDocument/signatureHelp"
	typeDefinition       method = "textDocument/typeDefinition" // + TODO: unchecked

	subtypes   method = "typeHierarchy/subtypes"
	supertypes method = "typeHierarchy/supertypes"

	workDoneProgressCancel method = "window/workDoneProgress/cancel" // -

	didChangeConfiguration    method = "workspace/didChangeConfiguration"
	didChangeWatchedFiles     method = "workspace/didChangeWatchedFiles"
	didChangeWorkspaceFolders method = "workspace/didChangeWorkspaceFolders"
	didCreateFiles            method = "workspace/didCreateFiles"
	executeCommand            method = "workspace/executeCommand"
	symbol                    method = "workspace/symbol"
)

/* not implemented by gopls: */
/*
	colorPresentation   method = "textDocument/colorPresentation"
	declaration         method = "textDocument/declaration"
	diagnosticWorkspace method = "workspace/diagnostic"

	didChangeNotebookDocument method = "notebookDocument/didChange"
	didCloseNotebookDocument  method = "notebookDocument/didClose"
	didDeleteFiles            method = "workspace/didDeleteFiles"
	didOpenNotebookDocument   method = "notebookDocument/didOpen"
	didRenameFiles            method = "workspace/didRenameFiles"
	didSaveNotebookDocument   method = "notebookDocument/didSave"

	documentColor method = "textDocument/documentColor"

	inlineCompletion method = "textDocument/inlineCompletion"
	inlineValue      method = "textDocument/inlineValue"

	linkedEditingRange method = "textDocument/linkedEditingRange"

	moniker method = "textDocument/moniker"

	onTypeFormatting method = "textDocument/onTypeFormatting"

	progress method = "$/progress"

	rangeFormatting  method = "textDocument/rangeFormatting"
	rangesFormatting method = "textDocument/rangesFormatting"

	resolve                method = "inlayHint/resolve"
	resolveCodeLens        method = "codeLens/resolve"
	resolveCompletionItem  method = "completionItem/resolve"
	resolveDocumentLink    method = "documentLink/resolve"
	resolveWorkspaceSymbol method = "workspaceSymbol/resolve"

	semanticTokensFullDelta method = "textDocument/semanticTokens/full/delta"

	setTrace method = "$/setTrace"

	willCreateFiles method = "workspace/willCreateFiles"
	willDeleteFiles method = "workspace/willDeleteFiles"
	willRenameFiles method = "workspace/willRenameFiles"

	willSave          method = "textDocument/willSave"
	willSaveWaitUntil method = "textDocument/willSaveWaitUntil"

	textDocumentContent method = "workspace/textDocumentContent"
*/

// calls gopls -> client

const (
	applyEdit method = "workspace/applyEdit"
)

// notfif gopls -> client

const (
	windowShowMessage  method = "window/showMessage"
	publishDiagnostics method = "textDocument/publishDiagnostics"
)
