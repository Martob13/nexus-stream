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

	"github.com/Martob13/nexus-stream/worker/pkg/frame"
)

type SignalMetrics struct {
	RMS  float64
	Peak float64
}

func CallRustEngine(rawFrame []byte) (SignalMetrics, error) {
	if len(rawFrame) < frame.HeaderSize {
		return SignalMetrics{}, errors.New("frame buffer smaller than header size")
	}

	var cMetrics C.SignalMetrics
	rawPtr := (*C.uint8_t)(unsafe.Pointer(&rawFrame[0]))
	frameLen := C.size_t(len(rawFrame))

	ret := C.parse_and_compute_metrics(rawPtr, frameLen, &cMetrics)
	if ret != 0 {
		return SignalMetrics{}, fmt.Errorf("rust core execution failed with error code: %d", int(ret))
	}

	return SignalMetrics{
		RMS:  float64(cMetrics.rms),
		Peak: float64(cMetrics.peak),
	}, nil
}
