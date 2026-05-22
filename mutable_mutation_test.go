package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
)

func TestMutableConfig_Set_YAMLRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		base  string
		path  config.KeyPath
		value any
		want  string
	}{
		{
			name:  "scalar",
			base:  "root:\n  value: old\n",
			path:  config.NewKeyPath("root/value"),
			value: "new",
			want:  "root:\n  value: new\n",
		},
		{
			name:  "map",
			base:  "root:\n  existing: 1\n",
			path:  config.NewKeyPath("root/added"),
			value: map[string]any{"zebra": "last", "alpha": "first"},
			want:  "root:\n  existing: 1\n  added:\n    alpha: first\n    zebra: last\n",
		},
		{
			name: "slice",
			base: "root:\n  existing: 1\n",
			path: config.NewKeyPath("root/listen"),
			value: []any{
				map[string]any{"uri": "127.0.0.1:3303"},
			},
			want: "root:\n  existing: 1\n  listen:\n    - uri: 127.0.0.1:3303\n",
		},
		{
			name:  "overwrite existing subtree",
			base:  "a:\n  b:\n    - x: 1\n",
			path:  config.NewKeyPath("a/b"),
			value: map[string]any{"new": "val"},
			want:  "a:\n  b:\n    new: val\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := buildFromYAML(t, tt.base)

			require.NoError(t, cfg.Set(tt.path, tt.value))
			requireMutableYAMLRoundTrip(t, cfg, tt.want)
		})
	}
}

func TestMutableConfig_Merge_YAMLRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		base  string
		other string
		want  string
	}{
		{
			name:  "existing list",
			base:  "root:\n  added:\n    listen:\n      - uri: old\n",
			other: "root:\n  added:\n    listen:\n      - uri: 127.0.0.1:3303\n",
			want:  "root:\n  added:\n    listen:\n      - uri: 127.0.0.1:3303\n",
		},
		{
			name:  "fresh list",
			base:  "root:\n  existing: 1\n",
			other: "root:\n  added:\n    listen:\n      - uri: 127.0.0.1:3303\n",
			want:  "root:\n  existing: 1\n  added:\n    listen:\n      - uri: 127.0.0.1:3303\n",
		},
		{
			name: "nested map and list",
			base: "root:\n  existing: 1\n",
			other: "root:\n  added:\n    groups:\n      storage:\n        listen:\n" +
				"          - uri: 127.0.0.1:3303\n",
			want: "root:\n  existing: 1\n  added:\n    groups:\n      storage:\n        listen:\n" +
				"          - uri: 127.0.0.1:3303\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := buildFromYAML(t, tt.base)
			other := buildFromYAML(t, tt.other)
			otherSnapshot := other.Snapshot()

			require.NoError(t, cfg.Merge(&otherSnapshot))
			requireMutableYAMLRoundTrip(t, cfg, tt.want)
		})
	}
}

func requireMutableYAMLRoundTrip(t *testing.T, cfg *config.MutableConfig, want string) {
	t.Helper()

	snapshot := cfg.Snapshot()
	out, err := snapshot.MarshalYAML()
	require.NoError(t, err)
	require.Equal(t, want, string(out))

	roundTripped := buildFromYAML(t, string(out))
	roundTripSnapshot := roundTripped.Snapshot()
	roundTripOut, err := roundTripSnapshot.MarshalYAML()
	require.NoError(t, err)
	require.Equal(t, want, string(roundTripOut))
}
