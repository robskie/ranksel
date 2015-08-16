package ranksel

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdd(t *testing.T) {
	vec := NewBitVector(100)

	// Simple case
	vec.Add(0xA, 4)
	vec.Add(0xF, 60)
	assert.Equal(t, 64, vec.Len())
	assert.Equal(t, 1, len(vec.bits))
	assert.EqualValues(t, 0xFA, vec.bits[0])

	// Byte array full then add
	vec.Add(0xE, 4)
	assert.Equal(t, 68, vec.Len())
	assert.Equal(t, 2, len(vec.bits))
	assert.EqualValues(t, 0xE, vec.bits[1])

	// Byte array partially full then add
	vec.Add(0x75<<56, 64)
	assert.Equal(t, 132, vec.Len())
	assert.Equal(t, 3, len(vec.bits))
	assert.EqualValues(t, 0x7, vec.bits[2])
}

func TestBit(t *testing.T) {
	vec := NewBitVector(0)
	vec.Add(0x5555, 16)
	expected := 1
	for i := 0; i < 16; i++ {
		if !assert.EqualValues(t, expected, vec.Bit(i)) {
			break
		}
		expected ^= 0x1
	}
}

func TestRank(t *testing.T) {
	vec := NewBitVector(1e6)
	ranks1 := make([]int, 1e6)
	ranks0 := make([]int, 1e6)

	rank1 := 0
	rank0 := 0
	for i := 0; i < 1e6; i++ {
		bit := rand.Intn(2)
		vec.Add(uint64(bit), 1)

		if bit == 1 {
			rank1++
		} else {
			rank0++
		}

		ranks1[i] = rank1
		ranks0[i] = rank0
	}

	for i, r := range ranks1 {
		if !assert.Equal(t, r, vec.Rank1(i)) {
			break
		}
	}

	for i, r := range ranks0 {
		if !assert.Equal(t, r, vec.Rank0(i)) {
			break
		}
	}
}

// TestRankSparse tests rank queries on sparse
// array (few 1s). A sparse bit array results in
// duplicate values in the rank sampling.
func TestRank1Sparse(t *testing.T) {
	vec := NewBitVector(1e6)
	ranks := make([]int, 1e6)

	popcount := 0
	for i := 0; i < 1e6; i++ {
		v := rand.Intn(sr)

		if v == 1 {
			popcount++
			vec.Add(1, 1)
		} else {
			vec.Add(0, 1)
		}

		ranks[i] = popcount
	}

	for i, r := range ranks {
		if !assert.Equal(t, r, vec.Rank1(i)) {
			break
		}
	}
}

func TestSelect(t *testing.T) {
	vec := NewBitVector(1e6)
	sel1 := []int{}
	sel0 := []int{}

	for i := 0; i < 1e6; i++ {
		bit := rand.Intn(2)
		vec.Add(uint64(bit), 1)

		if bit == 1 {
			sel1 = append(sel1, i)
		} else {
			sel0 = append(sel0, i)
		}
	}

	for i, idx := range sel1 {
		if !assert.Equal(t, idx, vec.Select1(i+1)) {
			break
		}
	}

	for i, idx := range sel0 {
		if !assert.Equal(t, idx, vec.Select0(i+1)) {
			fmt.Println(i+1, idx, vec.Select0(i+1))
			break
		}
	}
}

func TestSelect1Sparse(t *testing.T) {
	vec := NewBitVector(1e6)
	sel1 := []int{}

	for i := 0; i < 1e6; i++ {
		v := rand.Intn(sr)
		if v == 1 {
			vec.Add(1, 1)
			sel1 = append(sel1, i)
		} else {
			vec.Add(0, 1)
		}
	}

	for i, idx := range sel1 {
		if !assert.Equal(t, idx, vec.Select1(i+1)) {
			break
		}
	}
}

func TestSelect0Sparse(t *testing.T) {
	vec := NewBitVector(1e6)
	sel0 := []int{}

	for i := 0; i < 1e6; i++ {
		v := rand.Intn(sr)
		if v == 0 {
			vec.Add(0, 1)
			sel0 = append(sel0, i)
		} else {
			vec.Add(1, 1)
		}
	}

	for i, idx := range sel0 {
		if !assert.Equal(t, idx, vec.Select0(i+1)) {
			break
		}
	}
}

func TestOverhead(t *testing.T) {
	vec := NewBitVector(64 * 1e6)
	for i := 0; i < 1e6; i++ {
		vec.Add(^uint64(0), 64)
	}

	rawsize := float64(len(vec.bits) * 8)
	overhead := float64(vec.Size()) - rawsize
	percentage := (overhead / rawsize) * 100

	fmt.Printf("=== OVERHEAD: %.2f%%\n", percentage)
}

