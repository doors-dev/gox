![GoX](https://github.com/doors-dev/gox/raw/main/logo.png)

## GoX — HTML templates as first-class Go expressions

GoX lets you write HTML-like templates in `.gox` files, compile them into `.x.go` files in the same Go package, and keep editor features working across both source and generated code. The generated code is plain Go, so the template layer stays inspectable, debuggable, and extensible from ordinary Go code.

> Syntax guide: [doors.dev/docs/template-syntax](https://doors.dev/docs/template-syntax)  
> This README focuses on installation, workflow, editor integration, and the rendering API behind GoX.

---

## Install

### Install the `gox` tool

The easiest path is the prebuilt binary from [GitHub Releases](https://github.com/doors-dev/gox/releases).

To install from source:

```bash
make install
```

That builds the bundled Rust formatter and installs `gox`. Building from source requires Go, Cargo, and a working native toolchain.

### Editor integration

Recommended: use the official [VS Code](https://github.com/doors-dev/vscode-gox) or [Neovim](https://github.com/doors-dev/nvim-gox) extension.

The bare `gox` command starts the GoX language server. `gox srv` is the explicit form of the same command.

If you are wiring an editor manually:

1. Run `gox` or `gox srv`.
2. Attach it to both `.go` and `.gox` buffers.
3. Disable a separate workspace `gopls` client. GoX launches and proxies a `gopls` instance.
4. Make sure `gopls` is on `PATH`, or pass `-gopls /path/to/gopls`.
5. Install the [tree-sitter-gox](https://github.com/doors-dev/tree-sitter-gox) grammar.

Useful server flags:

- `-gopls` to point at a specific `gopls` binary
- `-listen` to expose the server over TCP or a Unix socket instead of stdio
- `-listen.timeout` to stop an idle socket server after a timeout

GoX sits in front of `gopls`. It parses `.gox`, generates `.x.go`, keeps source and target positions mapped both ways, and forwards normal Go language features through a `gopls` instance.

### Add the Go package

```bash
go get github.com/doors-dev/gox
```

Keep the installed `gox` tool and the Go module version reasonably in sync. Generated `.x.go` files carry a GoX version marker, and newer generated files require a tooling upgrade.

---

## Workflow

### File layout

A typical package can contain all three kinds of files:

```text
main.go     # regular Go
page.gox    # source template
page.x.go   # generated Go
```

What you edit:

- edit `.gox`
- treat `.x.go` as generated output
- keep normal `.go` files alongside both

What the tooling does:

- the language server regenerates `.x.go` on save
- `gox gen` does the same in batch mode
- orphaned `.x.go` files are removed automatically

Rules worth following:

- do not edit `.x.go` manually
- do not use the `.x.go` suffix for hand-written files
- if a generated file was produced by a newer GoX version, upgrade the tooling before continuing

### Commands you actually use

```text
gox                 # start the language server
gox gen             # generate .x.go files for the current directory
gox gen ./pkg       # generate a specific file or directory
gox fmt             # format .gox and .go files in the current directory
gox fmt ./internal  # format a specific directory
gox fmt ./main.go   # format a specific file
gox ver             # print the GoX version
```

By default, `gox gen` and `gox fmt` use the current directory and respect `.gitignore`. Both commands also accept an optional positional file or directory path. `-no-ignore` disables ignore handling, and `-force` skips target-file safety checks during generation.

The old `-path` flag is no longer supported.

### What happens under the hood

The `.gox` parser produces a syntax tree. The assembler walks that tree and lowers template nodes into plain Go built around `gox.Elem(func(cur gox.Cursor) error { ... })`.

Alongside generation, GoX keeps a source-to-target translation map between `.gox` and `.x.go`. That mapping is used not only for diagnostics, but also for editor navigation and edits that need to round-trip through generated Go.

`gox fmt` formats `.gox` with the bundled Rust formatter and regular `.go` files with `gofmt`. Embedded `<script>` and `<style>` blocks are reformatted too.

---

## Rendering Model

Generated `.gox` code ultimately works with four things:

- render values such as `Elem`, `Comp`, and templ-compatible components
- a `Cursor` that builds structure and emits rendering operations
- an `Attrs` set attached to an open head before it is submitted
- a stream of `Job` values consumed by a `Printer`

That is the useful mental model for GoX. Once you understand those pieces, generated `.x.go` files are easy to read and the advanced hooks stop feeling magical.

### Render values

These are the core renderable types:

```go
type Comp interface {
    Main() Elem
}

type Elem func(cur Cursor) error
```

`Elem` is the main render value in GoX. It is a function that renders through a `Cursor`.

It also:

- implements `Comp` by returning itself from `Main()`
- renders directly with `Render(ctx, w)`
- renders through custom pipelines with `Print(ctx, printer)`

GoX also defines a minimal `Templ` interface for values that render with `Render(ctx, w)`, and `Cursor.Any` knows how to emit those too.

So in normal Go code, a template is just a value you can return, store, pass around, and render.

### `Cursor`

`Cursor` is the low-level rendering state machine. It streams operations to a `Printer` and tracks active element heads so it can enforce correct ordering.

There are three head lifecycles:

1. Regular element
   `Init(tag)` -> optional `AttrSet` / `AttrMod` -> `Submit()` -> child content -> `Close()`
2. Void element
   `InitVoid(tag)` -> optional `AttrSet` / `AttrMod` -> `Submit()`
3. Container
   `InitContainer()` -> child content -> `Close()`

Regular and void heads become HTML tags. Containers do not emit a tag, but they still create open and close jobs in the stream, which makes them useful for grouping and for render-time transformations.

The important state rule is:

- before `Submit()`, you are still building a head and may mutate attributes
- after `Submit()`, you may emit child content, but you may no longer mutate that head

`cur.Context()` returns the default context for jobs emitted through that cursor. `cur.Send()` forwards a prebuilt job directly to the underlying printer and bypasses cursor state validation.

Generated `.x.go` files are mostly straightforward cursor code, simular to:

```go
func Badge(label string) gox.Elem {
    return func(cur gox.Cursor) error {
        err := cur.Init("span"); err != nil {
            return err
        }
        if err := cur.AttrSet("class", "badge"); err != nil {
            return err
        }
        if err := cur.Submit(); err != nil {
            return err
        }
        if err := cur.Text(label); err != nil {
            return err
        }
        return cur.Close()
    }
}
```

Most placeholder rendering ends up calling `Cursor.Any` or `Cursor.Many`.

`Cursor.Any` understands:

- `string` and `[]string`
- `Elem` and `[]Elem`
- `Comp` and `[]Comp`
- `Job` and `[]Job`
- `Editor`
- `Templ`
- `[]interface{}`

Anything else falls back to escaped `fmt.Fprint`.

### Attributes

`Attrs` is the mutable attribute set attached to a head while that head is being built.

You mainly encounter attributes in three places:

- when generated code or hand-written cursor code calls `AttrSet`
- when code calls `AttrMod` to attach one or more render-time modifiers
- when custom printers or proxies inspect `JobHeadOpen.Attrs`

Important details from the API:

- attributes are stored sorted by name
- names are case-sensitive
- `nil` means "unset"
- `false` means "unset"
- any other non-nil value means "set"

The attribute system is also an extension point:

```go
type Modify interface {
    Modify(ctx context.Context, tag string, attrs Attrs) error
}

type Mutate interface {
    Mutate(name string, value any) any
}

type Output interface {
    Output(w io.Writer) error
}
```

`Modify` is head-level, not value-level. It runs right before a head is rendered, receives the full `Attrs` set, and can inspect or change the final attributes for that element. This is the hook used by render-time attribute transformations.

`Mutate` is value-level. It lets a new attribute value depend on the previous value already stored under the same name.

`Output` lets a value control its own escaped output.

That makes attributes more than plain HTML metadata. They are also part of the rendering pipeline.

### Extension points

GoX exposes three main render-time extension points:

- `Editor` for code that needs direct cursor access
- `Proxy` for wrapping or rebasing an element subtree before it renders
- `Printer` for consuming and transforming the emitted job stream

#### `Editor`

`Editor` is the escape hatch for render-time behavior that needs direct cursor access.

Use it when rendering needs to:

- emit low-level jobs manually
- work directly with `cur.Context()`
- integrate with a larger rendering runtime
- do something more specific than "return another subtree"

#### `Proxy`

A `Proxy` wraps an `Elem` subtree before it renders.

You can do any type of transofrmation with proxy, for example:

- change attributes
- convert tags
- render the subtree through a custom printer before forwarding it
- basically any transormation

A common implementation pattern is a proxy printer: call `elem.Print(cur.Context(), customPrinter)`, inspect the first `*gox.JobHeadOpen` or `*gox.JobComp`, adjust it, then forward the rest into the current cursor.

#### `Printer`

Rendering is a job stream.

Useful concrete job types include:

- `*gox.JobHeadOpen`
- `*gox.JobHeadClose`
- `*gox.JobText`
- `*gox.JobRaw`
- `*gox.JobBytes`
- `*gox.JobComp`
- `*gox.JobTempl`
- `*gox.JobFprint`
- `*gox.JobError`

Important behavior from the API:

- open and close jobs for the same head share an `ID`
- container head jobs emit no HTML, but still exist in the stream
- the default printer from `gox.NewPrinter` checks `j.Context().Err()` before calling `Output`
- jobs are pooled and single-use

Custom printers are where GoX opens up the most. They can buffer, transform, route, inspect, or reinterpret the stream instead of just writing HTML sequentially.

### Helper adapters

The helpers in `helpers.go` keep the API lightweight when you want one-off implementations:

- `gox.EditorComp` for values that should be both `Editor` and `Comp`
- `gox.EditorCompFunc`
- `gox.EditorFunc`
- `gox.ProxyFunc`
- `gox.ModifyFunc`
- `gox.PrinterFunc`
- `gox.NewEscapedWriter`

`gox.NewEscapedWriter` is useful when custom rendering code needs the same escaping rules as GoX text and attribute output.


---

## Compatibility

`gox.Elem` implements both `gox.Comp` and a templ-style `Render(ctx, w)` method, so it can sit in existing render pipelines while still exposing lower-level hooks when you need them.

---

> *Disclaimer: GoX is an independent, third-party project and is not affiliated with, endorsed by, or sponsored by The Go Project, Google, or any official Go tooling.*
