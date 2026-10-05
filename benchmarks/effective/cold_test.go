package main_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	config "github.com/tarantool/go-config/v2"
	"github.com/tarantool/go-config/v2/tarantool"
)

func BenchmarkPR102ColdEffective(b *testing.B) {
	for _, instances := range []int{1, 64, 512} {
		b.Run(strconv.Itoa(instances), func(b *testing.B) {
			var input strings.Builder
			input.WriteString("console:\n  socket: var/run/control\nlog:\n  file: var/log/server.log\n")
			input.WriteString("groups:\n  g:\n    replicasets:\n      r:\n        instances:\n")

			for i := range instances {
				fmt.Fprintf(&input, "          i-%d:\n            app:\n              cfg:\n", i)

				for j := range 8 {
					fmt.Fprintf(&input, "                key-%d: instance-%d-value-%d\n", j, i, j)
				}
			}

			filename := filepath.Join(b.TempDir(), "config.yaml")

			err := os.WriteFile(filename, []byte(input.String()), 0600)
			if err != nil {
				b.Fatal(err)
			}

			path := config.NewKeyPath("groups/g/replicasets/r/instances/i-0")

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				b.StopTimer()

				cfg, err := tarantool.New().WithoutSchema().WithEnvPrefix("PR102_REVIEW_").
					WithConfigFile(filename).Build(b.Context())
				if err != nil {
					b.Fatal(err)
				}

				b.StartTimer()

				effective, err := cfg.Effective(path)

				b.StopTimer()

				if err != nil {
					b.Fatal(err)
				}

				var value string

				_, err = effective.Get(config.NewKeyPath("app/cfg/key-7"), &value)
				if err != nil {
					b.Fatal(err)
				}

				if value != "instance-0-value-7" {
					b.Fatalf("unexpected effective value: %q", value)
				}
			}
		})
	}
}
