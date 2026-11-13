use std::collections::HashMap;

use std::io::Write;

use super::init;
use tree_sitter::Node;

pub fn format(input: &[u8], output: &mut Vec<u8>) -> Result<(), topiary_core::FormatterError> {
    let terminated = input.last().is_some_and(|b| *b != b'\n').then(|| [input, b"\n"].concat());
    let input = terminated.as_deref().unwrap_or(input);
    let mut parser = init::new_parser();
    let tree = parser.parse(input, None)?;
    if tree.is_none() {
        return Ok(());
    }
    let tree = tree.unwrap();
    let root = tree.root_node();
    if root.has_error() {
        return Err(topiary_core::FormatterError::Internal("input has syntax errors".to_string(), None));
    }
    let cured = cure(input, &root);
    let input = std::str::from_utf8(cured.as_deref().unwrap_or(input)).map_err(|e| {
        topiary_core::FormatterError::Internal("input is not valid UTF-8".to_string(), Some(Box::new(e)))
    })?;
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
    let root = tree.root_node();
    let post_nodes = init::queries().post(&root, formatted_gox.as_slice());
    let mut proc = PostProcessor::new(formatted_gox, output);
    proc.add_externals(post_nodes.scripts, format_js, true);
    proc.add_externals(post_nodes.styles, format_css, true);
    proc.add_externals(post_nodes.keep, format_keep, false);
    proc.add_externals(post_nodes.align, format_align, false);
    proc.add_externals(post_nodes.antiline, format_antiline, false);
    proc.write_out();
    let append_line = if let Some(b) = output.last()
        && *b == b'\n'
    {
        false
    } else {
        true
    };
    if append_line {
        output.push(b'\n');
    }
    Ok(())
}

struct CureNode<'a> {
    node: Node<'a>,
    remove: bool,
}

