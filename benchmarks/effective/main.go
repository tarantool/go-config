// Command effective measures retained heap across configuration lifetimes.
//
//nolint:err113 // Validation errors are printed by this executable, not classified by callers.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	config "github.com/tarantool/go-config/v2"
	"github.com/tarantool/go-config/v2/tarantool"
)

const (
	modeNone         = "none"
	modeShared       = "shared"
	modeLocal        = "local"
	defaultInstances = 10000
	fixtureMode      = 0o600
)

//nolint:gochecknoglobals // Explicit GC roots keep measured objects alive until each release stage.
var (
	liveConfig config.Config
	liveOne    config.Config
	liveAll    map[string]config.Config
)

//nolint:tagliatelle // Snapshot JSON uses Go field names.
type measurement struct {
	Stage        string  `json:"Stage"`
	HeapBytes    uint64  `json:"HeapBytes"`
	AddedBytes   int64   `json:"AddedBytes"`
	AllocBytes   uint64  `json:"AllocBytes"`
	Milliseconds float64 `json:"Milliseconds"`
}

func snapshot() runtime.MemStats {
	runtime.GC()
	runtime.GC()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return m
}

func fixture(filename string, count int, mode string) (int, error) {
	var input strings.Builder

	socket := "var/run/control"
	if mode == modeShared {
		socket = "var/run/{{ instance_name }}/control"
	}

	fmt.Fprintf(&input, "console:\n  socket: '%s'\nlog:\n  file: var/log/server.log\n", socket)
	input.WriteString("groups:\n  g:\n    replicasets:\n      r:\n        instances:\n")

	for i := range count {
		fmt.Fprintf(&input, "          i-%d:\n            app:\n              cfg:\n", i)

		for keyIndex := range 8 {
			value := fmt.Sprintf("instance-%d-value-%d", i, keyIndex)
			if mode == modeLocal {
				value = fmt.Sprintf("{{ instance_name }}-value-%d", keyIndex)
			}

			fmt.Fprintf(&input, "                key-%d: '%s'\n", keyIndex, value)
		}
	}

	err := os.WriteFile(filename, []byte(input.String()), fixtureMode)
	if err != nil {
		return 0, fmt.Errorf("write fixture: %w", err)
	}

	return input.Len(), nil
}

func check(cfg *config.Config, instance int, mode string) error {
	var value string

	_, err := cfg.Get(config.NewKeyPath("app/cfg/key-7"), &value)
	if err != nil {
		return fmt.Errorf("get app value: %w", err)
	}

	want := fmt.Sprintf("instance-%d-value-7", instance)
	if mode == modeLocal {
		want = fmt.Sprintf("i-%d-value-7", instance)
	}

	if value != want {
		return fmt.Errorf("app/cfg/key-7: got %q, want %q", value, want)
	}

	_, err = cfg.Get(config.NewKeyPath("console/socket"), &value)
	if err != nil {
		return fmt.Errorf("get console socket: %w", err)
	}

	want = "var/run/control"
	if mode == modeShared {
		want = fmt.Sprintf("var/run/i-%d/control", instance)
	}

	if value != want {
		return fmt.Errorf("console/socket: got %q, want %q", value, want)
	}

	return nil
}

func run(count int, mode string) error {
	dir, err := os.MkdirTemp("", "pr-102-memory-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}

	defer func() {
		err := os.RemoveAll(dir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()

	size, err := fixture(filepath.Join(dir, "config.yaml"), count, mode)
	if err != nil {
		return err
	}

	base := snapshot()
	previous := base

	var rows []measurement

	record := func(stage string, elapsed time.Duration) {
		m := snapshot()

		rows = append(rows, measurement{
			Stage: stage, HeapBytes: m.HeapAlloc,
			AddedBytes:   int64(m.HeapAlloc) - int64(base.HeapAlloc), //nolint:gosec // Heap fits in int64.
			AllocBytes:   m.TotalAlloc - previous.TotalAlloc,
			Milliseconds: float64(elapsed) / float64(time.Millisecond),
		})
		previous = m
	}
	start := time.Now()

	liveConfig, err = tarantool.New().WithoutSchema().WithEnvPrefix("PR102_MEMORY_").
		WithConfigFile(filepath.Join(dir, "config.yaml")).Build(context.Background())
	if err != nil {
		return fmt.Errorf("build configuration: %w", err)
	}

	record("built", time.Since(start))

	start = time.Now()

	liveOne, err = liveConfig.Effective(config.NewKeyPath("groups/g/replicasets/r/instances/i-0"))
	if err != nil {
		return fmt.Errorf("resolve one instance: %w", err)
	}

	err = check(&liveOne, 0, mode)
	if err != nil {
		return err
	}

	record("one_effective", time.Since(start))

	start = time.Now()

	liveAll, err = liveConfig.EffectiveAll()
	if err != nil {
		return fmt.Errorf("resolve all instances: %w", err)
	}

	if len(liveAll) != count {
		return fmt.Errorf("EffectiveAll returned %d instances, want %d", len(liveAll), count)
	}

	for i := range count {
		cfg, ok := liveAll[fmt.Sprintf("groups/g/replicasets/r/instances/i-%d", i)]
		if !ok {
			return fmt.Errorf("missing effective instance %d", i)
		}

		err := check(&cfg, i, mode)
		if err != nil {
			return err
		}
	}

	record("all_effective", time.Since(start))

	liveAll = nil
	liveOne = config.Config{}

	record("views_released", 0)

	liveConfig = config.Config{}

	record("config_released", 0)

	//nolint:tagliatelle // Snapshot JSON uses Go field names.
	err = json.NewEncoder(os.Stdout).Encode(struct {
		Instances     int           `json:"Instances"`
		Mode          string        `json:"Mode"`
		YAMLBytes     int           `json:"YAMLBytes"`
		BaselineBytes uint64        `json:"BaselineBytes"`
		Rows          []measurement `json:"Rows"`
	}{count, mode, size, base.HeapAlloc, rows})
	if err != nil {
		return fmt.Errorf("encode measurements: %w", err)
	}

	return nil
}

func main() {
	count := flag.Int("instances", defaultInstances, "number of instances")
	mode := flag.String("mode", modeNone, "none, shared, or local templates")

	flag.Parse()

	if *count < 1 || (*mode != modeNone && *mode != modeShared && *mode != modeLocal) {
		fmt.Fprintln(os.Stderr, "invalid count or mode")
		os.Exit(1)
	}

	err := run(*count, *mode)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
