package symboldict

import (
	"encoding/binary"
	"github.com/unidoc/unidoc/common"
	"github.com/unidoc/unidoc/pdf/internal/jbig2/bitmap"
	"github.com/unidoc/unidoc/pdf/internal/jbig2/decoder/arithmetic"
	"github.com/unidoc/unidoc/pdf/internal/jbig2/decoder/container"
	"github.com/unidoc/unidoc/pdf/internal/jbig2/decoder/huffman"
	"github.com/unidoc/unidoc/pdf/internal/jbig2/segment/flags"
	"github.com/unidoc/unidoc/pdf/internal/jbig2/segment/header"
	"github.com/unidoc/unidoc/pdf/internal/jbig2/segment/kind"
	"math"

	// "github.com/unidoc/unidoc/pdf/internal/jbig2/bitmap"
	"github.com/unidoc/unidoc/pdf/internal/jbig2/reader"
	"github.com/unidoc/unidoc/pdf/internal/jbig2/segment/model"
)

// SymbolDictionarySegment is the struct that represents Symbol Dictionary Segment for JBIG2
// encoding
type SymbolDictionarySegment struct {
	*model.Segment

	// SDFlags are the Symbol Dictionary Segment flags
	SDFlags *SymbolDictFlags

	AdaptiveTemplateX []int8
	AdaptiveTemplateY []int8

	RAdaptiveTemplateX []int8
	RAdaptiveTemplateY []int8

	Bitmaps []*bitmap.Bitmap

	// privates
	ExportedSymbolsNumber uint32
	NewSymbolsNumber      uint32

	GenericRegionStats    *arithmetic.DecoderStats
	RefinementRegionStats *arithmetic.DecoderStats
}

// New creates new SymbolDictionarySegment
func New(d *container.Decoder, h *header.Header) *SymbolDictionarySegment {
	s := &SymbolDictionarySegment{
		Segment: model.New(d, h),
		SDFlags: &SymbolDictFlags{
			Flags: flags.New(),
		},
		AdaptiveTemplateX: make([]int8, 4),
		AdaptiveTemplateY: make([]int8, 4),

		RAdaptiveTemplateX: make([]int8, 2),
		RAdaptiveTemplateY: make([]int8, 2),
	}

	return s

}

