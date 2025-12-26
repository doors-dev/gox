package symbol

type Kind int

const (
	None          Kind = -1
	File          Kind = 1
	Module        Kind = 2
	Namespace     Kind = 3
	Package       Kind = 4
	Class         Kind = 5
	Method        Kind = 6
	Property      Kind = 7
	Field         Kind = 8
	Constructor   Kind = 9
	Enum          Kind = 10
	Interface     Kind = 11
	Function      Kind = 12
	Variable      Kind = 13
	Constant      Kind = 14
	String        Kind = 15
	Number        Kind = 16
	Boolean       Kind = 17
	Array         Kind = 18
	Object        Kind = 19
	Key           Kind = 20
	Null          Kind = 21
	EnumMember    Kind = 22
	Struct        Kind = 23
	Event         Kind = 24
	Operator      Kind = 25
	TypeParameter Kind = 26
)
