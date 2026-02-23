; GO ---------------------------------------------------------------------------------
; concrete rules

[
 (comment)
 (raw_string_literal)
 (interpreted_string_literal)
 (rune_literal)
 ] @leaf

[
 "const"
 "var"
 "package"
 "import"
 "defer"
 "return"
 "for"
 "if"
 "range"
 "switch"
 "case"
 "select"
 "go"
] @append_space


(type_declaration
  "type" @append_space
)

[
 "="
 ":="
 "+="
 "-="
 "*="
 "/="
 "%="
 "&="
 "|="
 "^="
 ">>="
 "<<="
 "else"
] @append_space @prepend_space

(binary_expression
  [
   "*"
   "/"
   "%"
   "<<"
   ">>"
   "&"
   "&^"
   "+"
   "-"
   "|"
   "^"
   "=="
   "!="
   "<"
   "<="
   ">"
   ">="
   "&&"
   "||"
   ] @append_space @prepend_space
)


; list: top-level
(source_file
  (_)  @allow_blank_line_before
)


(source_file
  (_) @append_hardline
  .
  (comment)* @do_nothing
)

(source_file
  (_) 
  .
  ";" @delete
)


; list: imports
(import_spec_list
  .
  "(" @append_spaced_softline @append_indent_start
  (_)+ @allow_blank_line_before
  ")" @prepend_spaced_softline @prepend_indent_end
  .
)

(import_spec_list
  (_) @append_hardline
  .
  (comment)* @do_nothing
)

(import_spec_list
 ";" @delete
)

; list: statements
(statement_list
  (_) @allow_blank_line_before
)

(statement_list
  (_) @append_hardline
  .
  (comment)* @do_nothing
)

(statement_list
  ";" @delete
)

; list: vars
(var_spec_list
  .
  "(" @append_indent_start @append_hardline
  (_)+
  ")" @prepend_indent_end
  .
)

(var_spec_list
  (_) @append_hardline
  .
  (comment)* @do_nothing
)

(var_spec_list
 ";" @delete
)

; list: const
(const_declaration
  "(" @append_indent_start @append_hardline
  (_)+
  ")" @prepend_indent_end
  )

(const_declaration
  (_) @append_hardline
  .
  (comment)* @do_nothing
  )

(const_declaration
  ";" @delete
)

; list: type

(type_declaration
  "(" @append_indent_start @append_hardline
  (_)+
  ")" @prepend_indent_end
)

(type_declaration
  (_) @append_hardline
  .
  (comment)* @do_nothing
)

(type_declaration
 ";" @delete
)

; list: literal_value
(literal_value 
  .
  "{" @append_indent_start @append_empty_softline
  (_)+
  "}" @prepend_indent_end @prepend_empty_softline
  .
)

(literal_value
  "," @append_spaced_softline
  .
  [(comment) "}" ]* @do_nothing
)

;; list: interface

(interface_type
  "{" @append_indent_start @append_hardline @prepend_space
  (_)+
  "}" @prepend_indent_end 
)

(interface_type
  (_) @append_hardline
  .
  (comment)* @do_nothing
)

(interface_type
 ";" @delete
)

;; list: fields

(field_declaration_list
  .
  "{" @append_indent_start @append_hardline @prepend_space
  (_)+
  "}" @prepend_indent_end
  .
)

(field_declaration_list
 (_) @append_hardline
 .
 (comment)* @do_nothing
 )

(field_declaration_list
 ";" @delete
 )

; blocks
(block
  .
  "{" @prepend_space
)

(block 
  .
  "{" @append_indent_start @append_hardline 
  (_)+
  "}" @prepend_indent_end
  .
)

; comments

(comment) @allow_blank_line_before
(comment) @prepend_input_softline @append_input_softline

