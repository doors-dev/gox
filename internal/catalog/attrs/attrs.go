package attrs

import (
	"cmp"
	"slices"
	"strings"

	"github.com/doors-dev/gox/internal/catalog/tags"
)

type Kind int

func (a Kind) isBool() bool {
	panic("unimplemented")
}

func (a Kind) String() string {
	return attrs[a]
}

func (a Kind) IsBool() bool {
	return a < end_of_bool_attrs
}

func (a Kind) IsEqual(attr string) bool {
	return strings.EqualFold(a.String(), attr)
}

const (
	ASYNC Kind = iota
	AUTOPLAY
	CHECKED
	CONTROLS
	DEFAULT
	DEFER
	DISABLED
	DOWNLOAD
	HIDDEN
	ISMAP
	LOOP
	MULTIPLE
	MUTED
	NOVALIDATE
	OPEN
	PLAYSINLINE
	READONLY
	REQUIRED
	REVERSED
	SELECTED
	end_of_bool_attrs
	ACCEPT
	ACCEPT_CHARSET
	ACCESSKEY
	ACTION
	ALIGN
	ALLOW
	ALPHA
	ALT
	AS
	AUTOCAPITALIZE
	AUTOCOMPLETE
	BACKGROUND
	BGCOLOR
	BORDER
	CAPTURE
	CHARSET
	CITE
	CLASS
	COLOR
	COLORSPACE
	COLS
	COLSPAN
	CONTENT
	CONTENTEDITABLE
	COORDS
	CROSSORIGIN
	CSP
	DATA
	DATETIME
	DECODING
	DIR
	DIRNAME
	DRAGGABLE
	ENCTYPE
	ENTERKEYHINT
	FETCHPRIORITY
	FOR
	FORM
	FORMACTION
	FORMENCTYPE
	FORMMETHOD
	FORMNOVALIDATE
	FORMTARGET
	HEADERS
	HEIGHT
	HIGH
	HREF
	HREFLANG
	HTTP_EQUIV
	ID
	INTEGRITY
	INPUTMODE
	ITEMPROP
	KIND
	LABEL
	LANG
	LANGUAGE
	LOADING
	LIST
	LOW
	MAX
	MAXLENGTH
	MINLENGTH
	MEDIA
	METHOD
	MIN
	NAME
	OPTIMUM
	PATTERN
	PING
	PLACEHOLDER
	POSTER
	PRELOAD
	REFERRERPOLICY
	REL
	ROLE
	ROWS
	ROWSPAN
	SANDBOX
	SCOPE
	SHAPE
	SIZE
	SIZES
	SLOT
	SPAN
	SPELLCHECK
	SRC
	SRCDOC
	SRCLANG
	SRCSET
	START
	STEP
	STYLE
	SUMMARY
	TABINDEX
	TARGET
	TITLE
	TRANSLATE
	TYPE
	USEMAP
	VALUE
	WIDTH
	WRAP
)

