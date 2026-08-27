# v0.3.1

This release removes redundant, competing mechanics and streamlines printer extension.

v0.2.x had two ways to write low-level cursor code (`Editor` vs `Elem`) and two kinds of job stream a printer could face (flat jobs vs opaque `JobComp` wrappers that bypassed custom printers). Now there is one of each: `Elem` is the single low-level primitive, and every printer sees the full, flat job stream — no special cases, no hand-expansion. Cursor state errors follow suit: one `ErrState` sentinel instead of a partial error type plus stray plain errors. Components render inline as a plain call, dropping per-component allocations.

Breaking for code that used the removed API (migration table below). Rendered output is unchanged — for well-formed components the default printer produces byte-identical HTML to v0.2.x.

## Breaking: components expand inline

`Cursor.Comp` no longer emits a deferred `JobComp` — it calls `Main()` immediately and runs the returned `Elem` against the calling cursor. A component's jobs go to the same `Printer` and share the head stack with its parent. `JobComp`/`NewJobComp` are removed, and `Elem.Print` now streams the expanded jobs.

For custom printers this is a simplification: the job stream is always flat, every job of every component passes through your `Send`. Previously nested component jobs went through a fresh default printer, bypassing custom printers unless each `*gox.JobComp` was expanded by hand. If your printer special-cased `JobComp`, delete that path.

Also faster: no per-component job allocation, pooled job cycle, or fresh printer/cursor per component render.

## Breaking: Editor merged into Elem

With inline expansion, `Editor` became redundant: an `Elem` already receives the rendering cursor directly, so `Elem` *is* the low-level escape hatch. The `Editor` interface and its adapters (`EditorComp`, `EditorFunc`, `EditorCompFunc`, `Cursor.Editor`) are removed. `Cursor.Any` (and therefore `~(...)`) now accepts a plain `func(cur gox.Cursor) error` and renders it as an `Elem`.

## Breaking: removed context variants and deprecated aliases

`Cursor.CompCtx`, `Cursor.TemplCtx`, and the deprecated `Cursor.Send`, `Cursor.AttrSet`, `Cursor.AttrMod` are removed. `JobHeadOpen`/`JobHeadClose` are renamed to `JobOpen`/`JobClose` (constructors likewise).

## Migration

| Before | After |
|---|---|
| `gox.EditorFunc(f)` | pass `f` directly — `Cursor.Any` accepts `func(cur gox.Cursor) error`; or wrap as `gox.Elem(f)` |
| type implementing `Editor` | implement `Comp`: return the cursor logic as an `Elem` from `Main()` |
| `EditorComp` / `EditorCompFunc` | `Comp` / `gox.Elem` |
| `cur.Send(job)` | `cur.Printer().Send(job)` |
| `cur.AttrSet` / `cur.AttrMod` | `cur.Set` / `cur.Modify` |
| `cur.CompCtx(ctx, c)` / `cur.TemplCtx(ctx, t)` | `cur.Comp(c)` / `cur.Templ(t)`; for a different context, run against `gox.NewCursor(ctx, cur.Printer())` |
| `*gox.JobHeadOpen` / `*gox.JobHeadClose` | `*gox.JobOpen` / `*gox.JobClose` |

## Breaking: `HeadError` replaced by `ErrState`

The `HeadError` string type is gone. Every cursor state error now wraps the `ErrState` sentinel:

```go
if errors.Is(err, gox.ErrState) { /* cursor misuse */ }
```

This check catches *all* state errors — under v0.2.x, `Submit`/`Close` misuse returned plain errors that a `HeadError` type check missed. Error messages are now lowercase per Go convention.

## Fixed

- Formatter: `<script>` blocks with top-level `await` or `import`/`export` are formatted again — bodies parse as classic script first, then retry as ES module. Genuinely invalid JS is left verbatim instead of coming out mangled.
