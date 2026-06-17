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

pub struct CureNodes<'a> {
    pub transform: Vec<Node<'a>>,
    pub remove: Vec<Node<'a>>,
}

const CURE_QUERY: &str = r#"
[(gox_implicit_close_head) (gox_func) (gox_tilde_block)] @transform

[(gox_redundant) (gox_space_filler) (gox_erroneous_close_head)] @remove
"#;


pub struct PostNodes<'a> {
    pub scripts: Vec<Node<'a>>,
    pub styles: Vec<Node<'a>>,
    pub shift: Vec<Node<'a>>,
    pub keep: Vec<Node<'a>>,
    pub align: Vec<Node<'a>>,
    pub antiline: Vec<Node<'a>>,
}

const POST_PROCESS_QUERY: &str = r#"
(gox_script_head) @script

(gox_style_head) @style

[(comment) (gox_comment) (gox_tilde_comment)] @shift

[(raw_string_literal) (gox_raw_head)] @keep

[(gox_plain_text)] @align

(func_literal
  body: (block
    .
    "{"
    .
    "}"
    .
  ) @antiline
)

(gox_func
  body: (block
    .
    "{"
    .
    "}"
    .
  ) @antiline
)
"#;

pub struct Queries {
    cure: topiary_tree_sitter_facade::Query,
    post: topiary_tree_sitter_facade::Query,
}
static QUERIES: OnceLock<Queries> = OnceLock::new();

impl Queries {
    pub fn cure<'a>(
        &self,
        node: &'a topiary_tree_sitter_facade::Node,
        source: &[u8],
    ) -> CureNodes<'a> {
        let capture_names = self.cure.capture_names();
        let mut nodes = CureNodes {
            transform: Vec::new(),
            remove: Vec::new(),
        };
        let mut cursor = topiary_tree_sitter_facade::QueryCursor::new();
        let mut matches = self.cure.matches(node, source, &mut cursor);
        while let Some(item) = matches.next() {
            for capture in item.captures.iter() {
                let name = capture_names[capture.index as usize];
                match name {
                    "transform" => nodes.transform.push(capture.node.clone()),
                    "remove" => nodes.remove.push(capture.node.clone()),
                    _ => {}
                }
            }
        }
        nodes
    }
    pub fn post<'a>(
        &self,
        node: &'a topiary_tree_sitter_facade::Node,
        source: &[u8],
    ) -> PostNodes<'a> {
        let capture_names = self.post.capture_names();
        let mut nodes = PostNodes {
            scripts: Vec::new(),
            styles: Vec::new(),
            shift: Vec::new(),
            keep: Vec::new(),
            align: Vec::new(),
            antiline: Vec::new(),
        };
        let mut cursor = topiary_tree_sitter_facade::QueryCursor::new();
        let mut matches = self.post.matches(node, source, &mut cursor);
        while let Some(item) = matches.next() {
            for capture in item.captures.iter() {
                let name = capture_names[capture.index as usize];
                let node = capture.node.clone();
                match name {
                    "script" => nodes.scripts.push(node),
                    "style" => nodes.styles.push(node),
                    "shift" => {
                        if node.start_position().row != node.end_position().row {
                            nodes.shift.push(node)
                        }
                    }
                    "keep" => {
                        if node.start_position().row != node.end_position().row {
                            nodes.keep.push(node)
                        }
                    }
                    "align" => {
                        if node.start_position().row != node.end_position().row {
                            nodes.align.push(node)
                        }
                    }
                    "antiline" => {
                        if node.start_position().row != node.end_position().row {
                            nodes.antiline.push(node)
                        }
                    }
                    _ => {}
                }
            }
        }
        nodes
    }
}

pub fn queries() -> &'static Queries {
    QUERIES.get_or_init(|| {
        let cure = topiary_tree_sitter_facade::Query::new(ts_lang(), CURE_QUERY).unwrap();
        let post = topiary_tree_sitter_facade::Query::new(ts_lang(), POST_PROCESS_QUERY).unwrap();
        Queries { cure, post }
    })
}
