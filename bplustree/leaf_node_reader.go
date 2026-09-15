package bplustree

import (
	bpm "github.com/Sourav-Nath-01/CambiumDB/bufferpoolmanager"
	codec "github.com/Sourav-Nath-01/CambiumDB/pagecodec"
)

// LeafNodeReader wraps a read guard that controls shared read access to a page containing a leaf node
type LeafNodeReader struct {
	guard *bpm.ReadGuard
	codec codec.LeafNodeCodec
}

func NewLeafNodeReader(rg *bpm.ReadGuard) *LeafNodeReader {

	return &LeafNodeReader{
		guard: rg,
		codec: codec.NewLeafNodeCodec(),
	}
}

// FindValue searches for and returns value corresponding to key
func (r *LeafNodeReader) FindValue(key []byte) (value []byte, found bool) {

	return r.codec.FindValue(r.guard.GetPageData(), key)
}

func (w *LeafNodeReader) PrintElements() {

	w.codec.PrintElements(w.guard.GetPageData())
}
