// Package imohash implements a fast, constant-time hash for files. It is based atop
// murmurhash3 and uses file size and sample data to construct the hash.
//
// For more information, including important caveats on usage, consult https://github.com/kalafut/imohash.
package imohash

import (
	"bytes"
	"encoding/binary"
	"hash"
	"io"
	"os"

	"github.com/spaolacci/murmur3"
)

const Size = 16

// Files smaller than 128kb will be hashed in their entirety.
const SampleThreshhold = 128 * 1024
const SampleSize = 16 * 1024

var emptyArray = [Size]byte{}

// Make sure interfaces are correctly implemented.
var (
	_ hash.Hash = new(ImoHash)
)

type ImoHash struct {
	hasher          murmur3.Hash128
	sampleSize      int
	sampleThreshold int
	bytesAdded      int
}

// New returns a new ImoHash using the default sample size
// and sample threshhold values.
func New() ImoHash {
	return NewCustom(SampleSize, SampleThreshhold)
}

// NewCustom returns a new ImoHash using the provided sample size
// and sample threshhold values. The entire file will be hashed
// (i.e. no sampling), if sampleSize < 1.
func NewCustom(sampleSize, sampleThreshold int) ImoHash {
	h := ImoHash{
		hasher:          murmur3.New128(),
		sampleSize:      sampleSize,
		sampleThreshold: sampleThreshold,
	}

	return h
}

// SumFile hashes a file using default sample parameters.
func SumFile(filename string) ([Size]byte, error) {
	imo := New()
	return imo.SumFile(filename)
}

// Sum128 hashes a byte slice using default sample parameters.
func Sum128(data []byte) [Size]byte {
	imo := New()
	sr := io.NewSectionReader(bytes.NewReader(data), 0, int64(len(data)))

	return imo.hashCore(sr)
}

// SumFile hashes a file using using the ImoHash parameters.
func (imo *ImoHash) SumFile(filename string) ([Size]byte, error) {
	f, err := os.Open(filename)
	defer f.Close()

	if err != nil {
		return emptyArray, err
	}

	fi, err := f.Stat()
	if err != nil {
		return emptyArray, err
	}
	sr := io.NewSectionReader(f, 0, fi.Size())
	return imo.hashCore(sr), nil
}

// Sum appends the current hash to data and returns the resulting slice.
// It does not change the underlying hash state.
func (imo *ImoHash) Sum(data []byte) []byte {
	hash := imo.hasher.Sum(nil)
	binary.PutUvarint(hash, uint64(imo.bytesAdded))
	return append(data, hash...)
}

// Write (via the embedded io.Writer interface) adds more data to the running hash.
func (imo *ImoHash) Write(data []byte) (n int, err error) {
	imo.hasher.Write(data)
	imo.bytesAdded += len(data)
	return len(data), nil
}

func (imo *ImoHash) BlockSize() int { return 1 }

// Reset resets the Hash to its initial state.
func (imo *ImoHash) Reset() {
	imo.bytesAdded = 0
	imo.hasher.Reset()
}

// Size returns the number of bytes Sum will return.
func (imo *ImoHash) Size() int { return Size }

// hashCore hashes a SectionReader using the ImoHash parameters.
func (imo *ImoHash) hashCore(f *io.SectionReader) [Size]byte {
	var result [Size]byte

	imo.hasher.Reset()

	if f.Size() < int64(imo.sampleThreshold) || imo.sampleSize < 1 {
		buffer := make([]byte, f.Size())
		f.Read(buffer)
		imo.hasher.Write(buffer)
	} else {
		buffer := make([]byte, imo.sampleSize)
		f.Read(buffer)
		imo.hasher.Write(buffer)
		f.Seek(f.Size()/2, 0)
		f.Read(buffer)
		imo.hasher.Write(buffer)
		f.Seek(int64(-imo.sampleSize), 2)
		f.Read(buffer)
		imo.hasher.Write(buffer)
	}

	hash := imo.hasher.Sum(nil)

	binary.PutUvarint(hash, uint64(f.Size()))
	copy(result[:], hash)

	return result
}
