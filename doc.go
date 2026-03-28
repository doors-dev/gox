// Package gox renders HTML-like templates as ordinary Go values.
//
// In practice, templates are usually authored in `.gox` with HTML expressions
// or the `elem` form, and GoX compiles that syntax to Elem values.
//
// Common forms:
//
//	// Regular Go function returning a template expression.
//	func Badge(label string) gox.Elem {
//		return <span class="badge">~(label)</span>
//	}
//
//	// `elem` is shorthand for the same thing.
//	elem Badge(label string) {
//		<span class="badge">~(label)</span>
//	}
//
//	// Generated/lowered form using Cursor directly.
//	func Badge(label string) gox.Elem {
//		return func(cur gox.Cursor) error {
//			if err := cur.Init("span"); err != nil {
//				return err
//			}
//			if err := cur.AttrSet("class", "badge"); err != nil {
//				return err
//			}
//			if err := cur.Submit(); err != nil {
//				return err
//			}
//			if err := cur.Text(label); err != nil {
//				return err
//			}
//			return cur.Close()
//		}
//	}
//
// Elem renders through Cursor under the hood. Cursor, Attrs, Job, and Printer
// are the lower-level APIs for custom rendering, stream transforms, and other
// integrations.
package gox
