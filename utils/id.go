package utils

type id struct{ _ int }

type ID = *id

func NewID() ID {
	return &id{}
}

