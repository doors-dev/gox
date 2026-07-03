package tags

import (
	"slices"
	"strings"
)

type Kind int

func (k Kind) String() string {
	if k == ANY {
		return "*"
	}
	return tags[k]
}

func (k Kind) IsVoid() bool {
	return k < end_of_void_tags
}

func (k Kind) Equals(tag string) bool {
	return strings.EqualFold(k.String(), tag)
}

const (
	ANY  Kind = -1
	AREA Kind = iota
	BASE
	BASEFONT
	BGSOUND
	BR
	COL
	COMMAND
	EMBED
	FRAME
	HR
	IMAGE
	IMG
	INPUT
	ISINDEX
	KEYGEN
	LINK
	MENUITEM
	META
	NEXTID
	PARAM
	SOURCE
	TRACK
	WBR
	end_of_void_tags
	A
	ABBR
	ADDRESS
	ARTICLE
	ASIDE
	AUDIO
	B
	BDI
	BDO
	BLOCKQUOTE
	BODY
	BUTTON
	CANVAS
	CAPTION
	CITE
	CODE
	COLGROUP
	DATA
	DATALIST
	DD
	DEL
	DETAILS
	DFN
	DIALOG
	DIV
	DL
	DT
	EM
	FIELDSET
	FIGCAPTION
	FIGURE
	FOOTER
	FORM
	H1
	H2
	H3
	H4
	H5
	H6
	HEAD
	HEADER
	HGROUP
	HTML
	I
	IFRAME
	INS
	KBD
	LABEL
	LEGEND
	LI
	MAIN
	MAP
	MARK
	MATH
	MENU
	METER
	NAV
	NOSCRIPT
	OBJECT
	OL
	OPTGROUP
	OPTION
	OUTPUT
	P
	PICTURE
	PRE
	PROGRESS
	Q
	RB
	RP
	RT
	RTC
	RUBY
	S
	SAMP
	SCRIPT
	SEARCH
	SECTION
	SELECT
	SLOT
	SMALL
	SPAN
	STRONG
	STYLE
	SUB
	SUMMARY
	SUP
	SVG
	TABLE
	TBODY
	TD
	TEMPLATE
	TEXTAREA
	TFOOT
	TH
	THEAD
	TIME
	TITLE
	TR
	U
	UL
	VAR
	VIDEO
	RAW
)

var tags = [...]string{
	AREA:             "area",
	BASE:             "base",
	BASEFONT:         "basefont",
	BGSOUND:          "bgsound",
	BR:               "br",
	COL:              "col",
	COMMAND:          "command",
	EMBED:            "embed",
	FRAME:            "frame",
	HR:               "hr",
	IMAGE:            "image",
	IMG:              "img",
	INPUT:            "input",
	ISINDEX:          "isindex",
	KEYGEN:           "keygen",
	LINK:             "link",
	MENUITEM:         "menuitem",
	META:             "meta",
	NEXTID:           "nextid",
	PARAM:            "param",
	SOURCE:           "source",
	TRACK:            "track",
	WBR:              "wbr",
	end_of_void_tags: "",
	A:                "a",
	ABBR:             "abbr",
	ADDRESS:          "address",
	ARTICLE:          "article",
	ASIDE:            "aside",
	AUDIO:            "audio",
	B:                "b",
	BDI:              "bdi",
	BDO:              "bdo",
	BLOCKQUOTE:       "blockquote",
	BODY:             "body",
	BUTTON:           "button",
	CANVAS:           "canvas",
	CAPTION:          "caption",
	CITE:             "cite",
	CODE:             "code",
	COLGROUP:         "colgroup",
	DATA:             "data",
	DATALIST:         "datalist",
	DD:               "dd",
	DEL:              "del",
	DETAILS:          "details",
	DFN:              "dfn",
	DIALOG:           "dialog",
	DIV:              "div",
	DL:               "dl",
	DT:               "dt",
	EM:               "em",
	FIELDSET:         "fieldset",
	FIGCAPTION:       "figcaption",
	FIGURE:           "figure",
	FOOTER:           "footer",
	FORM:             "form",
	H1:               "h1",
	H2:               "h2",
	H3:               "h3",
	H4:               "h4",
	H5:               "h5",
	H6:               "h6",
	HEAD:             "head",
	HEADER:           "header",
	HGROUP:           "hgroup",
	HTML:             "html",
	I:                "i",
	IFRAME:           "iframe",
	INS:              "ins",
	KBD:              "kbd",
	LABEL:            "label",
	LEGEND:           "legend",
	LI:               "li",
	MAIN:             "main",
	MAP:              "map",
	MARK:             "mark",
	MATH:             "math",
	MENU:             "menu",
	METER:            "meter",
	NAV:              "nav",
	NOSCRIPT:         "noscript",
	OBJECT:           "object",
	OL:               "ol",
	OPTGROUP:         "optgroup",
	OPTION:           "option",
	OUTPUT:           "output",
	P:                "p",
	PICTURE:          "picture",
	PRE:              "pre",
	PROGRESS:         "progress",
	Q:                "q",
	RB:               "rb",
	RP:               "rp",
	RT:               "rt",
	RTC:              "rtc",
	RUBY:             "ruby",
	S:                "s",
	SAMP:             "samp",
	SCRIPT:           "script",
	SEARCH:           "search",
	SECTION:          "section",
	SELECT:           "select",
	SLOT:             "slot",
	SMALL:            "small",
	SPAN:             "span",
	STRONG:           "strong",
	STYLE:            "style",
	SUB:              "sub",
	SUMMARY:          "summary",
	SUP:              "sup",
	SVG:              "svg",
	TABLE:            "table",
	TBODY:            "tbody",
	TD:               "td",
	TEMPLATE:         "template",
	TEXTAREA:         "textarea",
	TFOOT:            "tfoot",
	TH:               "th",
	THEAD:            "thead",
	TIME:             "time",
	TITLE:            "title",
	TR:               "tr",
	U:                "u",
	UL:               "ul",
	VAR:              "var",
	VIDEO:            "video",
	RAW:              ":",
}

func FindTag(beg string) (matches []Kind) {
	if beg == "" {
		return nil
	}
	beg = strings.ToLower(beg)
	for kind, s := range tags {
		if !strings.HasPrefix(s, beg) {
			continue
		}
		if s == beg {
			matches = slices.Insert(matches, 0, Kind(kind))
			continue
		}
		matches = append(matches, Kind(kind))
	}
	return
}