// Decode decodes the SymbolDictionarySegment from the jbig2 encoding
func (s *SymbolDictionarySegment) Decode(r *reader.Reader) error {
	common.Log.Debug("[DECODE] Symbol Dictionary Segment 'Decode' starts")
	defer func() { common.Log.Debug("[DECODE] Symbol Dictionary Segment 'Decode' finished") }()

	// Read Symbol Dictionary Segment Flags
	if err := s.readFlags(r); err != nil {
		common.Log.Debug("readFlags failed.")
		return err
	}

	var (
		inputSymbolsNumber uint32
	)

	for i := 0; i < s.Header.ReferredToSegmentCount; i++ {
		seg := s.Decoders.FindSegment(s.Header.ReferredToSegments[i])
		if seg.Kind() == kind.SymbolDictionary {
			inputSymbolsNumber += (seg).(*SymbolDictionarySegment).ExportedSymbolsNumber
		}
	}

	common.Log.Debug("InputSymbolsNumber: %d", inputSymbolsNumber)

	var symbolCodeLength int

	for i := 1; uint32(i) < inputSymbolsNumber+s.NewSymbolsNumber; i <<= 1 {
		symbolCodeLength++
	}

	common.Log.Debug("SymbolCodeLength: %d", symbolCodeLength)

	var bitmaps []*bitmap.Bitmap = make([]*bitmap.Bitmap, inputSymbolsNumber+s.NewSymbolsNumber)

	var k int

	var inputSymbolDictionary *SymbolDictionarySegment

	for i := 0; i < s.Header.ReferredToSegmentCount; i++ {
		seg := s.Decoders.FindSegment(s.Header.ReferredToSegments[i])
		if seg.Kind() == kind.SymbolDictionary {
			inputSymbolDictionary = seg.(*SymbolDictionarySegment)
			for j := 0; uint32(j) < inputSymbolDictionary.ExportedSymbolsNumber; j++ {
				bitmaps[k] = inputSymbolDictionary.Bitmaps[j]
				k += 1
				j += 1
			}
		}
	}
	common.Log.Debug("Copied bitmaps from inputsymboldictionary.")

	var (
		huffmanDHTable [][]int
		huffmanDWTable [][]int

		huffmanBMSizeTable  [][]int
		huffmanAggInstTable [][]int
	)

	sdHuff := s.SDFlags.GetValue(SD_HUFF) != 0

	i := 0
	if sdHuff {
		switch s.SDFlags.GetValue(SD_HUFF_DH) {
		case 0:
			huffmanDHTable = huffman.TableD
		case 1:
			huffmanDHTable = huffman.TableE
		}

		switch s.SDFlags.GetValue(SD_HUFF_DW) {
		case 0:
			huffmanDWTable = huffman.TableB
		case 1:
			huffmanDWTable = huffman.TableC
		}

		if s.SDFlags.GetValue(SD_HUFF_BM_SIZE) == 0 {
			huffmanBMSizeTable = huffman.TableA
		}

		if s.SDFlags.GetValue(SD_HUFF_AGG_INST) == 0 {
			huffmanAggInstTable = huffman.TableA
		}
	}

	if !sdHuff {
		sdTemplate := s.SDFlags.GetValue(SD_TEMPLATE)
		if s.SDFlags.GetValue(BITMAP_CC_USED) != 0 && inputSymbolDictionary != nil {
			s.Decoders.Arithmetic.ResetGenericStats(
				sdTemplate,
				inputSymbolDictionary.GenericRegionStats,
			)
		} else {
			s.Decoders.Arithmetic.ResetGenericStats(
				sdTemplate,
				nil,
			)
		}
		s.Decoders.Arithmetic.ResetIntStats(symbolCodeLength)

		common.Log.Debug("Arithmetic->Start begins")
		if err := s.Decoders.Arithmetic.Start(r); err != nil {
			common.Log.Error("Arithmetic Start failed: %v", err)
			return err
		}
		common.Log.Debug("Arithmetic->Start finished")
	}

	sdRefinementTemplate := s.SDFlags.GetValue(SD_R_TEMPLATE)

	if s.SDFlags.GetValue(SD_REF_AGG) != 0 {
		if s.SDFlags.GetValue(BITMAP_CC_USED) != 0 && inputSymbolDictionary != nil {
			s.Decoders.Arithmetic.ResetRefinementStats(
				sdRefinementTemplate,
				inputSymbolDictionary.RefinementRegionStats,
			)
		} else {
			s.Decoders.Arithmetic.ResetRefinementStats(
				sdRefinementTemplate,
				nil,
			)
		}
	}

	var (
		deltaWidths []int = make([]int, s.NewSymbolsNumber)
		deltaHeight int
	)

	i = 0

	for i < int(s.NewSymbolsNumber) {
		var instanceDeltaHeight int
		var err error

		// if huffman use huffman decoder
		if sdHuff {
			instanceDeltaHeight, _, err = s.Decoders.Huffman.DecodeInt(r, huffmanDHTable)
			if err != nil {
				common.Log.Error("Decoders Huffman->DecodeInt(huffmanDHTable). %v", err)
				return err
			}
			common.Log.Debug("Huffman->DecodeInt(huffmanDHTable) finished")
		} else {
			// otherwise use the arithmetic decoder
			instanceDeltaHeight, _, err = s.Decoders.Arithmetic.DecodeInt(
				r, s.Decoders.Arithmetic.IadhStats,
			)
			if err != nil {
				common.Log.Error("Decoders Arithmetic->DecodeInt(iadhStats) failed. %v", err)
				return err
			}
			common.Log.Debug("Arithmetic->DecodeInt(IadhStats) finished")
		}

		common.Log.Debug("Instance Delta Height: %d", instanceDeltaHeight)

		// check the instanceDeltaHeight value
		if instanceDeltaHeight < 0 && (-instanceDeltaHeight >= deltaHeight) {
			common.Log.Debug("Bad delta-height value in JBIG2 symbol dictionary. InstanceDeltaHeight: %v. DeltaHeight: %v", instanceDeltaHeight, deltaHeight)
		}

		deltaHeight += instanceDeltaHeight
		var (
			symbolWidth int
			totalWidth  int
			j           int = i
		)
		common.Log.Debug("deltaHeight: %v, j: %v", deltaHeight, j)

		for {

			var (
				deltaWidth     int
				deltaWidthBool bool
			)

			if sdHuff {
				deltaWidth, deltaWidthBool, err = s.Decoders.Huffman.DecodeInt(r, huffmanDWTable)
				if err != nil {
					common.Log.Error("Huffman DecodeInt->(huffmanDWTable) failed. %v", err)
					return err
				}
			} else {
				deltaWidth, deltaWidthBool, err = s.Decoders.Arithmetic.DecodeInt(r, s.Decoders.Arithmetic.IadwStats)
				if err != nil {
					common.Log.Error("Arithmetic DecodeInt->(iadwStats) failed: %v", err)
					return err
				}
			}

			common.Log.Debug("DeltaWidth: %d, %v", deltaWidth, deltaWidthBool)

			if !deltaWidthBool {
				break
			}

			if deltaWidth < 0 && -deltaWidth >= symbolWidth {
				common.Log.Debug("Bad delta-width value in JBIG2 symbol dictionary. DeltaWidth: %v. SymbolWidth: %v", deltaWidth, symbolWidth)
			}

			symbolWidth += deltaWidth

			sdRefinement := s.SDFlags.GetValue(SD_REF_AGG)

			if sdHuff && sdRefinement == 0 {
				deltaWidths[i] = symbolWidth
				totalWidth += symbolWidth

			} else if sdRefinement == 1 {

				var refAggNum int

				if sdHuff {
					refAggNum, _, err = s.Decoders.Huffman.DecodeInt(r, huffmanAggInstTable)
					if err != nil {
						common.Log.Debug("Huffman DecodeInt(huffmanAggInstTable) failed: %v", err)
						return err
					}
				} else {
					refAggNum, _, err = s.Decoders.Arithmetic.DecodeInt(r, s.Decoders.Arithmetic.IaaiStats)
					if err != nil {
						common.Log.Debug("Arithmetic->DecodeInt(iaaiStats) failed. %v", err)
						return err
					}
				}

				if refAggNum == 1 {
					var (
						symbolID                 uint64
						referenceDX, referenceDY int
					)

					if sdHuff {
						symbolID, err = r.ReadBits(byte(symbolCodeLength))
						if err != nil {
							common.Log.Debug("ReadBits(symbolCodeLength) failed: %v", err)
							return err
						}

						referenceDX, _, err = s.Decoders.Huffman.DecodeInt(r, huffman.TableO)
						if err != nil {
							common.Log.Debug("Huffman DecodeInt ReferenceDX failed. %v", err)
							return err
						}

						referenceDY, _, err = s.Decoders.Huffman.DecodeInt(r, huffman.TableO)
						if err != nil {
							common.Log.Debug("Huffman DecodeInt ReferenceDY failed. %v", err)
							return err
						}
					} else {

						symbolID, err = s.Decoders.Arithmetic.DecodeIAID(r, uint64(symbolCodeLength), s.Decoders.Arithmetic.IaidStats)
						if err != nil {
							common.Log.Debug("Arithmetic->DecodeIAID(symbolCodeLength,IaidStats) failed: %v", err)
							return err
						}

						referenceDX, _, err = s.Decoders.Arithmetic.DecodeInt(r, s.Decoders.Arithmetic.IardxStats)
						if err != nil {
							common.Log.Debug("Arithmetic->DecodeInt(IardxStats) failed: %v", err)
							return err
						}

						referenceDY, _, err = s.Decoders.Arithmetic.DecodeInt(r, s.Decoders.Arithmetic.IardyStats)
						if err != nil {
							common.Log.Debug("Arithmetic->DecodeInt(IardyStats) failed: %v", err)
							return err
						}
					}

					referedToBitmap := bitmaps[symbolID]

					bm := bitmap.New(symbolWidth, deltaHeight, s.Decoders)
					err = bm.ReadGenericRefinementRegion(
						r,
						sdRefinementTemplate,
						false,
						referedToBitmap,
						referenceDX, referenceDY,
						s.AdaptiveTemplateX, s.AdaptiveTemplateY,
					)
					if err != nil {
						common.Log.Debug("Bitmap->ReadGenericRefinementRegion failed: %v", err)
						return err
					}
					common.Log.Debug("Add bitmap: %d at index: %d", bm.BitmapNumber, inputSymbolsNumber+uint32(i))
					bitmaps[inputSymbolsNumber+uint32(i)] = bm
				} else {
					common.Log.Debug("Creating new Bitmap with no refinment region")
					bm := bitmap.New(symbolWidth, deltaHeight, s.Decoders)
					err = bm.ReadTextRegion(r, sdHuff, true, uint(refAggNum), 0,
						uint(inputSymbolsNumber)+uint(i), nil, symbolCodeLength,
						bitmaps, 0, 0, false, 1, 0,
						huffman.TableF, huffman.TableH, huffman.TableK,
						huffman.TableO, huffman.TableO, huffman.TableO, huffman.TableO,
						huffman.TableA, sdRefinementTemplate, s.AdaptiveTemplateX, s.AdaptiveTemplateY,
					)
					common.Log.Debug("Add bitmap: %d at index: %d", bm.BitmapNumber, inputSymbolsNumber+uint32(i))
					bitmaps[int(inputSymbolsNumber)+i] = bm
				}
			} else {
				common.Log.Debug("Not a SD_REF_AGG")
				bm := bitmap.New(symbolWidth, deltaHeight, s.Decoders)
				err := bm.Read(r, false, s.SDFlags.GetValue(SD_TEMPLATE), false, false, nil, s.AdaptiveTemplateX, s.AdaptiveTemplateY, 0)
				if err != nil {
					common.Log.Debug("bitmap.Read i: '%d', j: '%d' failed. %v", i, j, err)
					return err
				}
			}

			i++
			common.Log.Debug("i: %v", i)
			// time.Sleep(time.Millisecond * 100)
		}

		if sdHuff && s.SDFlags.GetValue(SD_REF_AGG) == 0 {
			bmSize, _, err := s.Decoders.Huffman.DecodeInt(r, huffmanBMSizeTable)
			if err != nil {
				common.Log.Debug("Huffman->DecodeInt(huffmanBMSizeTable) failed: %v", err)
				return err
			}

			r.ConsumeRemainingBits()

			collectiveBitMap := bitmap.New(totalWidth, deltaHeight, s.Decoders)
			if bmSize == 0 {
				var (
					padding     int    = totalWidth % 8
					bytesPerRow int    = int(math.Ceil(float64(totalWidth) / 8))
					size        int    = deltaHeight * ((totalWidth + 7) >> 3)
					buf         []byte = make([]byte, size)
				)

				_, err = r.Read(buf)
				if err != nil {
					common.Log.Debug("Read bitmap buf bytes: %v", err)
					return err
				}

				var logicalMap [][]byte = make([][]byte, deltaHeight)
				for i := 0; i < deltaHeight; i++ {
					logicalMap[i] = make([]byte, bytesPerRow)
				}

				var count int
				for row := 0; row < deltaHeight; row++ {
					for col := 0; col < bytesPerRow; col++ {
						logicalMap[row][col] = buf[count]
						count++

					}
				}

				var collectiveBitMapRow, collectiveBitmapCol int
				for row := 0; row < deltaHeight; row++ {
					for col := 0; col < bytesPerRow; col++ {
						if col == bytesPerRow-1 {
							currentByte := logicalMap[row][col]
							for bitPointer := 7; bitPointer >= padding; bitPointer-- {
								mask := byte(1 << byte(bitPointer))
								bit := int((currentByte & mask) >> byte(bitPointer))

								collectiveBitMap.SetPixel(collectiveBitmapCol, collectiveBitMapRow, bit)
								collectiveBitmapCol++
							}
							collectiveBitMapRow++
							collectiveBitmapCol = 0
						} else {
							currentByte := logicalMap[row][col]
							for bitPointer := 7; bitPointer >= 0; bitPointer-- {
								mask := byte(1 << byte(bitPointer))
								bit := int((currentByte & mask) >> byte(bitPointer))

								collectiveBitMap.SetPixel(collectiveBitmapCol, collectiveBitMapRow, bit)
								collectiveBitmapCol++
							}
						}
					}
				}
			} else {
				err := collectiveBitMap.Read(r, true, 0, false, false, nil, nil, nil, bmSize)
				if err != nil {
					common.Log.Debug("CollectiveBitMap.Read failed: %v", err)
					return err
				}
			}

			var x int

			for j < i {
				bitmaps[int(inputSymbolsNumber)+j], err = collectiveBitMap.GetSlice(x, 0, deltaWidths[j], deltaHeight)

				if err != nil {
					return err
				}
				x += deltaWidths[j]
				j++
			}
		}
	}

	s.Bitmaps = make([]*bitmap.Bitmap, s.ExportedSymbolsNumber)
	i = 0
	var (
		j      int = i
		export bool
		err    error
	)

	for uint32(i) < inputSymbolsNumber+s.NewSymbolsNumber {
		var run int
		if sdHuff {
			run, _, err = s.Decoders.Huffman.DecodeInt(r, huffman.TableA)
			if err != nil {
				common.Log.Debug("Huffman->DecodeInt(huffmanTableA) at i : %d. %v", i, err)
				return err
			}
		} else {
			run, _, err = s.Decoders.Arithmetic.DecodeInt(r, s.Decoders.Arithmetic.IaexStats)
			if err != nil {
				common.Log.Debug("Arithmetic->DecodeInt(iaexStats) failed at i= %d. %v", i, err)
				return err
			}
		}

		if export {
			for cnt := 0; cnt < run; cnt++ {
				s.Bitmaps[j] = bitmaps[i]
				j++
				i++
			}
		} else {
			i += run
		}
		export = !export
	}

	if !sdHuff && s.SDFlags.GetValue(BITMAP_CC_RETAINED) == 1 {
		s.GenericRegionStats = s.GenericRegionStats.Copy()
		if s.SDFlags.GetValue(SD_REF_AGG) == 1 {
			s.RefinementRegionStats = s.RefinementRegionStats.Copy()
		}
	}

	// consume any remaining bits
	r.ConsumeRemainingBits()

	return nil
}