fn cure(input: &[u8], root: &topiary_tree_sitter_facade::Node) -> Option<Vec<u8>> {
    let cure_nodes = init::queries().cure(root, input);
    let total = cure_nodes.implicit_close.len() + cure_nodes.remove.len();
    if total == 0 {
        return None;
    }
    let mut to_cure = Vec::with_capacity(total);
    for node in cure_nodes.implicit_close.into_iter() {
        to_cure.push(CureNode {
            node,
            remove: false,
        });
    }
    for node in cure_nodes.remove.into_iter() {
        to_cure.push(CureNode { node, remove: true });
    }
    to_cure.sort_by_key(|cure| cure.node.start_byte());
    let mut buf = Vec::new();
    let mut end = 0;
    for cure in to_cure.into_iter() {
        let node = cure.node;
        let chunk = &input[end..node.start_byte()];
        if cure.remove {
            buf.extend_from_slice(chunk);
            end = node.end_byte();
            continue;
        }
        let name: Option<String> = (|| {
            let head = node.parent()?;
            if head.kind() == "gox_container_head" {
                return Some(String::new());
            }
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
        end = node.end_byte();
    }
    buf.extend_from_slice(&input[end..]);
    Some(buf)
}

type Formatter = fn(
    output: &mut Vec<u8>,
    code: &str,
    indent: Indent,
    intent_str: &str,
) -> Result<(), Box<dyn std::error::Error>>;

struct External {
    code: String,
    indent_pos: tree_sitter::Point,
    start_pos: tree_sitter::Point,
    end_pos: tree_sitter::Point,
    formatter: Formatter,
}

impl External {
    fn write_out(&self, output: &mut Vec<u8>, indent: Indent, indent_str: &str) {
        if (self.formatter)(output, &self.code, indent, indent_str).is_err() {
            write!(output, "{}", &self.code).unwrap();
        }
    }
    fn create(text: &[u8], node: Node<'_>, formatter: Formatter, head: bool) -> Option<Self> {
        if !head {
            let code = node_text(text, node)?;
            if code.is_empty() {
                return None;
            }
            return Some(External {
                code: code.to_string(),
                formatter: formatter,
                indent_pos: node.start_position(),
                start_pos: node.start_position(),
                end_pos: node.end_position(),
            });
        }
        let content = node.child_by_field_name("content")?;
        let open = node.child_by_field_name("open")?;
        let close = node.child_by_field_name("close")?;
        let code = node_text(text, content)?;
        if code.is_empty() {
            return None;
        }
        Some(External {
            code: code.to_string(),
            formatter: formatter,
            indent_pos: open.start_position(),
            start_pos: open.end_position(),
            end_pos: close.start_position(),
        })
    }
}

fn node_text<'t>(text: &'t [u8], node: Node<'_>) -> Option<&'t str> {
    let bytes = text.get(node.start_byte()..node.end_byte())?;
    std::str::from_utf8(bytes).ok()
}

enum ProcessorState {
    Look,
    Enter(usize, Indent),
    Exit(usize, Indent),
}

#[derive(Clone)]
struct Indent {
    actual: usize,
    output: usize,
}

impl Indent {
    fn advance(&self) -> Indent {
        Indent {
            actual: self.actual,
            output: self.output + 1,
        }
    }
    fn indent(&self, indent: &str) -> String {
        indent.repeat(self.output)
    }
}

struct PostProcessor<'a> {
    text: Option<Vec<u8>>,
    externals: HashMap<usize, Vec<External>>,
    output: &'a mut Vec<u8>,
    indents: Vec<Indent>,
    state: ProcessorState,
    indent: String,
}

impl<'a> PostProcessor<'a> {
    fn new(text: Vec<u8>, output: &'a mut Vec<u8>) -> Self {
        Self {
            text: Some(text),
            externals: HashMap::new(),
            output,
            indents: vec![Indent {
                actual: 0,
                output: 0,
            }],
            state: ProcessorState::Look,
            indent: init::INDENT.to_string(),
        }
    }
    fn add_externals(&mut self, nodes: Vec<Node<'_>>, formatter: Formatter, head: bool) {
        for node in nodes {
            if let Some(ext) = External::create(self.text.as_ref().unwrap(), node, formatter, head)
            {
                if let Some(extenals) = self.externals.get_mut(&ext.indent_pos.row) {
                    extenals.push(ext);
                    extenals.sort_by_key(|x| std::cmp::Reverse(x.indent_pos.column));
                } else {
                    self.externals.insert(ext.indent_pos.row, vec![ext]);
                }
            }
        }
    }
    fn write_out(mut self) {
        let text = String::from_utf8(self.text.take().unwrap()).unwrap();
        for (i, line) in text.lines().enumerate() {
            self.on_line(i, line);
        }
    }
    fn on_line(&mut self, index: usize, line: &str) {
        match &self.state {
            ProcessorState::Look => {
                self.look(index, line);
            }
            ProcessorState::Enter(i, indent) => {
                self.enter(*i, indent.clone(), index, line);
            }
            ProcessorState::Exit(i, indent) => {
                self.exit(*i, indent.clone(), index, line);
            }
        }
    }

    fn look(&mut self, index: usize, line: &str) {
        if let Some((indent, rest)) = self.process_indent(line) {
            write!(&mut self.output, "{}", &indent.indent(&self.indent)).unwrap();
            self.examine(index, line, rest, indent);
            return;
        }
        self.write_empty_line();
    }

    fn examine(&mut self, index: usize, line: &str, rest: &str, indent: Indent) {
        if let Some(v) = self.externals.get(&index) {
            let ext = v.last().unwrap();
            if ext.start_pos.row == index {
                let end = ext.start_pos.column + rest.len() - line.len();
                write!(&mut self.output, "{}", &rest[..end]).unwrap();
                self.state = ProcessorState::Exit(index, indent);
                self.on_line(index, line);
                return;
            }
            self.state = ProcessorState::Enter(index, indent);
        }
        write!(&mut self.output, "{}\n", rest).unwrap();
    }

    fn enter(&mut self, ext_index: usize, ext_indent: Indent, line_index: usize, line: &str) {
        if let Some((indent, mut rest)) = self.process_indent(line) {
            let v = self.externals.get(&ext_index).unwrap();
            let ext = v.last().unwrap();
            if line_index != ext.start_pos.row {
                write!(
                    &mut self.output,
                    "{}{}\n",
                    indent.indent(&self.indent),
                    rest
                )
                .unwrap();
                return;
            }
            self.state = ProcessorState::Exit(ext_index, ext_indent);
            let end = ext.start_pos.column + rest.len() - line.len();
            rest = &rest[..end];
            write!(&mut self.output, "{}{}", indent.indent(&self.indent), rest).unwrap();
            self.on_line(line_index, line);
            return;
        }
        self.write_empty_line();
    }

    fn exit(&mut self, ext_index: usize, indent: Indent, line_index: usize, line: &str) {
        let v = self.externals.get_mut(&ext_index).unwrap();
        let ext = v.pop().unwrap();
        if line_index != ext.end_pos.row {
            v.push(ext);
            return;
        }
        if v.len() == 0 {
            self.externals.remove(&ext_index);
        }
        ext.write_out(&mut self.output, indent.clone(), &self.indent);
        let rest = &line[ext.end_pos.column..];
        self.state = ProcessorState::Look;
        self.examine(line_index, line, rest, indent);
    }

    fn write_empty_line(&mut self) {
        write!(&mut self.output, "\n").unwrap();
    }

    fn process_indent<'b>(&mut self, line: &'b str) -> Option<(Indent, &'b str)> {
        let mut last = self.indents.last().unwrap();
        let (indent_count, rest) = calc_indent(&self.indent, line)?;
        if indent_count > last.actual {
            let new_ind = Indent {
                actual: indent_count,
                output: last.output + 1,
            };
            self.indents.push(new_ind.clone());
            return Some((new_ind, rest));
        }
        loop {
            if indent_count >= last.actual {
                return Some((last.clone(), rest));
            }
            self.indents.pop();
            last = self.indents.last().unwrap();
        }
    }
}

