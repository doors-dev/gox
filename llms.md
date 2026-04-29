# GoX for LLM agents

GoX compiles `.gox` templates to `.go`. You write HTML-like templates as typed Go expressions; the `gox` CLI generates `.x.go` next to each `.gox`.

## Prerequisites

Before doing any GoX work, verify the tooling is in place. Run this check first:

```sh
gox ver        # must print a version, e.g. v0.1.27
```

If `gox` is not on `PATH`, the agent cannot generate `.x.go` and the build will fail with errors like `undefined: example` for every template. Install it before continuing:

- **Prebuilt binary (recommended for agents, CI, and most users):** download the matching archive from [GitHub Releases](https://github.com/doors-dev/gox/releases) and put the `gox` binary on `PATH`.
- **From source:** clone the repo and run `make install`. This builds the bundled Rust formatter alongside the Go binary, so a working native toolchain plus Go and Cargo are required. There is **no `go install ...@latest` shortcut** — `gox` includes native dependencies that the simple Go install path cannot build.

Stop and ask the user before installing if the environment forbids network access or global installs.

The Go module side is handled automatically — `go get github.com/doors-dev/gox` (or just adding an `import "github.com/doors-dev/gox"` and running `go mod tidy`) pulls the library used by generated `.x.go`. **Keep the CLI version and the module version in sync**: generated files carry a version marker and refuse to load with mismatched runtimes. `gox ver` and the `// Managed by GoX vX.Y.Z` header at the top of any `.x.go` should agree.

**Install the editor extension — always recommended.** `.gox` is not Go, so without an extension the editor highlights every template as broken syntax, breaks go-to-definition, and disables completion across `.gox` ↔ `.go` boundaries. The extensions also start the GoX language server (which proxies `gopls`), so the experience matches working in plain Go.

- VS Code: [`doors-dev/vscode-gox`](https://marketplace.visualstudio.com/items?itemName=doors-dev.gox)
- Neovim: [`doors-dev/nvim-gox`](https://github.com/doors-dev/nvim-gox)

This applies whether you are coding by hand or driving an agent: most agents (Claude Code, Cursor, Copilot, etc.) run inside VS Code, and the extension lets both you and the agent see real diagnostics on `.gox`.

## Default stack rule

- **If the project already depends on [`github.com/doors-dev/doors`](https://github.com/doors-dev/doors) — do NOT install or use `goxx`.** This document does not describe Doors-specific APIs or behavior.
- **If the project uses GoX without Doors — install [`github.com/doors-dev/goxx`](https://github.com/doors-dev/goxx) by default.** `gox` core is intentionally minimal; `goxx` provides the HTTP render helper, parallel rendering, and class/proxy helpers that almost every standalone GoX app ends up needing.

Check before adding `goxx`: `grep doors-dev/doors go.mod`. If Doors is present, skip `goxx` entirely.

## Golden rule

**Always edit `.gox` files. Never edit or hand-write `.x.go` files, and never write templates in the "generated" cursor style directly.** The `.x.go` output is a build artifact — it is overwritten on every `gox gen` (and by the language server on save). If you need to change template output, edit the `.gox` source and regenerate.

Do not:
- hand-write `gox.Elem(func(cur gox.Cursor) error { ... })` as an authoring style — that is what `.gox` compiles *to*, not what humans/LLMs write. The cursor API is for runtime extension points (`Editor`, `Proxy`, `Printer`, custom `Modify`), not for general templating.
- create new files with the `.x.go` suffix — that suffix is reserved for generated files and GoX may delete orphans.
- treat `.x.go` as the source of truth for template semantics — read the `.gox` source first. Generated `.x.go` can still be useful as a debugging/reference aid when you need to understand the lowered cursor calls, source-map positions, or low-level APIs such as `gox.Editor`.

## Workflow

```
gox fmt           # format .gox and .go (and embedded <script>/<style>)
gox gen           # regenerate .x.go for current directory
gox gen ./pkg     # regenerate a specific path
go run .          # build/run as normal Go
```

After editing a `.gox` file, run `gox gen` before `go build` / `go run` / `go test`. If you see "undefined: MyElem" it usually means you forgot to regenerate.

**Always run `gox gen` after `gox fmt`.** Formatting can shift lines and tokens in `.gox`, which invalidates the existing `.x.go` source-map. Regenerate immediately so the `.x.go` stays in sync and so language-server features (go-to-def, errors, hover) keep pointing at the right spots.

A typical package has all three kinds of files side by side:
```
main.go      # regular Go
page.gox     # template source (edit this)
page.x.go    # generated Go (do not edit)
```

## Syntax essentials

A `.gox` file is a Go file plus the `elem` keyword and HTML literals. Everything else (imports, types, methods, regular functions) is standard Go.

Use `.gox` when the file needs GoX template syntax: `elem`, HTML literals, fragments, placeholders, raw blocks, or template control flow. It is also normal for a `.gox` file to contain nearby regular Go declarations that support those templates: types, constructors, methods, constants, helper functions, `Modify` implementations, etc.

Do not use `.gox` for every file. If a file contains only ordinary Go and no GoX/HTML syntax, make it a regular `.go` file.

### HTML literals are Go expressions

Inside `.gox`, `<tag>...</tag>` is a value of type `gox.Elem`. It can appear anywhere any other Go expression can:

```gox
var greeting gox.Elem = <h1>Hi</h1> // var initializer

type Card struct {
    Body gox.Elem
}

card := Card{Body: <p>hello</p>} // composite literal

func make() gox.Elem {
    return <b>x</b>
}
```

Because `gox.Elem` implements `Main() gox.Elem`, it also satisfies `gox.Comp` and plugs into any slot that expects a component.

### `elem` keyword

Shorthand for a function/method that returns `gox.Elem` with an HTML body:

```gox
elem Greeting(name string) {
    <h1>Hello, ~(name)!</h1>
}
```

has the same generated API shape as writing a regular function in `.gox`:

```gox
func Greeting(name string) gox.Elem {
    return <h1>Hello, ~(name)!</h1>
}
```

That equivalence describes the generated API shape. For authoring templates, prefer `elem` syntax for top-level functions and component methods.

Important render-time boundary: an `elem` body is evaluated when the element renders. In an ordinary Go function that returns an HTML literal, the returned literal's content still renders later, but any ordinary Go code before `return <...>` runs immediately when the function is called:

```gox
elem Page() {
    ~{
        // render-time setup
    }
    <main>...</main>
}

func Page() gox.Elem {
    // call-time setup, before rendering
    return <main>...</main>
}
```

The idiomatic pattern is: use `elem`, put render-time setup at the top in `~{ ... }`, then render the markup. Use ordinary `.go`/`.gox` functions returning `gox.Elem` only when you intentionally want call-time setup.

Generated `elem` functions are ordinary Go functions. Go visibility works the same as in `.go` files: `elem Greeting(...)` is exported from the package, while `elem greeting(...)` is package-private.

Works in every function shape:

```gox
// top-level function
elem Header() {
    <h1>X</h1>
}

// method (typically to implement gox.Comp.Main)
elem (u User) Main() {
    <li>~(u.Name)</li>
}
```

Anonymous `elem() { ... }` exists, but do not use an empty-argument anonymous elem as a generic wrapper. Prefer a named `elem` helper for reusable markup, pass expressions as arguments/slots when possible, and use anonymous `elem` only when a true inline template function is the clearest fit.

**Pitfall:** `elem` is a reserved keyword in `.gox`. You cannot use it as a variable, parameter, or field name (e.g. `func (w Wrap) Proxy(cur gox.Cursor, elem gox.Elem) error` is a parse error — rename to `el`).

### Go statements inside an `elem` body: use `~{ ... }`

An `elem` body is in template mode. HTML literals, text, placeholders, template control flow, comments, and raw blocks can appear directly. Plain Go statements such as `x := 1`, `type T struct{}`, `sort.Slice(...)`, or `if err != nil { ... }` **cannot** be written bare inside an `elem` body.

Use a Go snippet block `~{ ... }` for local Go statements:

```gox
elem UserList() {
    ~{
        type User struct {
            Name string
        }
        users := []User{{Name: "Ada"}, {Name: "Ben"}}
    }
    <ul>
        ~(for _, u := range users {
            <li>~(u.Name)</li>
        })
    </ul>
}
```

Top-level declarations (`import`, `type`, `func`, methods) are still normal Go and live outside `elem` bodies. Inside an `elem`, reach for `~{ ... }` whenever you need statements; reach for `~(expr)` when you need to render an expression.

Inside a `~{ ... }` snippet you are writing code in the generated render function, whose return value is `error`.

A top-of-`elem` `~{ ... }` block, before any HTML has been emitted, is a common render-time setup pattern. Put validation, data loading, derived values, and whole-component guards there. From that position it is okay to `return nil` to make this entire `elem` render nothing, or `return err` for a real failure that should abort rendering and be handled by the caller/HTTP error path:

```gox
elem MaybePanel(show bool) {
    ~{
        if !show {
            return nil
        }
    }
    <section>Visible</section>
}
```

Do not use `return nil` from a snippet after output has started, especially inside a tag. It returns from the whole render function before enclosing tags are closed, so it can leave broken HTML:

```gox
elem BrokenPanel() {
    <div>
        ~{
            return nil // bad: exits before </div> is emitted
        }
    </div>
}
```

Inside already-open markup, use `~(if ...)` or an inline `~func { return nil }` to skip only a child. Returning a non-nil error from an inner snippet is allowed only for a real critical failure; it still aborts rendering rather than closing the surrounding tags.

```gox
elem OptionalChild(show bool) {
    <div>
        ~(if show {
            <span>Visible</span>
        })
        ~func {
            if !show {
                return nil
            }
            return <strong>Ready</strong>
        }
    </div>
}
```

If a real error occurs inside already-open markup, returning it is fine when the whole render should fail and the outer renderer will handle that failure:

```gox
elem CriticalChild() {
    <div>
        ~{
            if err := check(); err != nil {
                return err
            }
        }
        <span>OK</span>
    </div>
}
```

HTML tags also create Go scopes in the generated code. Variables declared inside a tag are scoped to that tag body. Declare values in a top-level `~{ ... }` before the tag if later siblings need them:

```gox
elem SharedValue() {
    ~{
        label := "GoX"
    }
    <h1>~(label)</h1>
    <p>~(label)</p>
}
```

### Placeholders: `~(expr)`

Insert any Go expression into HTML:

```gox
<p>~(user.Name)</p>
<p>~(a, " ", b, " ", c)</p>   // multi-arg: rendered left to right
```

Parentheses can be **omitted only for literals** — string, numeric, and composite literals:

```gox
<p>~"hello" ~42 ~User{Name: "Z"}</p>
```

Pitfall: `~name` (bare identifier) is a **parse error**. Always write `~(name)`. Only literal forms survive without parens.

Pitfall: adjacent placeholders do not insert whitespace. `~"a" ~"b"` renders as `a b` because of the literal space between them; `~"a"~"b"` renders `ab`. If in doubt, put spaces inside a single `~(...)` call: `~("a ", b)`.

### Text whitespace

GoX normalizes template indentation and blank lines, but preserves spaces that are part of text content. A leading or trailing space next to real text is intentional and appears in output:

```gox
<span> Text</span>      // <span> Text</span>
<span>Text </span>      // <span>Text </span>
<span>Text ~(v)</span>  // text node is "Text ", then v
```

For multi-line text, indentation used to line up the template is removed. If the line has an extra space before or after the actual text, that extra space is preserved:

```gox
<span>
     Text
</span>
// renders: <span> Text</span>

<span>
    Text
</span>
// renders: <span>Text</span>
```

Adjacent text-only lines are joined with a single space (`One` then `Two` renders `One Two`). Blank lines and whitespace-only lines render nothing. Text next to tags does not get an automatic separator: write an explicit leading/trailing space in the text when you need one.

`gox fmt` removes indentation and blank/edge whitespace that has no output effect; spaces that would be emitted are preserved.

### Control flow: `~(if ...)`, `~(for ...)`

Wrap the statement in `~(...)`:

```gox
~(if loggedIn {
    Welcome, ~(name)!
} else if guest {
    Please log in.
} else {
    Bye.
})

~(for _, u := range users {
    <li>~(u.Name)</li>
})

~(for i := 0; i < 3; i++ {
    <span>~(i)</span>
})
```

### Fragments / containers: `<>...</>`

Groups children without emitting a wrapper tag. Use for children slots and lists:

```gox
elem Layout(body gox.Elem) {
    <body>~(body)</body>
}

Layout(<>
    <h1>Title</h1>
    <p>Paragraph</p>
</>)
```

### Attributes

- String/numeric literals on the right: `<div class="card" tabindex="0">`.
- Go expression in parens: `<div id=(id) title=(user.Bio)>`.
- Function literal attribute, evaluated at render: `<input checked=func { return u.Agreed }>`.
- Bare attribute (no value): `<input type="text" required>` — equivalent to `required=(true)`.
- `nil` or `false` **omits the attribute** entirely (a stray space may remain between neighbours — cosmetic only).
- `true` renders as a bare name: `checked=(true)` → `checked`.
- Attribute names are **case sensitive**: `class` and `Class` are different attributes (both would be emitted).
- Emitted attributes are sorted alphabetically in the output — do not rely on source order when diffing HTML.

Attribute values are not text placeholders. Do not put `~` after `=` in attributes. Use `id=(id)`, `href=(item.Href)`, `class=(tone)`, or `checked=func { return ok }`; not `id=~(id)`, `href=~(item.Href)`, `class=~(tone)`, or `checked=~func { ... }`.

### Attribute Modifiers

Attribute modifiers are **most commonly provided by third-party libraries** (`goxx.Class`, component kits, etc.). Attach them inside parentheses **inside the opening tag**:

```gox
<button (goxx.Class("primary"))>Go</button>
<button (goxx.Class("a"), TestID("save"))>Multi</button>   // multiple, comma-separated
```

**Writing your own `Modify` is a reasonable thing to do** when you want to package a reusable set of attributes under one name (design-system presets, analytics tags, form-field conventions, etc.). It is a single method:

```go
type Modify interface {
    Modify(ctx context.Context, tag string, attrs gox.Attrs) error
}
```

Example — a reusable "primary CTA" attribute bundle:

```go
type PrimaryCTA struct {
    Label string
}

func (p PrimaryCTA) Modify(_ context.Context, _ string, attrs gox.Attrs) error {
    attrs.Get("class").Set("btn btn-primary")
    attrs.Get("role").Set("button")
    attrs.Get("aria-label").Set(p.Label)
    return nil
}

// Usage:
// <button (PrimaryCTA{Label: "Save changes"})>Save</button>
```

Inside `Modify`, mutate attributes via `attrs.Get(name).Set(value)` — not `attrs.Set(...)`, which does not exist on the type. For quick inline helpers without declaring a type, use `gox.ModifyFunc(func(ctx, tag, attrs) error { ... })`.

### Void / self-closing elements

Standard HTML void tags (`<br>`, `<hr>`, `<img>`, `<input>`, `<meta>`, `<link>`, …) may be written `<br>`, `<br/>`, or `<br />` — all three render as `<br>` (no closing tag, as HTML requires). Writing `</br>` is an error.

### Reading third-party docs: naming → syntax

When a library/API describes a value as:

- **`AttrMod` / "attribute mod" / "attribute modifier" / "modifier"** → use it with **modifier syntax**: `<tag (x)>`.
- **`Proxy`** → use it with **proxy syntax**: `~>(x) nextItem`.
- Described as **both** (e.g. `goxx.Class`) → **default to modifier syntax**. Reach for proxy syntax only when you cannot reach the target tag directly — typically when wrapping a component whose outer tag you don't author:

    ```gox
    ~>(goxx.Class("test").Remove("test2")) ~(test2())
    ~// the first real tag inside test2() will get class "test" and lose "test2"
    ```

Picking the wrong syntax usually produces a compile error or a no-op, not a silent bug — but knowing the convention up-front saves the round trip.

### Per-attribute value hooks: `Mutate` and `Output`


These run on individual attribute *values*, not on the whole attribute set. Useful when a value needs to compose with a previous one, or wants to control its own serialization.

```go
// Combine with the previous value stored under the same name.
type Mutate interface {
    Mutate(name string, prev any) (newValue any)
}

// Provide serialized attribute bytes; GoX still escapes/writes them.
type Output interface {
    Output(w io.Writer) error
}
```

`Mutate` is how `class`-style accumulators are built: each time the same attribute name is set, the new value can inspect the previous one and merge. `Output` is how a value renders into an attribute slot without going through the default `fmt.Fprint`, but escaping is still applied.

### Text escaping

Text and placeholder values are HTML-escaped by default:

```gox
<p>~("<script>")</p>   // → &lt;script&gt;
```

To emit literal HTML, use the raw block `<:>...</:>`:

```gox
<svg viewBox="0 0 24 24">
    <:>
        <path d="..." />
    </:>
</svg>
```

Whitespace inside `<:>` blocks is preserved verbatim.

Raw blocks are useful for large static SVG/HTML fragments that should be emitted as literal markup.

### Components (`gox.Comp`)

Anything with `Main() gox.Elem` is a component. In `.gox` files, implement that method with `elem` method syntax, not by hand-writing `func (c Component) Main() gox.Elem`:

```gox
type Card struct {
    Title string
}

elem (c Card) Main() {
    <article>~(c.Title)</article>
}
```

`gox.Elem` already implements `Main() gox.Elem`, so it is passable anywhere `gox.Comp` is expected.

Render a component the same way you render any other Go expression: put it in a normal placeholder. There is no separate component-call syntax; `~(...)` is just the ordinary expression placeholder. Only attribute modifiers (`<tag (x)>`) and proxies (`~>(x) ...`) have special attachment syntax.

```gox
~(myComponent)                 // needs parens — identifier
~User{Name: "Z"}               // composite literal, parens optional
```

There is no JSX-style `<MyComp/>`. Only HTML tag names go between `<...>`.

### Choosing `elem` helper vs component struct

Use a plain `elem Helper(args...)` for small, stateless fragments when the parameters are few and the helper does not need its own methods:

```gox
elem Badge(label string) {
    <span class="badge">~(label)</span>
}
```

Use a struct component with `Main()` when the UI is a real reusable unit with named props, shared data, local/derived state, setup logic, or helper render methods.

Good reasons to choose a component struct:

- The call site would otherwise pass many positional arguments.
- The values are clearer as named fields (`Title`, `Body`, `Items`, `Active`, callbacks, state handles).
- Several render helpers need the same data; make them receiver methods.
- The component is rendered repeatedly as composite literals.
- A constructor needs to initialize defaults, local state, derived values, callbacks, or other setup before rendering.
- The component should satisfy APIs that accept `gox.Comp`.

Keep the requested HTML shape intact when extracting helpers or components. If the output needs a wrapper around several children, render that wrapper in the parent and call the helpers/components inside it. Move a wrapper into a component only when the component itself owns that wrapper.

```gox
<section>
    ~Card{
        Title: "Build",
        Body:  <p>Use GoX</p>,
    }
    ~Card{
        Title: "Review",
        Body:  <p>Check output</p>,
    }
</section>
```

Pattern — data-shaped component with child content:

```gox
type Card struct {
    Title string
    Body gox.Elem
}

elem (c Card) Main() {
    <article>
        <h2>~(c.Title)</h2>
        ~(c.Body)
    </article>
}

~Card{
    Title: "Profile",
    Body:  <p>Ada</p>,
}
```

Pattern — component with receiver helpers that share fields:

```gox
type MenuItem struct {
    Slug string
    Title string
    Path string
}

type Menu struct {
    Active string
    Items []MenuItem
}

elem (m Menu) Main() {
    <ul>
        ~(for _, item := range m.Items {
            ~(m.item(item))
        })
    </ul>
}

elem (m Menu) item(item MenuItem) {
    <li
        class=func {
            if item.Slug == m.Active {
                return "active"
            }
            return nil
        }>
        <a href=(item.Path)>~(item.Title)</a>
    </li>
}
```


Pattern — constructor returns `gox.Comp` when setup should be hidden from callers:

```go
func NewSearch(users []User) gox.Comp {
    return searchBox{
        Users: users,
        // Initialize defaults or derived values here.
    }
}

type searchBox struct {
    Users []User
}

// elem (s searchBox) Main() { ... }
```

Pattern — compose a page from parts:

```gox
elem MarketingPage() {
    ~(PageShell{
        Header: <header>Product</header>,
        Body: <>
            ~(Hero("Build faster"))
            ~(FeatureList([]string{"Typed", "Composable"}))
        </>,
        Footer: <footer>Done</footer>,
    })
}

type PageShell struct {
    Header gox.Elem
    Body gox.Elem
    Footer gox.Elem
}

elem (p PageShell) Main() {
    <div class="page">
        ~(p.Header)
        <main>~(p.Body)</main>
        ~(p.Footer)
    </div>
}
```

For page composition, keep small sections as plain `elem` helpers when they have only a couple of inputs. Use a shell/layout component when the page has named slots (`Header`, `Body`, `Aside`, `Footer`), shared settings, or repeated structure. The call site should read as an assembly of parts, not as one giant template or a long positional argument list.

Do not use `Main` as a component field name. `Main()` is the render method required by `gox.Comp`; a field named `Main` collides with it, or tempts code like `~(p.Main)` to render the method value instead of the intended slot. Use `Body`, `Content`, `Children`, or a more specific name such as `MainContent`.

If all you need is "wrap these children in a tag", prefer an `elem Layout(body gox.Comp)` helper. If the wrapper has named configuration, state, or multiple helper methods, make it a component.


### Children / slot pattern

Pass an `Elem` as a function argument or struct field and render it in the child position. Fragments (`<>...</>`) are the idiomatic way to build a multi-node child:

```gox
elem Card(title string, body gox.Comp) {
    <article>
        <h2>~(title)</h2>
        ~(body)
    </article>
}

~(Card("Hi", <>
    <p>first</p>
    <p>second</p>
</>))
```

The same idea works via a struct field that holds `gox.Elem` (or `gox.Comp` that accepts also components with `Main() gox.Elem` besides plain `gox.Elem`):

```gox
type Page struct {
    Title string
    Body gox.Elem
}

elem (p Page) Main() {
    <section>
        <h1>~(p.Title)</h1>
        ~(p.Body)
    </section>
}
```

### Comments

```gox
~// single-line template comment (not emitted)
~/* multi-line template comment */
<!-- emitted HTML comment -->
```

### Inline func expression: `~func { ... }`

Evaluated at render time; the return value is inserted where it appears. Use `~func { ... }` when you need render-time work at a specific point and the logic is too complex for an inline `~(if ...)` or simple placeholder.

Inside the function literal you can write normal Go: `switch`, early returns, local variables, error handling, etc. It may return text, a component, or an HTML literal.

```gox
<div>
    ~func {
        user, err := db.Get(id)
        if err != nil {
            return <span>error</span>
        }
        switch user.Role {
        case "admin":
            return <strong>~(user.Name)</strong>
        }
        return Card(user)
    }
</div>
```

For simple reusable markup, prefer a named `elem` helper. For simple conditional markup, prefer `~(if ...)`. Function literals also work inside attributes (shown above).

### Go snippets: `~{ ... }`

Switch to plain Go statements:

```gox
elem page() {
    ~{
        users := loadUsers()
        sort.Slice(users, func(i, j int) bool {
            return users[i].Name < users[j].Name
        })
    }
    <ul>
        ~(for _, u := range users {
            <li>~(u.Name)</li>
        })
    </ul>
}
```

### Proxies: `~>(p) nextItem`

A `Proxy` captures the next renderable item at render time. That item can be an element, component placeholder, inline `~func`, raw block, text node, control-flow block, or ordinary placeholder — not only an HTML tag.

Proxy syntax captures **one item only**. In `~>(p) Text ~(dd)`, the proxy sees only the `Text` item; `~(dd)` is a following sibling. To proxy several values together, group them into one item: use a wrapper element, a fragment/container, or one multi-value placeholder such as `~>(p) ~("Text ", dd)`.

Proxies can be chained. `~>(proxy1) ~>(proxy2) item` is the same as `~>(proxy1, proxy2) item`. The list is written outermost first: `proxy2` handles the original item first, then `proxy1` captures the result.

**In the vast majority of cases, proxy values come from third-party libraries** (`goxx.Parallel`, `goxx.Class` as a proxy, `goxx.ProxyMod(...)`, telemetry, styling kits, etc.) — you import and attach them, you rarely author them.

**Do not write a custom `Proxy` unless the task explicitly asks for a low-level rendering transform and simpler tools cannot express it.** Treat custom proxies as a last resort. If the goal is "wrap content in another tag", write a normal component/slot helper. If the goal is "set attributes on this element", write a `Modify`. If the goal is "attach attributes through a wrapping component", use `goxx.ProxyMod` or `goxx.Class` as a proxy. Custom proxies require careful cursor lifecycle handling and are easy to get wrong.

Custom `Proxy` is reserved for transforming captured output before it is emitted — e.g. rewriting attributes on many descendants, running captured output through a filtering printer, collecting render metrics, or rebasing output into a different target.

Usage (common side):

```gox
~>(goxx.Parallel()) <section>~(SlowStats())</section>
~>(Track) ~func {
    return <span>computed</span>
}
~>(Track) ~("Text ", dd)
~>(Track) Text
~>(proxy1) ~>(proxy2) <div>same as comma list</div>
~>(proxy1, proxy2) <div>same as chained form</div>
```

Proxy type and sample implementation (uncommon side — only needed for real output transforms):

```go
type Proxy interface {
    Proxy(cur gox.Cursor, e gox.Elem) error   // parameter name cannot be `elem` in .gox
}

func (wrap) Proxy(cur gox.Cursor, e gox.Elem) error {
    if err := cur.Init("section"); err != nil {
        return err
    }
    if err := cur.Submit(); err != nil {
        return err
    }
    if err := e(cur); err != nil {
        return err
    }
    return cur.Close()
}
```

For a one-off without declaring a type, use `gox.ProxyFunc(func(cur, e) error { ... })`.

## Rendering at runtime

```go
elem := Greeting("World")
elem.Render(ctx, w)          // writes HTML to any io.Writer
elem.Print(ctx, customPrint) // streams jobs to a custom Printer
```

`gox.Elem` is templ-compatible: it implements `Render(ctx, w) error` so it drops into any `templ`-expecting API.

### `Cursor.Any` value handling

The `~(expr)` placeholder ultimately calls `Cursor.Any` (or `Cursor.Many` for multi-arg). It has dedicated handling for:

- `string`, `[]string` (each item rendered)
- `gox.Elem`, `[]gox.Elem`
- `gox.Comp`, `[]gox.Comp`
- `gox.Job`, `[]gox.Job`
- `gox.Editor`
- `gox.Templ` (anything with `Render(ctx, w) error`, e.g. `templ.Component`)
- `[]any`

Anything else falls back to escaped `fmt.Fprint`. `nil` Elem/Comp render as nothing.

### Raw HTML: `<:>...</:>` and `gox.EditorFunc`

`<:>...</:>` emits content verbatim, whitespace included. For programmatic raw output, drop an `Editor` through a placeholder:

```gox
~(gox.EditorFunc(func(cur gox.Cursor) error {
    return cur.Raw("<mark>unescaped</mark>")
}))
```

Never pipe untrusted input through either — these are escape hatches for trusted content.

## Runtime extension interfaces

Only reach for these when ordinary templating cannot express what you need. They live in regular `.go` files, not `.gox`.

- `gox.Editor` — `Edit(cur gox.Cursor) error`. Direct cursor access for low-level job emission. Use `gox.EditorFunc` for one-offs.
- `gox.Proxy` — `Proxy(cur gox.Cursor, e gox.Elem) error`. Intercepts the next renderable item. Use existing proxies (`goxx.Parallel`, `goxx.Class`, `goxx.ProxyMod`) unless you truly need a low-level output transform.
- `gox.Modify` — `Modify(ctx, tag, attrs Attrs) error`. Head-level attribute transformer attached via `<tag (Modifier)>`. Use `gox.ModifyFunc`.
- `gox.Mutate` — `Mutate(name, prev any) any`. Value-level: combine with an existing attribute value under the same name.
- `gox.Output` — `Output(w io.Writer) error`. Value controls its own attribute serialization; GoX still escapes/writes it through the normal attribute path.
- `gox.Printer` — consumer of the `Job` stream. Custom printers can buffer, transform, or reroute rendering.

Cursor lifecycle rules when writing an `Editor` or low-level code:
1. Regular element: `Init(tag)` → `AttrSet`/`AttrMod` → `Submit()` → children → `Close()`.
2. Void: `InitVoid(tag)` → `AttrSet` → `Submit()` (no `Close`).
3. Container: `InitContainer()` → children → `Close()` (emits no tag).

Before `Submit()` you may mutate attributes; after `Submit()` you may emit children but the head is frozen.

## `goxx` — the standard extension package

Reminder: only use `goxx` when the project is **not** using Doors. If `go.mod` contains `github.com/doors-dev/doors`, stop; this document does not define the Doors path.

```sh
go get github.com/doors-dev/goxx
```

`goxx` is a thin extension on top of `gox` that provides the batteries most standalone GoX apps want.

### HTTP render helper

`goxx.Render` buffers the full response before anything is written to the response writer, so a render failure can still produce a clean error status:

```go
func handlePage(w http.ResponseWriter, r *http.Request) {
    out, err := goxx.Render(r.Context(), Page())
    if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
        return // client/deadline went away, no body
    }
    if err != nil {
        http.Error(w, "render failed", http.StatusInternalServerError)
        return
    }
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    w.WriteHeader(http.StatusOK)
    out.WriteTo(w)
}
```

If you want to serve a custom GoX error page, render that page only after checking for context cancellation/deadline errors. If the error page render also fails, fall back to `http.Error`; `goxx.Render` has not written to the response yet, so it is still safe to choose the fallback status/body:

```go
func handlePage(w http.ResponseWriter, r *http.Request) {
    out, err := goxx.Render(r.Context(), Page())
    if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
        return
    }
    if err != nil {
        errOut, errPageErr := goxx.Render(r.Context(), ErrorPage())
        if errPageErr != nil {
            http.Error(w, "render failed", http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        w.WriteHeader(http.StatusInternalServerError)
        errOut.WriteTo(w)
        return
    }
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    w.WriteHeader(http.StatusOK)
    out.WriteTo(w)
}
```

For non-HTTP code that passes a printer directly to `elem.Print(ctx, printer)`, use `goxx.NewPrinter(w)` and `goxx.WriterError(err)` to distinguish writer failures from render failures.

### Parallel rendering

Mark independent slow fragments with `~>(goxx.Parallel())`. They render on background workers in parallel while output order stays deterministic.

```gox
elem Page() {
    <main>
        <h1>Dashboard</h1>
        ~>(goxx.Parallel()) <section>~(SlowStats())</section>
        ~>(goxx.Parallel()) <aside>~(SlowSidebar())</aside>
    </main>
}
```

Use it for DB queries, outbound HTTP, filesystem reads, heavy computation — anything that can wait independently. Default pool is seven workers plus the caller; tune via `goxx.NewPrinter(w, goxx.WithWorkers(n))` (pass `0` to use unbounded goroutines instead of a bounded pool).

Extend the pipeline with a custom printer via `goxx.WithPrinter(factory)`; add `goxx.WithFlat()` when your printer wants expanded content instead of `*gox.JobComp` values. But you don't need custom printers most of the time.

### `goxx.Class` — three-in-one class helper

`goxx.Class` builds an immutable class set that can be used as an attribute modifier, as an attribute value, or as a proxy — pick whichever fits the call site. Inputs are split with `strings.Fields`, so variadic and space-separated forms are equivalent.

```go
goxx.Class("button", "primary")
goxx.Class("button primary")
goxx.Class("button").Add("primary").Filter("hidden")
```

Three ways to attach it:

```gox
// 1. As an attribute modifier (merges with any class="..." on the same element)
<button (goxx.Class("button primary")) class="wide">Save</button>
// → class="wide button primary"

// 2. As the value of the class attribute directly (not recommended)
<button class=(goxx.Class("button", "primary"))>Save</button>

// 3. As a proxy — propagates through containers and components
//    until it reaches the first real element
~>(goxx.Class("button primary")) <button>Save</button>
```

`Filter` filters matches regardless of whether the class was added before or after:

```go
goxx.Class("button").Filter("hidden").Add("hidden").String() // "button"
```

Useful for wrapping a component that already ships with a class you don't want:

```gox
~>(goxx.Class("primary").Filter("disabled")) ~(BaseButton())
```

### `goxx.ProxyMod` — attach a `Modify` through wrappers

Turns a `gox.Modify` into a proxy that carries the modifier through leading components and containers until the first real element, then applies it once. Good for cross-cutting attributes like test ids that you do not want baked into component APIs:

Because `ProxyMod` and `goxx.Class` attach attributes, the captured item must begin with an element, component, or container that eventually begins with an element. Text before the first element is an error for these proxies.

```go
func TestID(id string) gox.Proxy {
    return goxx.ProxyMod(gox.ModifyFunc(func(_ context.Context, _ string, attrs gox.Attrs) error {
        attrs.Get("data-testid").Set(id)
        return nil
    }))
}
```

```gox
elem Toolbar() {
    ~>(TestID("save-button")) ~(SaveButton())
}
```

(`goxx.Class(...)` used as a proxy is built on this machinery.)

## Common pitfalls checklist

1. **Edited `.x.go` by hand** — changes will be blown away on next `gox gen`. Always edit the `.gox`.
2. **Wrote a new template in "cursor style"** (`gox.Elem(func(cur gox.Cursor) error { ... })`) in a `.go` file — don't; write `.gox` and let generation produce this.
3. **Wrote plain Go statements bare inside an `elem` body** — parse error. Wrap statements in `~{ ... }`, then render values with `~(...)`.
4. **Put render-time setup before `return <...>` in an ordinary function** — code before the return runs when the function is called. Prefer `elem` with a top-level `~{ ... }` snippet for render-time setup.
5. **Forgot to run `gox gen`** after editing `.gox` — build fails with "undefined" for new/renamed elems.
6. **Forgot to run `gox gen` after `gox fmt`** — formatting shifts positions and invalidates the source map; regenerate so `.x.go` matches.
7. **Used `elem` as an identifier name** — it is a reserved keyword in `.gox`. `func (w W) Proxy(cur gox.Cursor, elem gox.Elem)` is a parse error. Rename the parameter.
8. **Used `return nil` inside already-open markup** — this stops the whole render before closing tags are emitted. Put whole-component guards in a top-of-`elem` `~{ ... }` block; use `~(if ...)` or `~func { return nil }` to skip only a child. Return a non-nil error inside markup only for a real critical failure.
9. **Expected variables declared inside a tag to be visible later** — tags create Go scopes in generated code. Declare shared values in a top-level snippet before the tag.
10. **`~name` without parens** — parse error. Use `~(name)`. Paren-less form is literals only (string, number, composite literal).
11. **Mixed `class` and `Class`** — treated as two separate attributes. Pick one casing (lower by default).
12. **Relied on source order of attributes** — output order is alphabetical.
13. **Tried `<MyComponent/>`** — no such thing. Render components via `~(myComponent)` or composite literal placeholder.
14. **Used `Main` as a component field name** — `Main()` is the component render method. Name slots `Body`, `Content`, `Children`, or `MainContent` instead.
15. **Put ordinary-only Go code in `.gox`** — `.gox` can contain regular Go declarations, but use a `.go` file when there is no GoX/HTML syntax in the file.
16. **Expected different visibility rules for `elem`** — generated `elem` functions are normal Go functions. Uppercase names are exported; lowercase names are package-private.
17. **Used placeholder syntax inside an attribute value** — `~(...)` is for text/template positions. In attributes, write `id=(id)`, `href=(item.Href)`, `class=(tone)`, or `checked=func { return ok }`; not `id=~(id)` or `checked=~func { ... }`.
18. **Dropped a required parent wrapper around several children** — if the expected output has `<section>`, `<ul>`, `<main>`, a shell, or another grouping parent around multiple helpers/components, render that parent in the caller and place the helper/component calls inside it.
19. **Called `attrs.Set(...)` inside a `Modify`** — the method is `attrs.Get(name).Set(value)`.
20. **Used `</br>` or `</input>`** — void tags have no closing form. Use `<br>`, `<br/>`, or `<br />`.
21. **Unescaped injection via `~(untrustedHTML)`** — this escapes. Use `<:>...</:>` or `gox.EditorFunc` + `cur.Raw(...)` only for trusted/literal HTML, never for user input.
22. **Whitespace between placeholders** — `~(a) ~(b)` has no space; `~(a," ",b)` has one.
23. **Expected template indentation/newlines to render verbatim** — indentation and blank lines are normalized, but intentional leading/trailing spaces in text are preserved. Use raw blocks for verbatim whitespace-sensitive content.
24. **Expected a proxy to capture several siblings** — `~>(p) Text ~(v)` captures only `Text`. Group siblings into one item, such as `~>(p) ~("Text ", v)`, a fragment, or a wrapper element.
25. **Authored a custom `Proxy` for ordinary wrapping or attributes** — avoid it. Use components/fragments for wrapping, `Modify` for attributes, and `goxx.ProxyMod`/`goxx.Class` to carry modifiers through components.
26. **Imported `gox` but never used it in `.gox`** — allowed, but the generated `.x.go` references `gox.Elem`, so make sure the module is in `go.mod` (`go get github.com/doors-dev/gox`).
27. **Version drift** — generated files carry a GoX version marker. If CI uses a different `gox` binary than you, regenerate with the matching version.
28. **Added `goxx` to a project that already depends on Doors** — if `go.mod` contains `github.com/doors-dev/doors`, skip `goxx`; this document does not define the Doors path.
29. **Wrote a raw `http.Handler` that calls `Elem.Render(ctx, w)` directly** — works, but on render failure you have already written bytes and cannot send an error status. Prefer `goxx.Render(ctx, elem)` which buffers first, then commit with `out.WriteTo(w)`.