func (s *SymbolDictionarySegment) readFlags(r *reader.Reader) error {
	common.Log.Debug("[readFlags] SymbolDictionarySegment 'readFlags' starts")
	defer func() { common.Log.Debug("[readFlags] SymbolDictionarySegment 'readFlags' finished.") }()
	var flagsField []byte = make([]byte, 2)

	if _, err := r.Read(flagsField); err != nil {
		common.Log.Error("Reading SymbolDictionarySegment flags failed. %v", err)
		return err
	}

	flagValue := binary.BigEndian.Uint16(flagsField)

	common.Log.Debug("SymbolDictionryFlags SetValue: %16b", flagValue)
	s.SDFlags.SetValue(int(flagValue))

	if s.SDFlags.GetValue(SD_HUFF) == 0 {
		var buf []byte = make([]byte, 2)
		if s.SDFlags.GetValue(SD_TEMPLATE) == 0 {

			for i := 0; i < 4; i++ {
				_, err := r.Read(buf)
				if err != nil {
					common.Log.Error("Reading AdaptiveTemplate bytes at %d failed.", i)
					return err
				}

				// Set AdaptiveTemplates at 'i'
				s.AdaptiveTemplateX[i] = int8(buf[0])
				s.AdaptiveTemplateY[i] = int8(buf[1])
			}
		} else {
			if _, err := r.Read(buf); err != nil {
				common.Log.Error("Reading AdaptiveTemplate 0th elem failed. %v", err)
				return err
			}

			s.AdaptiveTemplateX[0] = int8(buf[0])
			s.AdaptiveTemplateY[0] = int8(buf[1])
		}
	}

	if s.SDFlags.GetValue(SD_REF_AGG) != 0 && s.SDFlags.GetValue(SD_R_TEMPLATE) == 0 {
		var buf []byte = make([]byte, 4)
		if _, err := r.Read(buf); err != nil {
			common.Log.Error("Reading RAdaptiveTemplate bytes failed. %v", err)
			return err
		}

		// Set RAdaptiveTemplates
		s.RAdaptiveTemplateX[0] = int8(buf[0])
		s.RAdaptiveTemplateY[0] = int8(buf[1])
		s.RAdaptiveTemplateX[1] = int8(buf[2])
		s.RAdaptiveTemplateY[1] = int8(buf[3])
	}

	// Read Number of exported symbols field
	var buf []byte = make([]byte, 4)

	if _, err := r.Read(buf); err != nil {
		common.Log.Error("Reading bytes for Exported Symbols Number failed. %v", err)
		return err
	}

	s.ExportedSymbolsNumber = binary.BigEndian.Uint32(buf)
	common.Log.Debug("Exported Symbols Number: %d", s.ExportedSymbolsNumber)

	// Read Number of new symbols
	if _, err := r.Read(buf); err != nil {
		common.Log.Error("Reading bytes for New Symbols Number failed: %v", err)
		return err
	}

	s.NewSymbolsNumber = binary.BigEndian.Uint32(buf)

	common.Log.Debug("New Symbols Number: %v", s.NewSymbolsNumber)

	return nil

}

// func (s *SymbolDictionarySegment) ExportedSymbolsNumber() int {
// 	return int
// }

// func (s *SymbolDictionarySegment) ListBitmaps() []*bitmap.Bitmap {

// }