fn calc_indent<'b>(indent: &str, line: &'b str) -> Option<(usize, &'b str)> {
    if line.len() == 0 {
        return None;
    }
    let mut indent_count = 0usize;
    let mut rest = line;
    while let Some(r) = rest.strip_prefix(indent) {
        indent_count += 1;
        rest = r;
    }
    if rest.len() == 0 {
        return None;
    }
    Some((indent_count, rest))
}

/*
fn dump(content: &[u8]) -> io::Result<()> {
    let mut f = OpenOptions::new()
        .create(true)
        .append(true)
        .open("/tmp/topiary.log")?;
    f.write_all(content)?;
    Ok(())
} */

fn write_inline_body(
    out: &mut Vec<u8>,
    code: &str,
    literals: &[std::ops::Range<usize>],
    indent: &Indent,
    indent_str: &str,
) {
    write!(out, "\n").unwrap();
    let advanced = indent.advance();
    for line in code.lines() {
        if in_literal(code, line, literals) {
            write!(out, "{}\n", line).unwrap();
            continue;
        }
        write!(out, "{}{}\n", advanced.indent(indent_str), line).unwrap();
    }
    write!(out, "{}", indent.indent(indent_str)).unwrap();
}

fn write_inline_errored_body(
    out: &mut Vec<u8>,
    code: &str,
    literals: &[std::ops::Range<usize>],
    indent: &Indent,
    indent_str: &str,
) {
    let min_indent = code
        .lines()
        .skip(1)
        .filter(|l| !l.trim().is_empty() && !in_literal(code, l, literals))
        .map(|l| l.len() - l.trim_start().len())
        .min()
        .unwrap_or(0);
    write!(out, "\n").unwrap();
    let advanced = indent.advance();
    let mut first = true;
    for line in code.lines() {
        if in_literal(code, line, literals) {
            write!(out, "{}\n", line).unwrap();
            continue;
        }
        let trimmed = if !first && line.len() >= min_indent {
            line.get(min_indent..).unwrap_or(line)
        } else {
            first = false;
            line
        };
        write!(out, "{}{}\n", advanced.indent(indent_str), trimmed).unwrap();
    }
    write!(out, "{}", indent.indent(indent_str)).unwrap();
}

fn in_literal(code: &str, line: &str, literals: &[std::ops::Range<usize>]) -> bool {
    let offset = line.as_ptr() as usize - code.as_ptr() as usize;
    literals.iter().any(|r| r.start < offset && offset <= r.end)
}

fn literal_ranges<L: biome_rowan::Language>(
    root: &biome_rowan::SyntaxNode<L>,
    kinds: &[L::Kind],
    aligned: fn(&biome_rowan::SyntaxTriviaPieceComments<L>) -> bool,
) -> Vec<std::ops::Range<usize>> {
    root.descendants_tokens(biome_rowan::Direction::Next)
        .flat_map(|token| {
            let comments = token
                .leading_trivia()
                .pieces()
                .chain(token.trailing_trivia().pieces())
                .filter_map(|piece| piece.as_comments())
                .filter(move |comment| comment.has_newline() && !aligned(comment))
                .map(|comment| comment.text_range());
            kinds
                .contains(&token.kind())
                .then(|| token.text_trimmed_range())
                .into_iter()
                .chain(comments)
        })
        .map(|range| range.start().into()..range.end().into())
        .collect()
}

