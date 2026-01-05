use std::{fmt, sync::OnceLock};
use streaming_iterator::StreamingIterator;
use tree_sitter::Node;

pub enum IndentStyle {
    Tab,
    Space(u8),
}

impl fmt::Display for IndentStyle {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match *self {
            IndentStyle::Tab => {
                f.write_str("\t")?;
                Ok(())
            }
            IndentStyle::Space(n) => {
                for _ in 0..n {
                    f.write_str(" ")?;
                }
                Ok(())
            }
        }
    }
}

impl IndentStyle {
    pub fn apply_css(&self, opt: &mut biome_css_formatter::context::CssFormatOptions) {
        match self {
            IndentStyle::Tab => {
                opt.set_indent_style(biome_formatter::IndentStyle::Tab);
            }
            IndentStyle::Space(n) => {
                opt.set_indent_style(biome_formatter::IndentStyle::Space);
                opt.set_indent_width(biome_formatter::IndentWidth::from(*n));
            }
        }
    }
    pub fn apply_js(&self, opt: &mut biome_js_formatter::context::JsFormatOptions) {
        match self {
            IndentStyle::Tab => {
                opt.set_indent_style(biome_formatter::IndentStyle::Tab);
            }
            IndentStyle::Space(n) => {
                opt.set_indent_style(biome_formatter::IndentStyle::Space);
                opt.set_indent_width(biome_formatter::IndentWidth::from(*n));
            }
        }
    }
}

static TS_LANG: OnceLock<topiary_tree_sitter_facade::Language> = OnceLock::new();

fn ts_lang() -> &'static topiary_tree_sitter_facade::Language {
    TS_LANG.get_or_init(|| {
        let ts_lang = topiary_tree_sitter_facade::Language::from(tree_sitter_gox::LANGUAGE);
        ts_lang
    })
}

static LANG: OnceLock<topiary_core::Language> = OnceLock::new();
pub fn lang() -> &'static topiary_core::Language {
    LANG.get_or_init(|| {
        let query_src: &str = include_str!(concat!(env!("CARGO_MANIFEST_DIR"), "/queries.scm"));
        let query = topiary_core::TopiaryQuery::new(&ts_lang(), query_src)
            .expect("failed to compile Topiary query");
        topiary_core::Language {
            name: "GoX".to_string(),
            query,
            grammar: ts_lang().clone(),
            indent: Some(indent().to_string()),
        }
    })
}

static INDENT: OnceLock<IndentStyle> = OnceLock::new();

pub fn set_space_indent(width: u8) {
    INDENT.get_or_init(|| IndentStyle::Space(width));
}
pub fn indent() -> &'static IndentStyle {
    INDENT.get_or_init(|| IndentStyle::Tab)
}

pub fn new_parser() -> topiary_tree_sitter_facade::Parser {
    let mut parser = topiary_tree_sitter_facade::Parser::new().expect("failed to create parser");
    parser
        .set_language(ts_lang())
        .expect("failed to set language");
    parser
}

pub struct Query {
    script: topiary_tree_sitter_facade::Query,
    style: topiary_tree_sitter_facade::Query,
    impl_close: topiary_tree_sitter_facade::Query,
    err_close: topiary_tree_sitter_facade::Query,
}

impl Query {
    fn query<'a>(
        query: &topiary_tree_sitter_facade::Query,
        node: &'a topiary_tree_sitter_facade::Node,
        source: &[u8],
    ) -> Vec<Node<'a>> {
        let mut cursor = topiary_tree_sitter_facade::QueryCursor::new();
        let mut matches = query.matches(node, source, &mut cursor);
        let mut nodes = Vec::new();
        while let Some(item) = matches.next() {
            for capture in item.captures.iter() {
                nodes.push(capture.node.clone());
            }
        }
        nodes
    }
    pub fn scripts<'a>(
        &self,
        node: &'a topiary_tree_sitter_facade::Node,
        source: &[u8],
    ) -> Vec<Node<'a>> {
        return Self::query(&self.script, node, source);
    }
    pub fn implicid_close<'a>(
        &self,
        node: &'a topiary_tree_sitter_facade::Node,
        source: &[u8],
    ) -> Vec<Node<'a>> {
        return Self::query(&self.impl_close, node, source);
    }
    pub fn err_close<'a>(
        &self,
        node: &'a topiary_tree_sitter_facade::Node,
        source: &[u8],
    ) -> Vec<Node<'a>> {
        return Self::query(&self.err_close, node, source);
    }
    pub fn styles<'a>(
        &self,
        node: &'a topiary_tree_sitter_facade::Node,
        source: &[u8],
    ) -> Vec<Node<'a>> {
        return Self::query(&self.style, node, source);
    }
}

static QUERY: OnceLock<Query> = OnceLock::new();

const SCRIPT_QUERY: &str = r#"
(gox_script_head) @cap
"#;

const STYLE_QUERY: &str = r#"
(gox_style_head) @cap
"#;

const IMPLICID_CLOSE_QUERY: &str = r#"
(gox_implicit_close_head) @cap
"#;

const ERR_CLOSE_HEAD: &str = r#"
(gox_erroneous_close_head) @cap
"#;
pub fn query() -> &'static Query {
    QUERY.get_or_init(|| {
        let script = topiary_tree_sitter_facade::Query::new(ts_lang(), SCRIPT_QUERY)
            .expect("failed to compile Topiary scripy query");
        let style = topiary_tree_sitter_facade::Query::new(ts_lang(), STYLE_QUERY)
            .expect("failed to compile Topiary style query");
        let impl_close = topiary_tree_sitter_facade::Query::new(ts_lang(), IMPLICID_CLOSE_QUERY)
            .expect("failed to compile Topiary style query");
        let err_close = topiary_tree_sitter_facade::Query::new(ts_lang(), ERR_CLOSE_HEAD)
            .expect("failed to compile Topiary style query");
        Query {
            script,
            style,
            impl_close,
            err_close,
        }
    })
}
