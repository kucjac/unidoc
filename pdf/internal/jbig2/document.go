package jbig2

import (
	"errors"
	"fmt"
	"github.com/unidoc/unidoc/common"
	"github.com/unidoc/unidoc/pdf/internal/jbig2/reader"
	"github.com/unidoc/unidoc/pdf/internal/jbig2/segments"
	"io"
	"runtime/debug"
)

var (
	/** ID string in file header, see ISO/IEC 14492:2001, D.4.1 */
	fileHeaderID = []byte{0x97, 0x4A, 0x42, 0x32, 0x0D, 0x0A, 0x1A, 0x0A}
)

// Document is the structure of jbig2 document with it's pages  and global segments
type Document struct {
	// Pages contains all pages of this document
	Pages map[int]*Page

	// AmountOfPagesUnknown defines if the ammount of the pages is knownw
	AmountOfPagesUnknown bool

	// AmountOfPages - D.4.3 - Number of pages field (4 bytes). Only presented if
	// AmountOfPagesUnknown is true
	AmountOfPages uint32

	// GBUseExtTemplate defines wether extended Template is used
	GBUseExtTemplate bool

	// SubInputStream is the source data stream wrapped into a SubInputStream
	InputStream *reader.Reader

	// GlobalSegments contains all segments that aren't associated with a page
	GlobalSegments Globals

	// OrganisationType
	OrgainsationType uint8

	fileHeaderLength uint8
}

// NewDocument creates new jbig2.Document for the provided reader
func NewDocument(data []byte) (*Document, error) {
	return NewDocumentWithGlobals(data, nil)
}

// NewDocumentWithGlobals creates new jbig2.Document
func NewDocumentWithGlobals(data []byte, globals Globals) (d *Document, err error) {
	defer func() {
		if x := recover(); x != nil {
			switch e := x.(type) {
			case error:
				err = e
			default:
				err = fmt.Errorf("JBIG2 Internal Error: %v. Trace: %s", e, string(debug.Stack()))
			}
		}
	}()
	d = &Document{
		Pages:                make(map[int]*Page),
		InputStream:          reader.New(data),
		OrgainsationType:     segments.OSequential,
		AmountOfPagesUnknown: true,
		GlobalSegments:       globals,
		fileHeaderLength:     9,
	}

	if d.GlobalSegments == nil {
		d.GlobalSegments = Globals(make(map[int]*segments.Header))
	}

	// mapData map the data stream
	if err = d.mapData(); err != nil {
		return
	}

	return
}

// GetGlobalSegment gets the global segment
func (d *Document) GetGlobalSegment(i int) *segments.Header {
	if d.GlobalSegments == nil {
		common.Log.Debug("Trying to get Global segment from nil Globals")
		return nil
	}
	return d.GlobalSegments[i]
}

// MapData maps the data and stores all segments
func (d *Document) mapData() error {
	// Get the header list
	var segmentHeaders []*segments.Header

	var (
		offset int64
		Type   segments.Type
	)

	isFileHeaderPresent, err := d.isFileHeaderPresent()
	if err != nil {
		return err
	}

	// Parse the file header if there is one
	if isFileHeaderPresent {
		if err = d.parseFileHeader(); err != nil {
			return err
		}

		offset += int64(d.fileHeaderLength)
	}

	var (
		page       *Page
		segmentNo  int
		reachedEOF bool
	)

	// type 51 is
	for Type != 51 && !reachedEOF {

		segmentNo++

		// get new segment
		segment, err := segments.NewHeader(d, d.InputStream, offset, d.OrgainsationType)
		if err != nil {
			return err
		}

		common.Log.Debug("Decoding segment number: %d, Type: %s", segmentNo, segment.Type)
		Type = segment.Type

		if segment.PageAssociation != 0 {
			page = d.Pages[segment.PageAssociation]

			if page == nil {
				page = NewPage(d, segment.PageAssociation)
				d.Pages[segment.PageAssociation] = page
			}

			page.Segments[int(segment.SegmentNumber)] = segment
		} else {
			d.GlobalSegments.AddSegment(int(segment.SegmentNumber), segment)
		}

		segmentHeaders = append(segmentHeaders, segment)

		offset = d.InputStream.StreamPosition()

		if d.OrgainsationType == segments.OSequential {
			offset += int64(segment.SegmentDataLength)
		}

		reachedEOF, err = d.reachedEOF(offset)
		if err != nil {
			common.Log.Debug("reachedEOF error: %v", err)
			return err
		}

	}
	d.determineRandomDataOffsets(segmentHeaders, uint64(offset))

	return nil

}

