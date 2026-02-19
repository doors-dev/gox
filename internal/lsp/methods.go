package lsp

type method string

// notifs client -> gopls

const (
	didOpen                   method = "textDocument/didOpen"                // +
	didChange                 method = "textDocument/didChange"              // +
	didSave                   method = "textDocument/didSave"                // +
	didClose                  method = "textDocument/didClose"               // +
	didChangeConfiguration    method = "workspace/didChangeConfiguration"    // skip
	didChangeWatchedFiles     method = "workspace/didChangeWatchedFiles"     //  add support?
	didChangeWorkspaceFolders method = "workspace/didChangeWorkspaceFolders" // +
	didCreateFiles            method = "workspace/didCreateFiles"            // add support?
)

// calls client -> gopls
const (
	prepareCallHierarchy method = "textDocument/prepareCallHierarchy" // +
	incomingCalls        method = "callHierarchy/incomingCalls"       // +
	outgoingCalls        method = "callHierarchy/outgoingCalls"       // +
	resolveCodeAction    method = "codeAction/resolve"                // +

	exit        method = "exit"        // skip
	initialize  method = "initialize"  // +
	initialized method = "initialized" // skip
	shutdown    method = "shutdown"    // skip

	codeAction           method = "textDocument/codeAction"           // +
	codeLens             method = "textDocument/codeLens"             // +
	completion           method = "textDocument/completion"           // +
	definition           method = "textDocument/definition"           // +
	diagnostic           method = "textDocument/diagnostic"           // +
	documentHighlight    method = "textDocument/documentHighlight"    // + TODO: add gox?
	documentLink         method = "textDocument/documentLink"         // +
	documentSymbol       method = "textDocument/documentSymbol"       // +
	foldingRange         method = "textDocument/foldingRange"         // + TODO: gox
	formatting           method = "textDocument/formatting"           // +
	hover                method = "textDocument/hover"                // +
	implementation       method = "textDocument/implementation"       // +
	inlayHint            method = "textDocument/inlayHint"            // +
	prepareRename        method = "textDocument/prepareRename"        // +
	prepareTypeHierarchy method = "textDocument/prepareTypeHierarchy" // +
	references           method = "textDocument/references"           // +
	rename               method = "textDocument/rename"               // +
	selectionRange       method = "textDocument/selectionRange"       // TODO: add gox?
	semanticTokensFull   method = "textDocument/semanticTokens/full"  // + TODO: gox
	semanticTokensRange  method = "textDocument/semanticTokens/range" // + TODO: gox
	signatureHelp        method = "textDocument/signatureHelp"        // +
	typeDefinition       method = "textDocument/typeDefinition"       // +
	subtypes             method = "typeHierarchy/subtypes"            // +
	supertypes           method = "typeHierarchy/supertypes"          // +
	executeCommand       method = "workspace/executeCommand"          // skip
	symbol               method = "workspace/symbol"                  // +

	workDoneProgressCancel method = "window/workDoneProgress/cancel" // skip

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

// notifs gopls -> client
const (
	logTrace                   method = "$/logTrace"                            // skip
	progress                   method = "$/progress"                            // skip
	registerCapability         method = "client/registerCapability"             // skip
	unregisterCapability       method = "client/unregisterCapability"           // skip
	event                      method = "telemetry/event"                       // skip
	publishDiagnostics         method = "textDocument/publishDiagnostics"       // + TODO: add gox
	logMessage                 method = "window/logMessage"                     // skip
	showMessage                method = "window/showMessage"                    // skip
	workDoneProgressCreate     method = "window/workDoneProgress/create"        // skip
	codeLensRefresh            method = "workspace/codeLens/refresh"            // skip
	diagnosticRefresh          method = "workspace/diagnostic/refresh"          // skip
	foldingRangeRefresh        method = "workspace/foldingRange/refresh"        //	skip
	inlayHintRefresh           method = "workspace/inlayHint/refresh"           // skip
	inlineValueRefresh         method = "workspace/inlineValue/refresh"         // skip
	semanticTokensRefresh      method = "workspace/semanticTokens/refresh"      // skip
	textDocumentContentRefresh method = "workspace/textDocumentContent/refresh" //skip
)

// calls gopls -> client
const (
	showDocument       method = "window/showDocument"        // skip
	showMessageRequest method = "window/showMessageRequest"  // skip
	applyEdit          method = "workspace/applyEdit"        // +
	configuration      method = "workspace/configuration"    // skip
	workspaceFolders   method = "workspace/workspaceFolders" // +
)
