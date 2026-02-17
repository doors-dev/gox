package common

import (
	"crypto/rand"
	"math/big"
	"time"
)

func Jitter(min, max time.Duration) {
	if max <= 0 {
		panic("wtf")
	}
	if min < 0 {
		panic("wtf")
	}
	if max < min {
		panic("wtf")
	}
	span := max - min
	if span == 0 {
		time.Sleep(min)
		return
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(span)+1))
	if err != nil {
		time.Sleep(min + span/2)
		return
	}
	time.Sleep(min + time.Duration(n.Int64()))
}

