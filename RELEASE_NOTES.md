# v0.4.0

## Breaking

- The legacy `~{ ... }` and `func { ... }` forms are removed (tree-sitter-gox v0.3.0). They are now syntax errors, and `gox fmt` no longer converts them. Before upgrading, run `gox fmt` from v0.3.5 to convert them to `~~ ... ~~` and `~({ ... })`, and in attributes to `attr=({ ... })` (no `~`).
- Implicit closing follows HTML more closely: `<p>` is now closed by `<ul>`, `<table>`, `<menu>`, `<dialog>`, `<hgroup>`, `<li>`, `<dd>`, `<dt>`, `<summary>` and `<search>`; a heading closes an open heading (`<h1>a<h2>b</h2>` puts the `<h2>` after the `<h1>`); `<caption>`, `<colgroup>`, `<col>`, `<thead>`, `<tbody>` and `<tfoot>` close an open cell, row, section or `<caption>`, and `<tr>`, `<td>` and `<th>` also close an open `<caption>`; `<option>` and `<optgroup>` are closed by `<hr>`. End tags of the elements closed this way no longer match, as with `</p>` below. v0.3.5 `gox fmt` itself wrote `<table><tr><td>b<colgroup><col></colgroup></td></tr></table>` for an unclosed cell; the `<colgroup>` now closes the cell and row, so it renders after the row, the stray `</td></tr>` is dropped, and `gox fmt` rewrites it to `<table><tr><td>b</td></tr><colgroup><col></colgroup></table>`. When the closed row or cell is not inside another element (directly in an elem body, in `~( )` or in an HTML literal in Go code), the stray end tag is a syntax error: `<tr><td>a<caption>c</caption></td></tr>` now fails gen, and v0.3.5 `gox fmt` wrote that form for `<tr><td>a<caption>c</caption>`.
- A `</p>` after such an element no longer matches the `<p>`. If the `<p>` is not inside another element (directly in an elem body, in `~( )` or in an HTML literal in Go code), that is a syntax error, the same as `<p><div>...</div></p>` already was; inside another element the stray `</p>` is dropped, and `gox fmt` removes it. When the stray end tag sits between two pieces of text, removing it can join them and change the rendered whitespace: `Outro</p>` followed by `Footer` renders `OutroFooter` before `gox fmt` and `Outro Footer` after. v0.3.5 `gox fmt` itself wrote `<p>...<ul>...</ul></p>` for an unclosed top-level `<p>` before a list, so such files now fail. Close the `<p>` before the block element.
- End tags match open elements by their full name, so a misspelled end tag of a custom element is ignored like that of a built-in one. It used to close the nearest custom element or be a syntax error. It now gets a warning (see Added).

## Added

- Go 1.27 generic methods, plain (`func (s Store) Get[T any]() T`) and elem (`elem (t Table) Row[T any](v T)`), and generic method values with explicit type arguments (`h.C.To[int, string]`). Generic methods need `go 1.27` in go.mod, editor support needs gopls built with Go 1.27, and `gox fmt` of `.go` files with generic methods needs gox built with Go 1.27, as the release binaries are.
- LSP: a warning on end tags that match no open element: "`</x>` does not match an open element and is ignored."

## Changed

- gen and LSP: a `.x.go` written by a newer GoX is now refused only when its major.minor version is higher. Files from another patch release are regenerated like any other stale file.

## Removed

- LSP: gox's own code actions Extract variable, Extract constant, Extract to elem and Extract to a new file are removed. gopls's extract actions were already not offered on `.gox`, so `.gox` files now get no extract actions.

## Fixed