((comment) @append_hardline
  (#match? @append_hardline "^//"))

((comment) @multi_line_indent_all
  (#match? @multi_line_indent_all "^/\\*"))



; enumerations

(parameter_declaration
  "," @append_space
)

(parameter_list
  "(" @append_indent_start @append_input_softline @append_antispace
  (_)+
  ")" @prepend_indent_end @prepend_input_softline @prepend_antispace
  )

(parameter_list
  "," @append_input_softline
  .
  ")"* @do_nothing
)

(argument_list
  "(" @append_indent_start @append_input_softline @append_antispace
  (_)+
  ")" @prepend_indent_end @prepend_input_softline @prepend_antispace
  )

(argument_list
  "," @append_input_softline
  .
  ")"* @do_nothing
)
(expression_list
  "," @append_space
)

; funcs
(
 method_declaration
 receiver: (_) @append_space
 )

(method_declaration
  "func" @append_space
  )


(function_declaration
  "func" @append_space
  )

(_
  result: (_) @prepend_space
  )


; "switches"

(expression_switch_statement
  "{" @prepend_space @append_hardline @append_indent_start
  "}" @prepend_hardline @prepend_indent_end
)
(type_switch_statement
  "{" @prepend_space @append_hardline @append_indent_start
  "}" @prepend_hardline @prepend_indent_end
)
(select_statement
  "{" @prepend_space @append_hardline @append_indent_start
  "}" @prepend_hardline @prepend_indent_end
)
(expression_switch_statement
  "{" 
  (comment) @prepend_indent_end @append_indent_start
  "}" 
)
(type_switch_statement
  "{" 
  (comment) @prepend_indent_end @append_indent_start
  "}" 
)
(select_statement
  "{" 
  (comment) @prepend_indent_end @append_indent_start
  "}" 
)
(expression_switch_statement
 initializer: (_)
 ";" @append_space @prepend_antispace
)
(type_switch_statement
 initializer: (_)
 ";" @append_space @prepend_antispace
)
(expression_case
  "case" @append_indent_start  
  ":" @append_hardline
)
(type_case
  "case" @append_indent_start  
  ":" @append_hardline
)
(communication_case
  "case" @append_indent_start  
  ":" @append_hardline
)
(default_case
  "default" @append_indent_start 
  ":" @append_hardline
)

[ 
  (expression_case)
  (default_case)
  (type_case)
  (communication_case)
  ] @prepend_indent_end

; control flow

(for_clause
 ";" @append_space @prepend_antispace
)

(if_statement
 ";" @append_space @prepend_antispace
)

; misc

(_
  name: (_) @append_space
  .
  type: (_)
)

(_
  tag: (_) @prepend_space
)

(type_elem
  "|" @append_space @prepend_space
)

(keyed_element
  ":" @append_space 
)

; channels
(send_statement
  "<-" @append_space @prepend_space
)

(
 "<-"
 .
 "chan" @append_space
)

(
 "chan"
 .
 "<-" @append_space
)
(channel_type
 "chan"
 .
 value: (_) @prepend_space
)


; labels

(_
_
.
(label_name) @prepend_space
)

(
 (label_name)
 .
 ":" @append_space
)


; GOX --------------------------------------------------------------------------------

[
 (gox_comment)
 (gox_tilde_comment)
 ] @leaf @multi_line_indent_all



(gox_tilde_marker) @append_antispace

(gox_open_head_beg) @append_indent_start
(gox_self_closing_head_end) @prepend_indent_end
(gox_close_head_beg) @prepend_indent_end
(gox_implicit_close_head) @prepend_indent_end

(gox_head
  .
 open: (_) @append_empty_softline
 close: (_) @prepend_empty_softline
 .
)

(gox_container_head
  .
 open: (_) @append_empty_softline
 close: (_) @prepend_empty_softline
 .
)

(_
  (gox_tilde_comment)  
  .
  [
   (gox_head) 
   (gox_container_head)
   (gox_void_head)
   (gox_plain_text)
   (gox_space_filler)
   (gox_self_closing_head)
   (gox_doctype)
   (gox_raw_head)
   (gox_tilde)
   (gox_tilde_proxy)
   (gox_tilde_block)
   (gox_comment)
   (gox_script_head)
   (gox_style_head)
  ] @prepend_input_softline  @allow_blank_line_before
)

(_
  [
   (gox_head) 
   (gox_container_head)
   (gox_void_head)
   (gox_plain_text)
   (gox_space_filler)
   (gox_self_closing_head)
   (gox_doctype)
   (gox_raw_head)
   (gox_tilde)
   (gox_tilde_proxy)
   (gox_tilde_block)
   (gox_comment)
   (gox_script_head)
   (gox_style_head)
  ] 
  .
  (gox_tilde_comment) @prepend_input_softline  @allow_blank_line_before
)

(_
  [
   (gox_head) 
   (gox_container_head)
   (gox_void_head)
   (gox_plain_text)
   (gox_space_filler)
   (gox_self_closing_head)
   (gox_doctype)
   (gox_raw_head)
   (gox_tilde)
   (gox_tilde_block)
   (gox_comment)
  ] 
  .
  [
   (gox_head) 
   (gox_container_head)
   (gox_void_head)
   (gox_plain_text)
   (gox_space_filler)
   (gox_self_closing_head)
   (gox_doctype)
   (gox_raw_head)
   (gox_tilde)
   (gox_tilde_block)
   (gox_comment)
  ] @prepend_input_softline @prepend_antispace @allow_blank_line_before
)

(_
  [
   (gox_head) 
   (gox_container_head)
   (gox_void_head)
   (gox_plain_text)
   (gox_space_filler)
   (gox_self_closing_head)
   (gox_doctype)
   (gox_raw_head)
   (gox_tilde)
   (gox_tilde_block)
   (gox_comment)
   (gox_script_head)
   (gox_style_head)
  ] 
  .
  [
   (gox_script_head)
   (gox_style_head)
  ] @prepend_hardline @allow_blank_line_before
)

(_
  [
   (gox_script_head)
   (gox_style_head)
  ] @append_hardline 
  .
  [
   (gox_head) 
   (gox_container_head)
   (gox_void_head)
   (gox_plain_text)
   (gox_space_filler)
   (gox_self_closing_head)
   (gox_doctype)
   (gox_raw_head)
   (gox_tilde)
   (gox_tilde_proxy)
   (gox_tilde_block)
   (gox_comment)
   (gox_script_head)
   (gox_style_head)
  ] @allow_blank_line_before
)

(gox_tilde_proxy) @append_space


; attrs

(_
  (gox_head_name) @append_begin_scope
  [
   (gox_self_closing_head_end)
   (gox_head_end)
   ] @prepend_end_scope
  (#scope_id! "attrs")
)

(_
  attrs: (_)+ @prepend_space
  (#single_line_scope_only! "attrs")
)

(_
  attrs: (_)+ @prepend_hardline
  (#multi_line_scope_only! "attrs")
)

(gox_attr_assign) @prepend_antispace @append_antispace

; elem

(gox_elem_method_declaration
 receiver: (_) @append_space
 )

(gox_elem_method_declaration
  "elem" @append_space
  )

(gox_elem_function_declaration
  "elem" @append_space
  )

; blocks

(gox_block
  .
  "{" @prepend_space
)

(gox_block 
  .
  "{" @append_indent_start @append_hardline 
  (_)+
  "}" @prepend_indent_end @prepend_hardline
  .
)


(gox_tilde_block
  .
  "{" @append_indent_start @append_hardline 
  (_)+
  "}" @prepend_indent_end
  .
)


(_
  (gox_lparen) @append_indent_start @append_input_softline @append_antispace
  (gox_rparen) @prepend_indent_end @prepend_input_softline @prepend_antispace
) 

(gox_multi_arg
  "," @append_input_softline
  .
  ")"* @do_nothing
)
