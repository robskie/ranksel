# ranksel

Package ranksel provides a bit vector that can answer rank and select
queries. More specifically, it implements the data structure described by G.
Navarro and E. Providel's *A Structure for Plain Bitmaps: Combined Sampling* in
[Fast, Small, Simple Rank/Select on Bitmaps](http://dcc.uchile.cl/~gnavarro/ps/sea12.1.pdf)
with some minor modifications.

## Installation
```sh
go get github.com/robskie/ranksel
```

## API Reference

Godoc documentation can be found
[here](https://godoc.org/github.com/robskie/ranksel).

## Benchmarks

All these benchmarks are done on a machine with a Core i5 running at 2.3GHz.
Note that Select0 is slower than Select1 in most cases. This is because the
implementation of Select1 is more optimized than Select0. The only case where
Select0 is faster than Select1 is when the bit density is lower than 3% as shown
from BenchmarkSelectDX where X is the bit density. Another thing to point out is
that Select1 starts to slow down when the bit density gets lower than 3%. So you
might want to use another data structure if you have a spare bitmap and you want
a fast Select1 operation.

You can run these benchmarks by typing
```go test github.com/robskie/ranksel -bench=.*``` from terminal.

```
BenchmarkAdd        50000000     34.1 ns/op
BenchmarkRank1      10000000    160 ns/op
BenchmarkRank0      10000000    170 ns/op
BenchmarkSelect1     5000000    253 ns/op
BenchmarkSelect0     3000000    427 ns/op
BenchmarkSelect1D3   5000000    261 ns/op
BenchmarkSelect0D3  10000000    207 ns/op
BenchmarkSelect1D2   5000000    336 ns/op
BenchmarkSelect0D2  10000000    195 ns/op
BenchmarkSelect1D1   3000000    561 ns/op
BenchmarkSelect0D1  10000000    214 ns/op
```
