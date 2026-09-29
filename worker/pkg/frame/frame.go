package frame

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

const (
	MagicByte0         = 0x55
	MagicByte1         = 0xAA
	MaxSamplesPerFrame = 65535
	HeaderSize         = 12
)

// EncodeBinaryFrame serializes telemetry samples into a deterministic binary format.
// Layout: [0..2] Magic (0x55, 0xAA) | [2..10] Timestamp LE | [10..12] Count LE | [12..] float64 LE array
func EncodeBinaryFrame(timestamp uint64, samples []float64) ([]byte, error) {
	n := len(samples)
	if n == 0 {
		return nil, errors.New("cannot encode empty samples array")
	}
	if n > MaxSamplesPerFrame {
		return nil, fmt.Errorf("sample count %d exceeds protocol limit of %d", n, MaxSamplesPerFrame)
	}

	buf := make([]byte, HeaderSize+(n*8))
	buf[0] = MagicByte0
	buf[1] = MagicByte1
	binary.LittleEndian.PutUint64(buf[2:10], timestamp)
	binary.LittleEndian.PutUint16(buf[10:12], uint16(n))

	for i, s := range samples {
		if math.IsNaN(s) || math.IsInf(s, 0) {
			return nil, fmt.Errorf("sample index %d is not a finite number", i)
		}
		bits := math.Float64bits(s)
		offset := HeaderSize + (i * 8)
		binary.LittleEndian.PutUint64(buf[offset:offset+8], bits)
	}

	return buf, nil
}
