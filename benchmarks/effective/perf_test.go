package main

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	config "github.com/tarantool/go-config/v2"
	"github.com/tarantool/go-config/v2/tarantool"
)

func BenchmarkPR102Perf(b *testing.B) {
	const instances = 10000

	for _, mode := range []string{"none", "shared", "local"} {
		b.Run(mode, func(b *testing.B) {
			filename := filepath.Join(b.TempDir(), "config.yaml")

			_, err := fixture(filename, instances, mode)
			if err != nil {
				b.Fatal(err)
			}

			for _, operation := range []string{"ColdOne", "WarmOne", "ColdAll", "WarmAll"} {
				b.Run(operation, func(b *testing.B) {
					b.StopTimer()

					build := func() config.Config {
						cfg, err := tarantool.New().WithoutSchema().WithEnvPrefix("PR102_PERF_").
							WithConfigFile(filename).Build(b.Context())
						if err != nil {
							b.Fatal(err)
						}

						return cfg
					}
					path := config.NewKeyPath("groups/g/replicasets/r/instances/i-0")
					cold := operation == "ColdOne" || operation == "ColdAll"
					all := operation == "ColdAll" || operation == "WarmAll"

					var (
						cfg, one config.Config
						views    map[string]config.Config
					)

					invoke := func() {
						var err error
						if all {
							views, err = cfg.EffectiveAll()
						} else {
							one, err = cfg.Effective(path)
						}

						if err != nil {
							b.Fatal(err)
						}
					}

					if !cold {
						cfg = build()

						invoke()

						one = config.Config{}
						views = nil

						runtime.GC()
					}

					b.ReportAllocs()
					b.ResetTimer()

					for i := range b.N {
						if cold {
							cfg = build()

							runtime.GC()
							b.StartTimer()
						} else if i == 0 {
							b.StartTimer()
						}

						invoke()

						if cold {
							b.StopTimer()
						}
					}

					b.StopTimer()

					if !all {
						err := check(&one, 0, mode)
						if err != nil {
							b.Fatal(err)
						}

						return
					}

					if len(views) != instances {
						b.Fatalf("got %d instances, want %d", len(views), instances)
					}

					for _, i := range []int{0, instances - 1} {
						view, ok := views[fmt.Sprintf("groups/g/replicasets/r/instances/i-%d", i)]
						if !ok {
							b.Fatalf("missing instance %d", i)
						}

						err := check(&view, i, mode)
						if err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		})
	}
}
