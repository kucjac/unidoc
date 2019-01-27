package arithmetic

import (
	"github.com/stretchr/testify/assert"
	"github.com/unidoc/unidoc/common"
	"github.com/unidoc/unidoc/pdf/internal/jbig2/reader"
	"testing"
)

func TestArithmeticDecoder(t *testing.T) {
	if testing.Verbose() {
		common.SetLogger(common.NewConsoleLogger(common.LogLevelDebug))
	}
	var encoded []byte = []byte{
		0x84, 0xC7, 0x3B, 0xFC, 0xE1, 0xA1, 0x43, 0x04, 0x02,
		0x20, 0x00, 0x00, 0x41, 0x0D, 0xBB, 0x86, 0xF4, 0x31,
		0x7F, 0xFF, 0x88, 0xFF, 0x37, 0x47, 0x1A, 0xDB, 0x6A,
		0xDF, 0xFF, 0xAC,
	}

	// var original []byte = []byte{
	// 	0x00, 0x02, 0x00, 0x51, 0x00, 0x00, 0x00, 0xC0, 0x03, 0x52, 0x87,
	// 	0x2A, 0xAA, 0xAA, 0xAA, 0xAA, 0x82, 0xC0, 0x20, 0x00, 0xFC, 0xD7,
	// 	0x9E, 0xF6, 0xBF, 0x7F, 0xED, 0x90, 0x4F, 0x46, 0xA3, 0xBF,
	// }

	a := New()

	r := reader.New(encoded)

	err := a.Start(r)
	if assert.NoError(t, err) {
		// var b byte
		for j := 0; j < 8; j++ {
			for i := 0; i < 8; i++ {
				s, v, err := a.DecodeInt(r, a.IaaiStats)
				if assert.NoError(t, err) {
					t.Logf("%08b, %v", s, v)
				}
				// s, err := a.DecodeBit(r, 0, a.IaaiStats)
				// if assert.NoError(t, err) {
				// 	t.Logf("S: %v", s)
				// 	if s == 1 {
				// 		b |= 1 << uint(i)
				// 	}
				// }
			}

		}
	}

}