func (d *Document) isFileHeaderPresent() (bool, error) {
	d.InputStream.Mark()

	for _, magicByte := range fileHeaderID {
		b, err := d.InputStream.ReadByte()
		if err != nil {
			return false, err
		}

		if magicByte != b {
			d.InputStream.Reset()
			return false, nil
		}
	}

	d.InputStream.Reset()
	return true, nil
}

func (d *Document) determineRandomDataOffsets(segmentHeaders []*segments.Header, offset uint64) {

	if d.OrgainsationType == segments.ORandom {

		for _, s := range segmentHeaders {
			s.SegmentDataStartOffset = offset
			offset += s.SegmentDataLength
		}
	}
}

// parseFileHeader - this method reads the stream and sets variables for information about
// organization type and length etc.
func (d *Document) parseFileHeader() error {
	// D.4.1 ID string read will be skipped
	_, err := d.InputStream.Seek(8, io.SeekStart)
	if err != nil {
		return err
	}

	// D.4.2 Header flag (1 byte)

	// Bit 3-7 are reserverd and must be 0
	_, err = d.InputStream.ReadBits(5)
	if err != nil {
		return err
	}

	// Bit 2 - extended templates are used
	b, err := d.InputStream.ReadBit()
	if err != nil {
		return err
	}
	if b == 1 {
		d.GBUseExtTemplate = true
	}

	// Bit 1 - Indicates if amount of pages are unknown
	b, err = d.InputStream.ReadBit()
	if err != nil {
		return err
	}
	if b != 1 {
		d.AmountOfPagesUnknown = false
	}

	// Bit 0 - Indicates file organisation type
	b, err = d.InputStream.ReadBit()
	if err != nil {
		return err
	}
	d.OrgainsationType = uint8(b)

	// D.4.3 Number of pages
	if !d.AmountOfPagesUnknown {
		d.AmountOfPages, err = d.InputStream.ReadUnsignedInt()
		if err != nil {
			return err
		}
		d.fileHeaderLength = 13
	}

	return nil
}

// GetPage gets the gage for the provided 'pageNumber'
func (d *Document) GetPage(pageNumber int) (segments.Pager, error) {
	p, ok := d.Pages[pageNumber]
	if !ok {
		common.Log.Debug("Can't find the page: %d. %s", pageNumber, debug.Stack())
		return nil, errors.New("No page found")
	}

	return p, nil
}

// GetAmountOfPages gets the amount of Pages
func (d *Document) GetAmountOfPages() (uint32, error) {
	if d.AmountOfPagesUnknown || d.AmountOfPages == 0 {
		if len(d.Pages) == 0 {
			d.mapData()
		}

		return uint32(len(d.Pages)), nil
	}
	return d.AmountOfPages, nil
}

func (d *Document) reachedEOF(offset int64) (bool, error) {
	_, err := d.InputStream.Seek(offset, io.SeekStart)
	if err != nil {
		common.Log.Debug("reachedEOF - d.InputStream.Seek failed: %v", err)
		return false, err
	}

	_, err = d.InputStream.ReadBits(32)
	if err == io.EOF {
		return true, nil
	} else if err != nil {
		return false, err
	}

	return false, nil
}
