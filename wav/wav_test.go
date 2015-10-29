package wav

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"
)

func TestStringFormatting(t *testing.T) {
	if s := fmt.Sprintf("%v", uint32(123)); s != "123" {
		t.Error(s)
	}
}

func TestFormattingErrors(t *testing.T) {
	tests := []struct {
		name       string
		input      []byte
		shouldFail bool
	}{
		{
			"BIFF is not RIFF",
			makeWave("BIFF"),
			true,
		},

		{
			"valid empty sound data",
			makeWave("RIFF", uint32(4), "WAVE"),
			false,
		},

		{
			"only format chunk",
			makeWave("RIFF", uint32(4+8+16), "WAVE",
				"fmt ", uint32(16),
				uint16(1),
				uint16(2),
				uint32(44100),
				uint32(44100*4),
				uint16(2),
				uint16(16)),
			false,
		},
	}

	for _, test := range tests {
		r := bytes.NewReader(test.input)
		_, err := Load(r)

		if test.shouldFail && err == nil {
			t.Error(test.name, "- error expected")
		}

		if !test.shouldFail && err != nil {
			t.Error(test.name, "- unexpected error:", err)
		}
	}
}

func makeWave(data ...interface{}) []byte {
	buf := bytes.NewBuffer(nil)
	for _, d := range data {
		if s, ok := d.(string); ok {
			buf.WriteString(s)
		} else if i, ok := d.(uint32); ok {
			binary.Write(buf, binary.LittleEndian, i)
		} else if i, ok := d.(uint16); ok {
			binary.Write(buf, binary.LittleEndian, i)
		} else {
			panic("invalid test data type")
		}
	}
	return buf.Bytes()
}
