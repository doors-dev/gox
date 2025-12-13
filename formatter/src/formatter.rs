use std::fmt;
use std::io::Write;

use super::init;
use tree_sitter::Node;
/*
#[cfg(test)]
#[test]

fn test_simple() {
    let test = r#"
    func main() {
        return <div>
			<style>
				/* format-test.css */
:root{--gap:8px;--primary:#09f}
.container{display:flex;gap:var(--gap);padding:calc(var(--gap)*2)}
.container > .item{flex:1 1 auto;border:1px solid #0003;padding:4px  6px}
.container>.item:hover,.container>.item:focus-visible{outline:2px solid var(--primary);outline-offset:2px}

@media (max-width: 600px){
  .container{flex-direction:column}
  .item{font-size:clamp(14px, 2vw ,18px)}
}

@supports (display: grid){
  .container{display:grid;grid-template-columns:repeat(2,minmax(0,1fr))}
}

@keyframes wiggle{
  0%,100%{transform:translateX(0)}
  50%{transform:translateX(3px)}
}

.button{
  background:linear-gradient(90deg,var(--primary), #f0c);
  color:#fff;
  border:none;
  padding:8px 12px;
  animation:wiggle .6s ease-in-out infinite;
}

			</style>

        </div>
    }
    "#;
    let mut out = Vec::new();
    format(test, &mut out).unwrap();
    println!("{}", std::str::from_utf8(&out).unwrap());
} */

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
                    continue
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

pub fn format(input: &str, output: &mut Vec<u8>) -> Result<(), topiary_core::FormatterError> {
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
    let mut parser = init::new_parser();
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
    let parsed = biome_css_parser::parse_css(
        code,
        biome_css_parser::CssParserOptions::default(),
    );
    let root = parsed.syntax();
    let mut options = biome_css_formatter::context::CssFormatOptions::new(source_type);
    init::indent().apply_css(&mut options);
    let formatted = biome_css_formatter::format_node(options, &root)?;
    let printed = formatted.print()?;
    Ok(printed.into_code())
}
