// Package ranksel provides a bit vector
// that can answer rank and select queries.
package ranksel

import (
	"unsafe"

	"github.com/robskie/bit"
)

// Options determines the size of the rank and
// select sampling block. Lower values translates
// to faster operations but results in larger size.
type Options struct {
	// Sr is the rank sampling block size.
	// This represents the number of bits in
	// each rank sampling block. Default is 1024.
	Sr int

	// Ss is the select sampling block size.
	// This represents the number of 1s in each
	// select sampling block. Default is 8192.
	Ss int
}

// NewOptions creates an Options
// object with default values.
func NewOptions() *Options {
	return &Options{1024, 8192}
}

// BitVector is a bitmap with added data structure described by G. Navarro and
// E. Providel's `A Structure for Plain Bitmaps: Combined Sampling` in "Fast,
// Small, Simple Rank/Select on Bitmaps" with some minor modifications.
//
// See http://dcc.uchile.cl/~gnavarro/ps/sea12.1.pdf for more details.
type BitVector struct {
	bits *bit.Array

	// ranks[i] is the number of 1s
	// from 0 to index (i*sr)-1
	ranks []int

	// indices[i] points to the
	// beginning of the uint64 (LSB)
	// that contains the (i*ss)+1th
	// set bit.
	indices []int

	popcount int

	opts *Options
}

// NewBitVector creates a new BitVector.
func NewBitVector(opts *Options) *BitVector {
	if opts == nil {
		opts = NewOptions()
	}

	b := bit.NewArray(0)
	rs := make([]int, 1)
	idx := make([]int, 1)

	return &BitVector{
		bits:    b,
		ranks:   rs,
		indices: idx,
		opts:    opts,
	}
}

// Add appends the bits given its size to the vector.
func (v *BitVector) Add(bits uint64, size int) {
	if size <= 0 || size > 64 {
		panic("ranksel: bit size must be in range [1,64]")
	}

	// Add bits
	v.bits.Add(bits, size)
	vlength := v.bits.Len()

	// Increment popcount
	popcnt := bit.PopCount(bits)
	v.popcount += popcnt

	// Update rank sampling
	lenranks := len(v.ranks)
	overflow := vlength - (lenranks * v.opts.Sr)
	if overflow > 0 {
		v.ranks = append(v.ranks, 0)

		rank := bit.Rank(bits, size-overflow-1)
		v.ranks[lenranks] = v.popcount - popcnt + rank
	}

	// Update select sampling
	lenidx := len(v.indices)
	overflow = v.popcount - (lenidx * v.opts.Ss)
	if overflow > 0 {
		v.indices = append(v.indices, 0)

		sel := bit.Select(bits, popcnt-overflow+1)
		v.indices[lenidx] = (vlength - size + sel) & ^0x3F
	}
}

// Get returns the uint64 representation of
// bits starting from index idx given the bit size.
func (v *BitVector) Get(idx, size int) uint64 {
	return v.bits.Get(idx, size)
}

// Bit returns the bit value at index i.
func (v *BitVector) Bit(i int) uint {
	if i >= v.bits.Len() {
		panic("ranksel: index out of range")
	}

	vbits := v.bits.Bits()
	if vbits[i>>6]&(1<<uint(i&63)) != 0 {
		return 1
	}
	return 0
}

// Rank1 counts the number of 1s from
// the beginning up to the ith index.
func (v *BitVector) Rank1(i int) int {
	if i >= v.bits.Len() {
		panic("ranksel: index out of range")
	}

	j := i / v.opts.Sr
	ip := (j * v.opts.Sr) >> 6
	rank := v.ranks[j]

	aidx := i & 63
	bidx := i >> 6
	vbits := v.bits.Bits()
	for _, b := range vbits[ip:bidx] {
		rank += bit.PopCount(b)
	}

	return rank + bit.Rank(vbits[bidx], aidx)
}

// Rank0 counts the number of 0s from
// the beginning up to the ith index.
func (v *BitVector) Rank0(i int) int {
	return i - v.Rank1(i) + 1
}

// Select1 returns the index of the ith set bit.
// Panics if i is zero or greater than the number
// of set bits.
func (v *BitVector) Select1(i int) int {
	if i > v.popcount {
		panic("ranksel: input exceeds number of 1s")
	} else if i == 0 {
		panic("ranksel: input must be greater than 0")
	}

	j := (i - 1) / v.opts.Ss
	q := v.indices[j] / v.opts.Sr

	k := 0
	r := 0
	rq := v.ranks[q:]
	for k, r = range rq {
		if r >= i {
			k--
			break
		}
	}

	idx := 0
	rank := rq[k]
	vbits := v.bits.Bits()
	aidx := ((q + k) * v.opts.Sr) >> 6
	for ii, b := range vbits[aidx:] {
		rank += bit.PopCount(b)

		if rank >= i {
			overflow := rank - i
			popcnt := bit.PopCount(b)

			idx = (aidx + ii) << 6
			idx += bit.Select(b, popcnt-overflow)

			break
		}
	}

	return idx
}

// Select0 returns the index of the ith zero. Panics
// if i is zero or greater than the number of zeroes.
// This is slower than Select1 in most cases.
func (v *BitVector) Select0(i int) int {
	if i > (v.bits.Len() - v.popcount) {
		panic("ranksel: input exceeds number of 0s")
	} else if i == 0 {
		panic("ranksel: input must be greater than 0")
	}

	// Do a binary search on the rank samples to find
	// the largest rank sample that is less than i.
	// From https://en.wikipedia.org/wiki/Binary_search_algorithm
	imin := 1
	imax := len(v.ranks) - 1
	for imin < imax {
		imid := imin + ((imax - imin) >> 1)

		rmid0 := (imid * v.opts.Sr) - v.ranks[imid]
		if rmid0 < i {
			imin = imid + 1
		} else {
			imax = imid
		}
	}
	imin--

	idx := 0
	vbits := v.bits.Bits()
	aidx := (imin * v.opts.Sr) >> 6
	rank0 := (imin * v.opts.Sr) - v.ranks[imin]
	for ii, b := range vbits[aidx:] {
		b = ^b
		rank0 += bit.PopCount(b)

		if rank0 >= i {
			overflow := rank0 - i
			popcnt := bit.PopCount(b)

			idx = (aidx + ii) << 6
			idx += bit.Select(b, popcnt-overflow)

			break
		}
	}

	return idx
}

// Len returns the number of bits stored.
func (v *BitVector) Len() int {
	return v.bits.Len()
}

// PopCount returns the total number of 1s.
func (v *BitVector) PopCount() int {
	return v.popcount
}

// Size returns the vector size in bytes.
func (v *BitVector) Size() int {
	sizeofInt := int(unsafe.Sizeof(int(0)))

	size := v.bits.Size()
	size += len(v.ranks) * sizeofInt
	size += len(v.indices) * sizeofInt

	return size
}

// String returns a hexadecimal
// string representation of the vector.
func (v *BitVector) String() string {
	return v.bits.String()
}
