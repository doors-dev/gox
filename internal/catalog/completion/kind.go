package completion

type Kind int

const (
	Text          Kind = 1
	Method        Kind = 2
	Function      Kind = 3
	Constructor   Kind = 4
	Field         Kind = 5
	Variable      Kind = 6
	Class         Kind = 7
	Interface     Kind = 8
	Module        Kind = 9
	Property      Kind = 10
	Unit          Kind = 11
	Value         Kind = 12
	Enum          Kind = 13
	Keyword       Kind = 14
	Snippet       Kind = 15
	Color         Kind = 16
	File          Kind = 17
	Reference     Kind = 18
	Folder        Kind = 19
	EnumMember    Kind = 20
	Constant      Kind = 21
	Struct        Kind = 22
	Event         Kind = 23
	Operator      Kind = 24
	TypeParameter Kind = 25
)