var attrs = [...]string{
	ASYNC:           "async",
	AUTOPLAY:        "autoplay",
	CHECKED:         "checked",
	CONTROLS:        "controls",
	DEFAULT:         "default",
	DEFER:           "defer",
	DISABLED:        "disabled",
	DOWNLOAD:        "download",
	HIDDEN:          "hidden",
	ISMAP:           "ismap",
	LOOP:            "loop",
	MULTIPLE:        "multiple",
	MUTED:           "muted",
	NOVALIDATE:      "novalidate",
	OPEN:            "open",
	PLAYSINLINE:     "playsinline",
	READONLY:        "readonly",
	REQUIRED:        "required",
	REVERSED:        "reversed",
	SELECTED:        "selected",
	ACCEPT:          "accept",
	ACCEPT_CHARSET:  "accept-charset",
	ACCESSKEY:       "accesskey",
	ACTION:          "action",
	ALIGN:           "align",
	ALLOW:           "allow",
	ALPHA:           "alpha",
	ALT:             "alt",
	AS:              "as",
	AUTOCAPITALIZE:  "autocapitalize",
	AUTOCOMPLETE:    "autocomplete",
	BACKGROUND:      "background",
	BGCOLOR:         "bgcolor",
	BORDER:          "border",
	CAPTURE:         "capture",
	CHARSET:         "charset",
	CITE:            "cite",
	CLASS:           "class",
	COLOR:           "color",
	COLORSPACE:      "colorspace",
	COLS:            "cols",
	COLSPAN:         "colspan",
	CONTENT:         "content",
	CONTENTEDITABLE: "contenteditable",
	COORDS:          "coords",
	CROSSORIGIN:     "crossorigin",
	CSP:             "csp",
	DATA:            "data",
	DATETIME:        "datetime",
	DECODING:        "decoding",
	DIR:             "dir",
	DIRNAME:         "dirname",
	DRAGGABLE:       "draggable",
	ENCTYPE:         "enctype",
	ENTERKEYHINT:    "enterkeyhint",
	FETCHPRIORITY:   "fetchpriority",
	FOR:             "for",
	FORM:            "form",
	FORMACTION:      "formaction",
	FORMENCTYPE:     "formenctype",
	FORMMETHOD:      "formmethod",
	FORMNOVALIDATE:  "formnovalidate",
	FORMTARGET:      "formtarget",
	HEADERS:         "headers",
	HEIGHT:          "height",
	HIGH:            "high",
	HREF:            "href",
	HREFLANG:        "hreflang",
	HTTP_EQUIV:      "http-equiv",
	ID:              "id",
	INTEGRITY:       "integrity",
	INPUTMODE:       "inputmode",
	ITEMPROP:        "itemprop",
	KIND:            "kind",
	LABEL:           "label",
	LANG:            "lang",
	LANGUAGE:        "language",
	LOADING:         "loading",
	LIST:            "list",
	LOW:             "low",
	MAX:             "max",
	MAXLENGTH:       "maxlength",
	MINLENGTH:       "minlength",
	MEDIA:           "media",
	METHOD:          "method",
	MIN:             "min",
	NAME:            "name",
	OPTIMUM:         "optimum",
	PATTERN:         "pattern",
	PING:            "ping",
	PLACEHOLDER:     "placeholder",
	POSTER:          "poster",
	PRELOAD:         "preload",
	REFERRERPOLICY:  "referrerpolicy",
	REL:             "rel",
	ROLE:            "role",
	ROWS:            "rows",
	ROWSPAN:         "rowspan",
	SANDBOX:         "sandbox",
	SCOPE:           "scope",
	SHAPE:           "shape",
	SIZE:            "size",
	SIZES:           "sizes",
	SLOT:            "slot",
	SPAN:            "span",
	SPELLCHECK:      "spellcheck",
	SRC:             "src",
	SRCDOC:          "srcdoc",
	SRCLANG:         "srclang",
	SRCSET:          "srcset",
	START:           "start",
	STEP:            "step",
	STYLE:           "style",
	SUMMARY:         "summary",
	TABINDEX:        "tabindex",
	TARGET:          "target",
	TITLE:           "title",
	TRANSLATE:       "translate",
	TYPE:            "type",
	USEMAP:          "usemap",
	VALUE:           "value",
	WIDTH:           "width",
	WRAP:            "wrap",
}

