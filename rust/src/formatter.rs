use std::fs::OpenOptions;
use std::io::Write;
use std::{fmt, io};

use super::init;
use tree_sitter::Node;

struct Replace {
    code: String,
    prefix: String,
    beg: usize,
    end: usize,
}

impl fmt::Display for Replace {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str("\n")?;
        for line in self.code.lines() {
            f.write_str(self.prefix.as_str())?;
            init::indent().fmt(f)?;
            f.write_str(line)?;
            f.write_str("\n")?;
        }
        f.write_str(self.prefix.as_str())?;
        Ok(())
    }
}

impl Replace {
    fn append_to(
        acc: &mut Vec<Replace>,
        text: &[u8],
        nodes: Vec<Node<'_>>,
        formatter: fn(&str) -> Result<String, Box<dyn std::error::Error>>,
    ) {
        for node in nodes {
            let mut indent_beg = node.start_byte();
            for i in (0..node.start_byte()).rev() {
                if text[i] == b'\n' {
                    break;
                }
                indent_beg = i
            }
            let mut indent_end = indent_beg;
            for i in indent_beg..=node.start_byte() {
                indent_end = i;
                if !text[i].is_ascii_whitespace() {
                    break;
                }
            }
            let prefix = &text[indent_beg..indent_end];
            let prefix = str::from_utf8(prefix).unwrap().to_string();
            if let Some(content) = node.child_by_field_name("content") {
                let open = node.child_by_field_name("open");
                let close = node.child_by_field_name("close");
                if open.is_none() || close.is_none() {
                    continue;
                }
                let beg = open.unwrap().end_byte();
                let end = close.unwrap().start_byte();
                let code = content.utf8_text(text).unwrap();
                if let Ok(code) = formatter(code) {
                    acc.push(Self {
                        prefix: prefix,
                        code: code,
                        beg,
                        end,
                    });
                }
            }
        }
    }
}
/*
fn dump(content: &[u8]) -> io::Result<()> {
    let mut f = OpenOptions::new()
        .create(true)
        .append(true)
        .open("/tmp/topiary.log")?;
    f.write_all(content)?;
    Ok(())
}  */

pub fn format(input: &[u8], output: &mut Vec<u8>) -> Result<(), topiary_core::FormatterError> {
    let mut parser = init::new_parser();
    let tree = parser.parse(input, None)?;
    if tree.is_none() {
        return Ok(());
    }
    let tree = tree.unwrap();
    let root = tree.root_node();
    let cured = cure(input, &root);
    let input = if let Some(cured) = cured.as_ref() {
        std::str::from_utf8(cured).unwrap()
    } else {
        std::str::from_utf8(input).unwrap()
    };
    let mut formatted_gox = Vec::new();
    topiary_core::formatter_str(
        input,
        &mut formatted_gox,
        init::lang(),
        topiary_core::Operation::Format {
            skip_idempotence: true,
            tolerate_parsing_errors: false,
        },
    )?;
    while let Some(b'\n') = formatted_gox.last() {
        formatted_gox.pop();
    }
    let tree = parser.parse(formatted_gox.as_slice(), None)?;
    if tree.is_none() {
        return Ok(());
    }
    let tree = tree.unwrap();
    let mut replacements = Vec::new();
    let root = tree.root_node();
    let scripts = init::query().scripts(&root, formatted_gox.as_slice());
    let styles = init::query().styles(&root, formatted_gox.as_slice());
    Replace::append_to(
        &mut replacements,
        formatted_gox.as_slice(),
        scripts,
        format_js,
    );
    Replace::append_to(
        &mut replacements,
        formatted_gox.as_slice(),
        styles,
        format_css,
    );
    replacements.sort_by_key(|replace| replace.beg);
    let mut last_replace_end = 0;
    for replace in replacements {
        let chunk = &formatted_gox[last_replace_end..replace.beg];
        last_replace_end = replace.end;
        output.extend_from_slice(chunk);
        write!(output, "{}", replace).unwrap();
    }
    let chunk = &formatted_gox[last_replace_end..];
    output.extend_from_slice(chunk);
    Ok(())
}

struct CureNode<'a> {
    node: Node<'a>,
    remove: bool,
}

fn cure(input: &[u8], root: &topiary_tree_sitter_facade::Node) -> Option<Vec<u8>> {
    let implicid_close = init::query().implicid_close(root, input);
    let remove = init::query().remove(root, input);
    let total = implicid_close.len() + remove.len();
    if total == 0 {
        return None;
    }
    let mut to_cure = Vec::with_capacity(implicid_close.len() + remove.len());
    for node in implicid_close.into_iter() {
        to_cure.push(CureNode {
            node,
            remove: false,
        });
    }
    for node in remove.into_iter() {
        to_cure.push(CureNode { node, remove: true });
    }
    let mut buf = Vec::new();
    let mut insert_end = 0;
    for node in to_cure.into_iter() {
        let remove = node.remove;
        let node = node.node;
        let chunk = &input[insert_end..node.start_byte()];
        if !remove {
            buf.extend_from_slice(chunk);
            insert_end = node.end_byte();
            continue;
        }
        let name: Option<String> = (|| {
            let head = node.parent()?;
            let open = head.child_by_field_name("open")?;
            let name_node = open.child_by_field_name("name")?;
            name_node.utf8_text(input).ok().map(|s| s.to_owned())
        })();
        if name.is_none() {
            continue;
        }
        buf.extend_from_slice(chunk);
        buf.extend_from_slice("</".as_bytes());
        buf.extend_from_slice(name.unwrap().as_bytes());
        buf.extend_from_slice(">".as_bytes());
        insert_end = node.end_byte();
    }
    buf.extend_from_slice(&input[insert_end..]);
    Some(buf)
}

fn format_js(code: &str) -> Result<String, Box<dyn std::error::Error>> {
    let source_type = biome_js_syntax::JsFileSource::js_script();
    let parsed = biome_js_parser::parse(
        code,
        source_type,
        biome_js_parser::JsParserOptions::default(),
    );
    let root = parsed.syntax();
    let mut options = biome_js_formatter::context::JsFormatOptions::new(source_type);
    options.set_semicolons(biome_js_formatter::context::Semicolons::AsNeeded);
    init::indent().apply_js(&mut options);
    let formatted = biome_js_formatter::format_node(options, &root)?;
    let printed = formatted.print()?;
    Ok(printed.into_code())
}

fn format_css(code: &str) -> Result<String, Box<dyn std::error::Error>> {
    let source_type = biome_css_syntax::CssFileSource::css();
    let parsed = biome_css_parser::parse_css(code, biome_css_parser::CssParserOptions::default());
    let root = parsed.syntax();
    let mut options = biome_css_formatter::context::CssFormatOptions::new(source_type);
    init::indent().apply_css(&mut options);
    let formatted = biome_css_formatter::format_node(options, &root)?;
    let printed = formatted.print()?;
    Ok(printed.into_code())
}
