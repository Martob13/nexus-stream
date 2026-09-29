package ffi

/*
#cgo CFLAGS: -I${SRCDIR}/../../../core-rs/include
#cgo LDFLAGS: ${SRCDIR}/../../../core-rs/target/release/libcore_parser.a -lpthread -ldl -lm
#include "core_parser.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"unsafe"
)

type SignalMetrics struct {
	RMS  float64
	Peak float64
}

func CallRustEngine(frame []byte) (SignalMetrics, error) {
	if len(frame) < 12 {
		return SignalMetrics{}, errors.New("frame buffer smaller than 12-byte header")
	}

	var cMetrics C.SignalMetrics
	rawPtr := (*C.uint8_t)(unsafe.Pointer(&frame[0]))
	frameLen := C.size_t(len(frame))

	ret := C.parse_and_compute_metrics(rawPtr, frameLen, &cMetrics)
	if ret != 0 {
		return SignalMetrics{}, fmt.Errorf("rust core execution failed with error code: %d", int(ret))
	}

	return SignalMetrics{
		RMS:  float64(cMetrics.rms),
		Peak: float64(cMetrics.peak),
	}, nil
}