var attrTags = map[Kind][]tags.Kind{
	ASYNC:           {tags.SCRIPT},
	AUTOPLAY:        {tags.AUDIO, tags.VIDEO},
	CHECKED:         {tags.INPUT},
	CONTROLS:        {tags.AUDIO, tags.VIDEO},
	DEFAULT:         {tags.TRACK},
	DEFER:           {tags.SCRIPT},
	DISABLED:        {tags.BUTTON, tags.FIELDSET, tags.INPUT, tags.OPTGROUP, tags.OPTION, tags.SELECT, tags.TEXTAREA},
	DOWNLOAD:        {tags.A, tags.AREA},
	HIDDEN:          {tags.ANY},
	ISMAP:           {tags.IMG},
	LOOP:            {tags.AUDIO, tags.VIDEO},
	MULTIPLE:        {tags.INPUT, tags.SELECT},
	MUTED:           {tags.AUDIO, tags.VIDEO},
	NOVALIDATE:      {tags.FORM},
	OPEN:            {tags.DETAILS, tags.DIALOG},
	PLAYSINLINE:     {tags.VIDEO},
	READONLY:        {tags.INPUT, tags.TEXTAREA},
	REQUIRED:        {tags.INPUT, tags.SELECT, tags.TEXTAREA},
	REVERSED:        {tags.OL},
	SELECTED:        {tags.OPTION},
	ACCEPT:          {tags.FORM, tags.INPUT},
	ACCEPT_CHARSET:  {tags.FORM},
	ACCESSKEY:       {tags.ANY},
	ACTION:          {tags.FORM},
	ALIGN:           {tags.CAPTION, tags.COL, tags.COLGROUP, tags.HR, tags.IFRAME, tags.IMG, tags.TABLE, tags.TBODY, tags.TD, tags.TFOOT, tags.TH, tags.THEAD, tags.TR},
	ALLOW:           {tags.IFRAME},
	ALPHA:           {tags.INPUT},
	ALT:             {tags.AREA, tags.IMG, tags.INPUT},
	AS:              {tags.LINK},
	AUTOCAPITALIZE:  {tags.ANY},
	AUTOCOMPLETE:    {tags.FORM, tags.INPUT, tags.SELECT, tags.TEXTAREA},
	BACKGROUND:      {tags.BODY, tags.TABLE, tags.TD, tags.TH},
	BGCOLOR:         {tags.BODY, tags.COL, tags.COLGROUP, tags.TABLE, tags.TBODY, tags.TFOOT, tags.TD, tags.TH, tags.TR},
	BORDER:          {tags.IMG, tags.OBJECT, tags.TABLE},
	CAPTURE:         {tags.INPUT},
	CHARSET:         {tags.META},
	CITE:            {tags.BLOCKQUOTE, tags.DEL, tags.INS, tags.Q},
	CLASS:           {tags.ANY},
	COLOR:           {tags.HR},
	COLORSPACE:      {tags.INPUT},
	COLS:            {tags.TEXTAREA},
	COLSPAN:         {tags.TD, tags.TH},
	CONTENT:         {tags.META},
	CONTENTEDITABLE: {tags.ANY},
	COORDS:          {tags.AREA},
	CROSSORIGIN:     {tags.AUDIO, tags.IMG, tags.LINK, tags.SCRIPT, tags.VIDEO},
	CSP:             {tags.IFRAME},
	DATA:            {tags.OBJECT},
	DATETIME:        {tags.DEL, tags.INS, tags.TIME},
	DECODING:        {tags.IMG},
	DIR:             {tags.ANY},
	DIRNAME:         {tags.INPUT, tags.TEXTAREA},
	DRAGGABLE:       {tags.ANY},
	ENCTYPE:         {tags.FORM},
	ENTERKEYHINT:    {tags.TEXTAREA},
	FETCHPRIORITY:   {tags.IMG, tags.LINK, tags.SCRIPT},
	FOR:             {tags.LABEL, tags.OUTPUT},
	FORM:            {tags.BUTTON, tags.FIELDSET, tags.INPUT, tags.OBJECT, tags.OUTPUT, tags.SELECT, tags.TEXTAREA},
	FORMACTION:      {tags.INPUT, tags.BUTTON},
	FORMENCTYPE:     {tags.BUTTON, tags.INPUT},
	FORMMETHOD:      {tags.BUTTON, tags.INPUT},
	FORMNOVALIDATE:  {tags.BUTTON, tags.INPUT},
	FORMTARGET:      {tags.BUTTON, tags.INPUT},
	HEADERS:         {tags.TD, tags.TH},
	HEIGHT:          {tags.CANVAS, tags.EMBED, tags.IFRAME, tags.IMG, tags.INPUT, tags.OBJECT, tags.VIDEO},
	HIGH:            {tags.METER},
	HREF:            {tags.A, tags.AREA, tags.BASE, tags.LINK},
	HREFLANG:        {tags.A, tags.LINK},
	HTTP_EQUIV:      {tags.META},
	ID:              {tags.ANY},
	INTEGRITY:       {tags.LINK, tags.SCRIPT},
	INPUTMODE:       {tags.TEXTAREA},
	ITEMPROP:        {tags.ANY},
	KIND:            {tags.TRACK},
	LABEL:           {tags.OPTGROUP, tags.OPTION, tags.TRACK},
	LANG:            {tags.ANY},
	LANGUAGE:        {tags.SCRIPT},
	LOADING:         {tags.IMG, tags.IFRAME},
	LIST:            {tags.INPUT},
	LOW:             {tags.METER},
	MAX:             {tags.INPUT, tags.METER, tags.PROGRESS},
	MAXLENGTH:       {tags.INPUT, tags.TEXTAREA},
	MINLENGTH:       {tags.INPUT, tags.TEXTAREA},
	MEDIA:           {tags.A, tags.AREA, tags.LINK, tags.SOURCE, tags.STYLE},
	METHOD:          {tags.FORM},
	MIN:             {tags.INPUT, tags.METER},
	NAME:            {tags.BUTTON, tags.FORM, tags.FIELDSET, tags.IFRAME, tags.INPUT, tags.OBJECT, tags.OUTPUT, tags.SELECT, tags.TEXTAREA, tags.MAP, tags.META, tags.PARAM},
	OPTIMUM:         {tags.METER},
	PATTERN:         {tags.INPUT},
	PING:            {tags.A, tags.AREA},
	PLACEHOLDER:     {tags.INPUT, tags.TEXTAREA},
	POSTER:          {tags.VIDEO},
	PRELOAD:         {tags.AUDIO, tags.VIDEO},
	REFERRERPOLICY:  {tags.A, tags.AREA, tags.IFRAME, tags.IMG, tags.LINK, tags.SCRIPT},
	REL:             {tags.A, tags.AREA, tags.LINK},
	ROLE:            {tags.ANY},
	ROWS:            {tags.TEXTAREA},
	ROWSPAN:         {tags.TD, tags.TH},
	SANDBOX:         {tags.IFRAME},
	SCOPE:           {tags.TH},
	SHAPE:           {tags.A, tags.AREA},
	SIZE:            {tags.INPUT, tags.SELECT},
	SIZES:           {tags.LINK, tags.IMG, tags.SOURCE},
	SLOT:            {tags.ANY},
	SPAN:            {tags.COL, tags.COLGROUP},
	SPELLCHECK:      {tags.ANY},
	SRC:             {tags.AUDIO, tags.EMBED, tags.IFRAME, tags.IMG, tags.INPUT, tags.SCRIPT, tags.SOURCE, tags.TRACK, tags.VIDEO},
	SRCDOC:          {tags.IFRAME},
	SRCLANG:         {tags.TRACK},
	SRCSET:          {tags.IMG, tags.SOURCE},
	START:           {tags.OL},
	STEP:            {tags.INPUT},
	STYLE:           {tags.ANY},
	SUMMARY:         {tags.TABLE},
	TABINDEX:        {tags.ANY},
	TARGET:          {tags.A, tags.AREA, tags.BASE, tags.FORM},
	TITLE:           {tags.ANY},
	TRANSLATE:       {tags.ANY},
	TYPE:            {tags.BUTTON, tags.INPUT, tags.EMBED, tags.OBJECT, tags.OL, tags.SCRIPT, tags.SOURCE, tags.STYLE, tags.MENU, tags.LINK},
	USEMAP:          {tags.IMG, tags.INPUT, tags.OBJECT},
	VALUE:           {tags.BUTTON, tags.DATA, tags.INPUT, tags.LI, tags.METER, tags.OPTION, tags.PROGRESS, tags.PARAM},
	WIDTH:           {tags.CANVAS, tags.EMBED, tags.IFRAME, tags.IMG, tags.INPUT, tags.OBJECT, tags.VIDEO},
	WRAP:            {tags.TEXTAREA},
}

var tagAttrs = make(map[string][]Kind)

func init() {
	for k, v := range attrTags {
		for _, kind := range v {
			tagAttrs[kind.String()] = append(tagAttrs[kind.String()], k)
		}
	}
}

func Find(tag string, beg string) (matches []Kind) {
	attrs, _ := tagAttrs[tag]
	global := tagAttrs[tags.ANY.String()]
	beg = strings.ToLower(beg)
	for _, attr := range attrs {
		if !strings.HasPrefix(attr.String(), beg) {
			continue
		}
		matches = append(matches, attr)
	}
	for _, attr := range global {
		if !strings.HasPrefix(attr.String(), beg) {
			continue
		}
		matches = append(matches, attr)
	}
	slices.SortFunc(matches, func(a, b Kind) int {
		return cmp.Compare(a.String(), b.String())
	})
	return
}
