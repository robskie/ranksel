// Package ranksel provides a bit vector
// that can answer rank and select queries.
package ranksel

import (
	"bytes"
	"fmt"
	"strings"
	"unsafe"

	"github.com/robskie/bit"
)

const (
	// sr is the rank sampling block size.
	// This represents the number of bits in
	// each rank sampling block.
	sr = 1024

	// ss is the number of 1s in each select
	// sampling block. Note that the number of
	// bits in each block varies.
	ss = 8192
)

// BitVector is a bitmap with added data structure described by G. Navarro and
// E. Providel's `A Structure for Plain Bitmaps: Combined Sampling` in "Fast,
// Small, Simple Rank/Select on Bitmaps" with some minor modifications.
//
// See http://dcc.uchile.cl/~gnavarro/ps/sea12.1.pdf for more details.
type BitVector struct {
	bits []uint64

	// ranks[i] is the number of 1s
	// from 0 to index (i*sr)-1
	ranks []int

	// indices[i] points to the
	// beginning of the uint64 (LSB)
	// that contains the (i*ss)+1th
	// set bit.
	indices []int

	length   int
	popcount int
}

// NewBitVector creates a new BitVector
// with an initial bit capacity of n.
func NewBitVector(n int) *BitVector {
	if n < 0 {
		panic("ranksel: vector size must be greater than or equal 0")
	}

	b := make([]uint64, 0, (n>>6)+1)
	rs := make([]int, 1, (n/sr)+1)
	idx := make([]int, 1)

	return &BitVector{
		bits:    b,
		ranks:   rs,
		indices: idx,
	}
}

// Add appends the bits given its size to the vector.
func (v *BitVector) Add(bits uint64, size int) {
	if size <= 0 || size > 64 {
		panic("ranksel: bit size must be in range [1,64]")
	}

	// Extend bits if necessary
	lenbits := len(v.bits)
	freespace := (lenbits << 6) - v.length
	overflow := size - freespace
	if overflow > 0 {
		v.bits = append(v.bits, 0)
	}

	// Append bits
	idx := lenbits - 1
	if freespace > 0 {
		v.bits[idx] |= bits << uint(v.length&63)
	}

	if overflow > 0 {
		v.bits[idx+1] |= bits >> uint(freespace)
	}

	// Increment size and popcount
	v.length += size
	popcnt := bit.PopCount(bits)
	v.popcount += popcnt

	// Update rank sampling
	lenranks := len(v.ranks)
	overflow = v.length - (lenranks * sr)
	if overflow > 0 {
		v.ranks = append(v.ranks, 0)

		rank := rank1_64(bits, size-overflow-1)
		v.ranks[lenranks] = v.popcount - popcnt + rank
	}

	// Update select sampling
	lenidx := len(v.indices)
	overflow = v.popcount - (lenidx * ss)
	if overflow > 0 {
		v.indices = append(v.indices, 0)

		sel := select1_64(bits, popcnt-overflow+1)
		v.indices[lenidx] = (v.length - size + sel) & ^0x3F
	}
}

// Bit returns the bit value at index i.
func (v *BitVector) Bit(i int) uint {
	if i >= v.length {
		panic("ranksel: index out of range")
	}

	if v.bits[i>>6]&(1<<uint(i&63)) != 0 {
		return 1
	}
	return 0
}

// Rank1 counts the number of 1s from
// the beginning up to the ith index.
func (v *BitVector) Rank1(i int) int {
	if i >= v.length {
		panic("ranksel: index out of range")
	}

	j := i / sr
	ip := (j * sr) >> 6
	rank := v.ranks[j]

	aidx := i & 63
	bidx := i >> 6
	for _, b := range v.bits[ip:bidx] {
		rank += bit.PopCount(b)
	}

	return rank + rank1_64(v.bits[bidx], aidx)
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

	j := (i - 1) / ss
	q := v.indices[j] / sr

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
	aidx := ((q + k) * sr) >> 6
	for ii, b := range v.bits[aidx:] {
		rank += bit.PopCount(b)

		if rank >= i {
			overflow := rank - i
			popcnt := bit.PopCount(b)

			idx = (aidx + ii) << 6
			idx += select1_64(b, popcnt-overflow)

			break
		}
	}

	return idx
}

// Select0 returns the index of the ith zero. Panics
// if i is zero or greater than the number of zeroes.
// This is slower than Select1 in most cases.
func (v *BitVector) Select0(i int) int {
	if i > (v.length - v.popcount) {
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

		rmid0 := (imid * sr) - v.ranks[imid]
		if rmid0 < i {
			imin = imid + 1
		} else {
			imax = imid
		}
	}
	imin--

	idx := 0
	aidx := (imin * sr) >> 6
	rank0 := (imin * sr) - v.ranks[imin]
	for ii, b := range v.bits[aidx:] {
		b = ^b
		rank0 += bit.PopCount(b)

		if rank0 >= i {
			overflow := rank0 - i
			popcnt := bit.PopCount(b)

			idx = (aidx + ii) << 6
			idx += select1_64(b, popcnt-overflow)

			break
		}
	}

	return idx
}

// Len returns the number of bits stored.
func (v *BitVector) Len() int {
	return v.length
}

// PopCount returns the total number of 1s.
func (v *BitVector) PopCount() int {
	return v.popcount
}

// Size returns the vector size in bytes.
func (v *BitVector) Size() int {
	sizeofInt := int(unsafe.Sizeof(int(0)))
	size := len(v.bits) * 8
	size += len(v.ranks) * sizeofInt
	size += len(v.indices) * sizeofInt

	return size
}

// String returns a bit string
// representation of the vector.
func (v *BitVector) String() string {
	buf := new(bytes.Buffer)
	for i := len(v.bits) - 1; i >= 0; i-- {
		bits := fmt.Sprintf("%8b", v.bits[i])
		bits = strings.Replace(bits, " ", "0", -1)
		fmt.Fprintf(buf, "%s [%d-%d] ", bits, (i<<3)+7, i<<3)
	}

	return buf.String()
}