- Formatter: a composite literal spread over several lines got no comma after its last element, also when a comment followed that element, so the output did not compile.
- Formatter: no space was put after commas in type parameter and type argument lists (`[K, V any]`, `Map[string, int]`) and name lists (`var x, y int`, struct fields `A, B int`, `case int, string`), or around `|` in constraints, and the space after the type parameters in `type S[K comparable] struct` was dropped (`]struct`).
- Formatter: the parameter type `func(int)` in `func f(int, func(int))` was formatted as `func (int)`.
- Unclosed `<option>` elements were nested inside each other. An open `<option>` is now closed by the next `<option>` or `<optgroup>`.
- A `<template>` inside `<colgroup>` closed the `<colgroup>`. It now stays inside, as in HTML.
- Formatter: files whose only syntax errors are missing tokens (an unclosed call, for example) were rewritten around the gap instead of refused, and `gox fmt -check` passed some of them.
- Formatter: an implicitly closed `<>` (without `</>`) shifted the indentation of the rest of the file and often needed two runs. It now gets a `</>` like other implicitly closed elements.
- Formatter: template literals, JS strings continued with a backslash, and multi-line block comments in `<script>` and `<style>` were re-indented on every run, changing the emitted string content.
- Formatter: blank lines inside blocks no longer carry indentation, and one blank line between entries of var, const and type groups, struct fields, interface members and composite literals is kept. Nearly every file formatted by v0.3.5 changes on the first `gox fmt` after upgrading; run `gox fmt` and then `gox gen`.
- Formatter: spacing around `~//` and `~/* */` comments in markup is kept as written. `two~/* c */three` used to get spaces, changing the rendered text.
- Formatter: multi-line comments in markup got spaces added to their other lines. A `~/* */` after another node gained a space on every run, and a re-indented `<!-- -->` changed the emitted comment text.
- Formatter: the other lines of a multi-line Go `/* */` comment were re-indented, and a file-header comment (a license, for example) gained a space on every run. They are now kept as written.
- Formatter: the line break before `</:>` was removed, changing the raw HTML a `<:>` block emits.
- Formatter: in files with CRLF line endings, multi-line raw strings and `<:>` blocks kept their CRLF line breaks, so the formatted file mixed line endings. Their line breaks are now LF like the rest of the file.
- Formatter: some CRLF files made `gox fmt` crash, for example a bare `~` followed by a `//` comment on a later line.
- Formatter: a `<script>` or `<style>` element inside the attribute of another `<script>` or `<style>` with a body (`<script x=(<style>a</style>)>x</script>`) crashed `gox fmt` and the language server. The inner element's CSS or JS is now left as topiary writes it.
- Formatter: the parentheses of `~( )` around a literal were removed even when a comment was inside, which changed the rendered page: `~(/* note */ "Hi")` rendered `"Hi"` with its quotes, and `~("Hi" /* c */)` rendered the comment.
- gen, fmt and LSP: a `<` at the start of a Go expression in a `<script>` or `<style>` attribute (`<script data-x=(<)>`, typed on the way to `<-ch`) crashed gox, and the language server could not start while such a file was saved. It is now a syntax error.
- gen and LSP: a `.gox` without a package clause (empty or comment-only, for example) produced a `.x.go` without one, and gen reported success. It is now a syntax error.
- gen and LSP: a `.x.go` with CRLF line endings (from a Git checkout with `core.autocrlf`, for example) skipped the newer-version check, so `gox gen` overwrote a file written by a newer GoX with older code.
- LSP: gopls's copy of a `.x.go` went stale when its `.gox` was reopened while the `.x.go` buffer was open, or when the `.x.go` was opened alone, so requests failed with "column is beyond end of line" or "line number N out of range".
- LSP: a `.x.go` buffer left open across a `git checkout` kept the old generated code in gopls when the editor reloaded it with didChange (VS Code does), so other files got false errors such as `undefined: X` until it was closed.
- LSP: workspace folder changes dropped open documents. Removing a folder left its open buffers unserved, and adding a parent folder replaced open documents with copies from disk, losing unsaved text. A folder replaced by its subfolder in one event left nothing served, and a nested folder was forgotten when its parent was removed.
- LSP: closing a `.gox` without saving now returns the document, gopls, the open `.x.go` buffer and the `.x.go` on disk to the file on disk.
- LSP: a `.gox` that is not open is re-read when it changed on disk (after `git checkout`, for example). A rename or code action that touched it used to edit the wrong text. Now, when a code action is listed with such an edit, no code actions from gopls are listed until the next request, and a rename, or resolving or running such a code action, is refused with a request to try again.
- LSP: clients that send only `rootUri` or `rootPath` (or `workspaceFolders: null`) got no `.gox` support.

## Build

- `make build_rust` builds with `cargo build --locked`.

## Docs

- llms.md: the section on the removed legacy forms is gone, and type parameters on elems and generic methods are documented.

## Notes

- A `.gox` opened before its folder is added stays untracked until it is reopened. Until then its edits, saves and formatting are refused with "GoX is not tracking this buffer: it was opened outside the workspace folders or could not be loaded. Close and reopen the file.", and renames, code-action edits and completions in it are computed from the file on disk, so they can land on the wrong line.
- In editors that do not reload changed files automatically (Helix, Emacs without auto-revert), an open `.x.go` regenerated by gox after a checkout needs a second reload. The first reload makes gox rewrite the file; until the second, the buffer still shows the old generated code while gopls has the new one.

## Compatibility

- Verified against gopls v0.24.0.
- Grammar: tree-sitter-gox v0.3.1.
