package protocol_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/jasonahenderson/modeltap/internal/protocol"
)

// WU-094 P-6 flagged FrameReader's byte-by-byte ReadByte path as
// ~10× slower than scanner-based reads on multi-MB frames. Baseline
// the current hot path so regressions / improvements are measurable.

func BenchmarkFrameReader_1MBFrame(b *testing.B) {
	frame := make([]byte, 1<<20)
	for i := range frame {
		frame[i] = 'a'
	}
	frame[len(frame)-1] = '\n'

	b.ResetTimer()
	b.SetBytes(int64(len(frame)))
	for i := 0; i < b.N; i++ {
		r := protocol.NewFrameReader(bytes.NewReader(frame))
		buf, err := r.ReadFrame()
		if err != nil && err != io.EOF {
			b.Fatalf("ReadFrame: %v", err)
		}
		_ = buf
	}
}

func BenchmarkFrameReader_SmallFrames(b *testing.B) {
	// 1000 small frames — typical harness traffic shape.
	var sb strings.Builder
	for i := 0; i < 1000; i++ {
		sb.WriteString(`{"jsonrpc":"2.0","id":1,"method":"connection.ping"}` + "\n")
	}
	payload := sb.String()

	b.ResetTimer()
	b.SetBytes(int64(len(payload)))
	for i := 0; i < b.N; i++ {
		r := protocol.NewFrameReader(strings.NewReader(payload))
		for {
			_, err := r.ReadFrame()
			if err != nil {
				break
			}
		}
	}
}

func BenchmarkFrameWriter_1KBFrame(b *testing.B) {
	frame := make([]byte, 1024)
	for i := range frame {
		frame[i] = 'a'
	}
	b.ResetTimer()
	b.SetBytes(int64(len(frame)))
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		w := protocol.NewFrameWriter(&buf)
		if err := w.WriteFrame(frame); err != nil {
			b.Fatalf("WriteFrame: %v", err)
		}
	}
}
