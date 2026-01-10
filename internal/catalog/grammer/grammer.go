package grammer

const (
	// generic
	ERROR                     = "ERROR"
	COMMENT                   = "comment"
	RAW_STRING_LITERAL        = "raw_string_literal"
	INTERPETED_STRING_LITERAL = "interpreted_string_literal"
	PACKAGE                   = "package_clause"

	// GO
	CONST_DECLARATION  = "const_declaration"
	TYPE_DECLARATION   = "type_declaration"
	VAR_DECLARATION    = "var_declaration"
	FUNC_DECLARATION   = "function_declaration"
	METHOD_DECLARATION = "method_declaration"
	TYPE_IDENT         = "type_identifier"
	STRUCT_TYPE        = "struct_type"
	INTERFACE_TYPE     = "interface_type"
	FIELD_IDENT        = "field_identifier"

	//
	GOX_ELEM_FUNC_DEC  = "gox_elem_function_declaration"
	GOX_ELEM_METH_DEC  = "gox_elem_method_declaration"
	GOX_ELEM_FUNC_TYPE = "gox_elem_function_type"
	GOX_ELEM_FUNC_LIT  = "gox_elem_func_literal"

	// content
	GOX_ELEMENT                   = "gox_element"
	GOX_HEAD                      = "gox_head"
	GOX_CONTAINER_HEAD            = "gox_container_head"
	GOX_RAW_HEAD                  = "gox_raw_head"
	GOX_SCRIPT_HEAD               = "gox_script_head"
	GOX_STYLE_HEAD                = "gox_style_head"
	GOX_VOID_HEAD                 = "gox_void_head"
	GOX_SELF_CLOSING_HEAD         = "gox_self_closing_head"
	GOX_ERRONEOUS_CLOSE_HEAD      = "gox_erroneous_close_head"
	GOX_ERRONEOUS_CLOSE_HEAD_NAME = "gox_erroneous_close_head_name"
	GOX_DOCTYPE                   = "gox_doctype"
	GOX_TILDE                     = "gox_tilde"
	GOX_TILDE_PROXY               = "gox_tilde_proxy"
	GOX_TILDE_COMMENT             = "gox_tilde_comment"
	GOX_COMMENT                   = "gox_comment"
	GOX_PLAIN_TEXT                = "gox_plain_text"
	GOX_RAW_TEXT                  = "gox_raw_text"
	GOX_HEAD_NAME                 = "gox_head_name"
	GOX_CLOSE_HEAD                = "gox_close_head"
	GOX_OPEN_HEAD                 = "gox_open_head"

	// gox.Attr
	GOX_ATTR               = "gox_attr"
	GOX_ATTR_NAME          = "gox_attr_name"
	GOX_LITERAL_ATTR       = "gox_literal_attr"
	GOX_CLASS_ATTR         = "gox_class_attr"
	GOX_CLASS_LITERAL_ATTR = "gox_class_literal_attr"
	GOX_BOOL_ATTR          = "gox_bool_attr"
	GOX_ATTR_MOD           = "gox_attr_mod"

	// MISC
	GOX_SINGLE_ARG            = "gox_single_arg"
	GOX_MULTI_ARG             = "gox_multi_arg"
	GOX_FUNC                  = "gox_func"
	GOX_BLOCK                 = "gox_block"
	GOX_IMPLICID_CLOSE        = "gox_implicit_close_head"
	GOX_HEAD_END              = "gox_head_end"
	GOX_SELF_CLOSING_HEAD_END = "gox_self_closing_head_end"
	GOX_SPACE_FILLER          = "gox_space_filler"

	// GOX_TILDE
	GOX_TILDE_IF            = "gox_tilde_if"
	GOX_TILDE_FOR           = "gox_tilde_for"
	GOX_TILDE_JOB           = "gox_tilde_job"
	GOX_TILDE_LITERAL_VALUE = "gox_tilde_literal_value"
	GOX_TIDE_BLOCK          = "gox_tilde_block"
	GOX_TILDE_MARKER        = "gox_tilde_marker"
)
