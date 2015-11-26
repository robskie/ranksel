package ranksel

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBit(t *testing.T) {
	vec := NewBitVector(nil)
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
	vec := NewBitVector(nil)
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
	const sr = 1024

	vec := NewBitVector(nil)
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
	vec := NewBitVector(nil)
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
	const sr = 1024

	vec := NewBitVector(nil)
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
	const sr = 1024

	vec := NewBitVector(nil)
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
	vec := NewBitVector(nil)
	for i := 0; i < 1e6; i++ {
		vec.Add(^uint64(0), 64)
	}

	rawsize := float64(vec.bits.Size())
	overhead := float64(vec.Size()) - rawsize
	percentage := (overhead / rawsize) * 100

	fmt.Printf("=== OVERHEAD: %.2f%%\n", percentage)
}

var bigVector *BitVector

func initBigVector() {
	if bigVector == nil {
		size := 1 << 28
		bigVector = NewBitVector(nil)
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
	vec := NewBitVector(nil)
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
	vec := NewBitVector(nil)
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
	vec := NewBitVector(nil)
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
	vec := NewBitVector(nil)
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
	vec := NewBitVector(nil)
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
	vec := NewBitVector(nil)
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
