package rust

/*
#cgo LDFLAGS: -L${SRCDIR}/../../rust/target/release -lgox
#include <stdint.h>
#include <stdlib.h>

typedef struct {
    uint8_t* ptr;
    size_t   len;
    size_t   cap;
    int32_t  res;
} Buf;

Buf format(const uint8_t* ptr_in, size_t len_in);
void free_buf(uint8_t* ptr, size_t len);
*/
import "C"

import (
	"fmt"
	"math"
	"unsafe"
)

func Format(in []byte) ([]byte, error) {
	var inPtr *C.uint8_t
	if len(in) > 0 {
		inPtr = (*C.uint8_t)(unsafe.Pointer(&in[0]))
	}

	res := C.format(inPtr, C.size_t(len(in)))
	switch res.res {
	case -1:
		return nil, nil
	case 0:
		defer C.free_buf(res.ptr, res.len)
		if res.len > C.size_t(math.MaxInt32) {
			return nil, fmt.Errorf("output too large: %d", uint64(res.len))
		}
		out := C.GoBytes(unsafe.Pointer(res.ptr), C.int(res.len))
		return out, nil
	default:
		return nil, fmt.Errorf("format failed: %d", int32(res.res))
	}
}
