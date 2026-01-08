package common

import "unsafe"

func Bytes(s string) []byte {
	// pointer to the first byte of the string
	p := unsafe.StringData(s)
	// construct a slice header from that pointer
	return unsafe.Slice(p, len(s))
}

/*

func String(b []byte) string {
    // pointer to the first byte of the slice
    p := unsafe.Pointer(&b[0])
    // construct a string header from that pointer
    return *(*string)(unsafe.Pointer(&p))
} */
