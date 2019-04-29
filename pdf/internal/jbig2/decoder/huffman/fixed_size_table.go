package huffman

import (
	"github.com/unidoc/unidoc/pdf/internal/jbig2/reader"
)

type FixedSizeTable struct {
	rootNode *InternalNode
}

// NewFixedSizeTable creates new fixedSizeTable
func NewFixedSizeTable(codeTable []*Code) (*FixedSizeTable, error) {
	f := &FixedSizeTable{
		rootNode: &InternalNode{},
	}

	if err := f.InitTree(codeTable); err != nil {
		return nil, err
	}

	return f, nil
}

// Decode decodes the fixedSizeTable
func (f *FixedSizeTable) Decode(r reader.StreamReader) (int64, error) {
	return f.rootNode.Decode(r)
}

// InitTree implements HuffmanTabler interface
func (f *FixedSizeTable) InitTree(codeTable []*Code) error {
	preprocessCodes(codeTable)
	for _, c := range codeTable {
		err := f.rootNode.append(c)
		if err != nil {
			return err
		}
	}
	return nil
}

// String implements Stringer interface
func (f *FixedSizeTable) String() string {
	return f.rootNode.String() + "\n"
}

// RootNode returns the root node for the fixedSizeTable
func (f *FixedSizeTable) RootNode() *InternalNode {
	return f.rootNode
}