fn format_js(
    out: &mut Vec<u8>,
    code: &str,
    indent: Indent,
    indent_str: &str,
) -> Result<(), Box<dyn std::error::Error>> {
    let mut source_type = biome_js_syntax::JsFileSource::js_script();
    let mut parsed = biome_js_parser::parse(
        code,
        source_type,
        biome_js_parser::JsParserOptions::default(),
    );
    if parsed.has_errors() {
        source_type = biome_js_syntax::JsFileSource::js_module();
        parsed = biome_js_parser::parse(
            code,
            source_type,
            biome_js_parser::JsParserOptions::default(),
        );
    }
    if parsed.has_errors() {
        let literals = literal_ranges(
            &parsed.syntax(),
            &[
                biome_js_syntax::JsSyntaxKind::TEMPLATE_CHUNK,
                biome_js_syntax::JsSyntaxKind::JS_STRING_LITERAL,
            ],
            biome_formatter::comments::is_alignable_comment,
        );
        write_inline_errored_body(out, code, &literals, &indent, indent_str);
        return Ok(());
    }
    let root = parsed.syntax();
    let mut options = biome_js_formatter::context::JsFormatOptions::new(source_type);
    options.set_semicolons(biome_js_formatter::context::Semicolons::AsNeeded);
    options.set_indent_style(biome_formatter::IndentStyle::Tab);
    let formatted = biome_js_formatter::format_node(options, &root)?;
    let code = formatted.print()?.into_code();
    let reparsed = biome_js_parser::parse(
        &code,
        source_type,
        biome_js_parser::JsParserOptions::default(),
    );
    let literals = literal_ranges(
        &reparsed.syntax(),
        &[
            biome_js_syntax::JsSyntaxKind::TEMPLATE_CHUNK,
            biome_js_syntax::JsSyntaxKind::JS_STRING_LITERAL,
        ],
        biome_formatter::comments::is_alignable_comment,
    );
    write_inline_body(out, &code, &literals, &indent, indent_str);
    Ok(())
}

fn format_css(
    out: &mut Vec<u8>,
    code: &str,
    indent: Indent,
    indent_str: &str,
) -> Result<(), Box<dyn std::error::Error>> {
    let source_type = biome_css_syntax::CssFileSource::css();
    let parsed = biome_css_parser::parse_css(code, biome_css_parser::CssParserOptions::default());
    let root = parsed.syntax();
    if parsed.has_errors() {
        let literals = literal_ranges(
            &root,
            &[biome_css_syntax::CssSyntaxKind::CSS_STRING_LITERAL],
            biome_formatter::comments::is_doc_comment,
        );
        write_inline_errored_body(out, code, &literals, &indent, indent_str);
        return Ok(());
    }
    let mut options = biome_css_formatter::context::CssFormatOptions::new(source_type);
    options.set_indent_style(biome_formatter::IndentStyle::Tab);
    let formatted = biome_css_formatter::format_node(options, &root)?;
    let code = formatted.print()?.into_code();
    let reparsed =
        biome_css_parser::parse_css(&code, biome_css_parser::CssParserOptions::default());
    let literals = literal_ranges(
        &reparsed.syntax(),
        &[biome_css_syntax::CssSyntaxKind::CSS_STRING_LITERAL],
        biome_formatter::comments::is_doc_comment,
    );
    write_inline_body(out, &code, &literals, &indent, indent_str);
    Ok(())
}

fn format_keep(
    out: &mut Vec<u8>,
    code: &str,
    _indent: Indent,
    _indent_str: &str,
) -> Result<(), Box<dyn std::error::Error>> {
    for line in code.split_inclusive('\n') {
        match line.strip_suffix("\r\n") {
            Some(rest) if !rest.ends_with('\r') => write!(out, "{}\n", rest).unwrap(),
            _ => write!(out, "{}", line).unwrap(),
        }
    }
    Ok(())
}

fn format_align(
    out: &mut Vec<u8>,
    code: &str,
    indent: Indent,
    indent_str: &str,
) -> Result<(), Box<dyn std::error::Error>> {
    if code.is_empty() {
        return Ok(());
    }
    for (i, line) in code.lines().enumerate() {
        if i == 0 {
            write!(out, "{}\n", line).unwrap();
            continue;
        }
        if let Some((_, rest)) = calc_indent(indent_str, line) {
            write!(out, "{}{}\n", indent.indent(indent_str), rest.trim_start()).unwrap();
        } else {
            write!(out, "{}\n", indent.indent(indent_str)).unwrap();
        }
    }
    if !code.ends_with('\n') {
        out.pop();
    }
    Ok(())
}

fn format_antiline(
    out: &mut Vec<u8>,
    code: &str,
    _: Indent,
    _: &str,
) -> Result<(), Box<dyn std::error::Error>> {
    if code.is_empty() {
        return Ok(());
    }
    for (_, line) in code.lines().enumerate() {
        write!(out, "{}", line.trim()).unwrap();
    }
    Ok(())
}