func BenchmarkAdd(b *testing.B) {
	vec := NewBitVector(0)

	// Generate random bit sizes
	sz := make([]int, b.N)
	for i := 0; i < b.N; i++ {
		sz[i] = rand.Intn(64) + 1
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vec.Add(0, sz[i])
	}
}

var bigVector *BitVector

func initBigVector() {
	if bigVector == nil {
		size := 1 << 28
		bigVector = NewBitVector(size)
		for i := 0; i < size/64; i++ {
			bigVector.Add(uint64(rand.Int63()), 64)
		}
	}
}

func BenchmarkRank1(b *testing.B) {
	initBigVector()

	idx := make([]int, b.N)
	for i := range idx {
		idx[i] = rand.Intn(bigVector.Len())
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bigVector.Rank1(idx[i])
	}
}

func BenchmarkRank0(b *testing.B) {
	initBigVector()

	idx := make([]int, b.N)
	for i := range idx {
		idx[i] = rand.Intn(bigVector.Len())
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bigVector.Rank0(idx[i])
	}
}

func BenchmarkSelect1(b *testing.B) {
	initBigVector()

	in := make([]int, b.N)
	for i := range in {
		in[i] = rand.Intn(bigVector.PopCount()) + 1
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bigVector.Select1(in[i])
	}
}

func BenchmarkSelect0(b *testing.B) {
	initBigVector()

	in := make([]int, b.N)
	popcnt0 := bigVector.Len() - bigVector.PopCount()
	for i := range in {
		in[i] = rand.Intn(popcnt0) + 1
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bigVector.Select0(in[i])
	}
}

func BenchmarkSelect1D3(b *testing.B) {
	// Create vector with 3% bit density
	vec := NewBitVector(1e7)
	for i := 0; i < 1e7; i++ {
		v := rand.Intn(33)
		if v == 1 {
			vec.Add(1, 1)
		} else {
			vec.Add(0, 1)
		}
	}

	in := make([]int, b.N)
	for i := range in {
		in[i] = rand.Intn(vec.PopCount()) + 1
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vec.Select1(in[i])
	}
}

func BenchmarkSelect0D3(b *testing.B) {
	// Create vector with 3% bit density
	vec := NewBitVector(1e7)
	for i := 0; i < 1e7; i++ {
		v := rand.Intn(33)
		if v == 1 {
			vec.Add(1, 1)
		} else {
			vec.Add(0, 1)
		}
	}

	in := make([]int, b.N)
	popcnt0 := vec.Len() - vec.PopCount()
	for i := range in {
		in[i] = rand.Intn(popcnt0) + 1
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vec.Select0(in[i])
	}
}

func BenchmarkSelect1D2(b *testing.B) {
	// Create vector with 2% bit density
	vec := NewBitVector(1e7)
	for i := 0; i < 1e7; i++ {
		v := rand.Intn(50)
		if v == 1 {
			vec.Add(1, 1)
		} else {
			vec.Add(0, 1)
		}
	}

	in := make([]int, b.N)
	for i := range in {
		in[i] = rand.Intn(vec.PopCount()) + 1
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vec.Select1(in[i])
	}
}

func BenchmarkSelect0D2(b *testing.B) {
	// Create vector with 2% bit density
	vec := NewBitVector(1e7)
	for i := 0; i < 1e7; i++ {
		v := rand.Intn(50)
		if v == 1 {
			vec.Add(1, 1)
		} else {
			vec.Add(0, 1)
		}
	}

	in := make([]int, b.N)
	popcnt0 := vec.Len() - vec.PopCount()
	for i := range in {
		in[i] = rand.Intn(popcnt0) + 1
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vec.Select0(in[i])
	}
}

func BenchmarkSelect1D1(b *testing.B) {
	// Create vector with 1% bit density
	vec := NewBitVector(1e7)
	for i := 0; i < 1e7; i++ {
		v := rand.Intn(100)
		if v == 1 {
			vec.Add(1, 1)
		} else {
			vec.Add(0, 1)
		}
	}

	in := make([]int, b.N)
	for i := range in {
		in[i] = rand.Intn(vec.PopCount()) + 1
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vec.Select1(in[i])
	}
}

func BenchmarkSelect0D1(b *testing.B) {
	// Create vector with 1% bit density
	vec := NewBitVector(1e7)
	for i := 0; i < 1e7; i++ {
		v := rand.Intn(100)
		if v == 1 {
			vec.Add(1, 1)
		} else {
			vec.Add(0, 1)
		}
	}

	in := make([]int, b.N)
	popcnt0 := vec.Len() - vec.PopCount()
	for i := range in {
		in[i] = rand.Intn(popcnt0) + 1
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vec.Select0(in[i])
	}
}