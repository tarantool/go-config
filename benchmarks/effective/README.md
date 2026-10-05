# Effective configuration benchmarks

Reproduce the timing, allocation and retained-heap measurements used for
[PR #102](https://github.com/tarantool/go-config/pull/102). The harness uses only
public APIs, so it can also run against the original branch. Copy this directory
into that checkout or cherry-pick the benchmark-only commit.

Run commands from the repository root. Use the same Go toolchain, CPU count and
environment for both versions; the original measurements used Go 1.26.5. Run
versions sequentially, alternating their order, without `-race` or competing jobs.

## Timing and allocations

`BenchmarkPR102Perf` builds a YAML configuration with 10,000 instances and eight
local values per instance. Its modes are:

- `none`: literal values, no placeholders.
- `shared`: one global `console.socket` template inherited by every instance.
- `local`: eight `instance_name` templates per instance.

`ColdOne` and `ColdAll` measure the first `Effective` or `EffectiveAll` on a fresh
configuration. `WarmOne` and `WarmAll` reuse a warmed configuration. File creation,
`Build`, explicit GC and result checks are outside the timer. Schema validation is
disabled, and distinct environment prefixes avoid ordinary `TT_*` variables.

Use fixed iteration counts: automatic calibration of a cold benchmark can spend
a long time rebuilding configurations outside the measured interval.

```sh
export GOTOOLCHAIN=go1.26.5
export GOFLAGS=-mod=readonly

go test ./benchmarks/effective -run '^$' \
  -bench '^BenchmarkPR102Perf$/.*$/^Cold(One|All)$' \
  -benchtime 1x -count 6 -benchmem

go test ./benchmarks/effective -run '^$' \
  -bench '^BenchmarkPR102Perf$/.*$/^WarmOne$' \
  -benchtime 20000x -count 6 -benchmem

go test ./benchmarks/effective -run '^$' \
  -bench '^BenchmarkPR102Perf$/.*$/^WarmAll$' \
  -benchtime 10x -count 6 -benchmem

go test ./benchmarks/effective -run '^$' \
  -bench '^BenchmarkPR102Parallel$' \
  -benchtime 100000x -count 3 -benchmem
```

`BenchmarkPR102Parallel` warms all instances before concurrent `Effective` calls,
with a fixed instance per worker. Each active worker checks its last result;
these checks are included in the aggregate parallel timing.

The initial small-config comparison is also included. It measures a cold
`Effective` for 1, 64 and 512 instances without placeholders. Unlike the large
fixture, it does not explicitly force GC between builds:

```sh
go test ./benchmarks/effective -run '^$' \
  -bench '^BenchmarkPR102ColdEffective$' -benchtime 1x -count 6 -benchmem
```

## Retained heap

The standalone command reports JSON snapshots after two GCs at each stage:
`built`, `one_effective`, `all_effective`, `views_released`, `config_released`.
It validates every effective instance before releasing views. Run each mode in a
fresh process; repeat twice to match the original retained-heap measurements.

```sh
go run ./benchmarks/effective -instances 10000 -mode none
go run ./benchmarks/effective -instances 10000 -mode shared
go run ./benchmarks/effective -instances 10000 -mode local
```

- `HeapBytes`: total live heap at the snapshot.
- `AddedBytes`: live heap above the baseline taken after fixture generation.
- `AllocBytes`: cumulative allocations since the previous snapshot, including
  result checks and measurement bookkeeping; not comparable directly to `B/op`.
- `Milliseconds`: stage time before snapshot GC, including result checks. Use
  the benchmarks above for operation timings.

The retained index estimate is `AddedBytes(views_released) - AddedBytes(built)`.
It is part of the configuration's heap, not an additional amount to add to it.
