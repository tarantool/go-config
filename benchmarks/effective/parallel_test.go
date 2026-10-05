package main

import (
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"

	config "github.com/tarantool/go-config/v2"
	"github.com/tarantool/go-config/v2/tarantool"
)

func BenchmarkPR102Parallel(b *testing.B) {
	const instances = 10000

	for _, mode := range []string{"none", "shared", "local"} {
		b.Run(mode, func(b *testing.B) {
			filename := filepath.Join(b.TempDir(), "config.yaml")

			_, err := fixture(filename, instances, mode)
			if err != nil {
				b.Fatal(err)
			}

			cfg, err := tarantool.New().WithoutSchema().WithEnvPrefix("PR102_PERF_").
				WithConfigFile(filename).Build(b.Context())
			if err != nil {
				b.Fatal(err)
			}

			views, err := cfg.EffectiveAll()
			if err != nil || len(views) != instances {
				b.Fatalf("warmup: got %d views, error %v", len(views), err)
			}

			paths := make([]config.KeyPath, instances)
			for i := range paths {
				paths[i] = config.NewKeyPath(fmt.Sprintf("groups/g/replicasets/r/instances/i-%d", i))
			}

			var worker atomic.Uint64

			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				instance := int(worker.Add(1) % instances)

				var view config.Config

				ran := false
				for pb.Next() {
					ran = true

					var err error

					view, err = cfg.Effective(paths[instance])
					if err != nil {
						b.Error(err)
						return
					}
				}

				if !ran {
					return
				}

				err := check(&view, instance, mode)
				if err != nil {
					b.Error(err)
				}
			})
		})
	}
}
