# v0.3.4

## Added

- `Attrs.Iter()` yields the set attributes in name order.

## Fixed

- LSP: inlay hints on `.gox` files were placed at generated-code positions, often past the end of the file, and included hints for generated code (`cur:`, `tag:`, `any:`, `context.Context`). Hints now map back to `.gox` positions, and hints that belong to generated code are dropped.
- LSP: gopls's experimental move refactors (`refactor.move.*`) are no longer offered on `.gox` files, where they would move generated code.

## Compatibility

- Verified against gopls v0.24.0.
