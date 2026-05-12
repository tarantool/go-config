package collectors_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
	"github.com/tarantool/go-config/internal/testutil"
)

type structLog struct {
	Level string `config:"level"`
	File  string `yaml:"file"`
}

type structServer struct {
	Port    int      `config:"port"`
	Hosts   []string `config:"hosts"`
	Skipped string   `config:"-"`
	private string   //nolint:unused
}

type structConfig struct {
	Listen string               `config:"listen"`
	Log    structLog            `config:"log"`
	Server structServer         `config:"server"`
	Extra  map[string]any       `config:"extra"`
	Empty  string               `config:"empty,omitempty"`
	Tags   map[int]string       `config:"tags"`
	Nested map[string]structLog `config:"nested"`
}

func TestNewStruct(t *testing.T) {
	t.Parallel()

	coll := collectors.NewStruct(structConfig{})
	require.NotNil(t, coll)
	assert.Equal(t, "struct", coll.Name())
	assert.Equal(t, config.UnknownSource, coll.Source())
	assert.Equal(t, config.RevisionType(""), coll.Revision())
	assert.True(t, coll.KeepOrder())
}

func TestStruct_Builders(t *testing.T) {
	t.Parallel()

	coll := collectors.NewStruct(structConfig{}).
		WithName("custom").
		WithSourceType(config.FileSource).
		WithRevision("v1").
		WithKeepOrder(false)

	assert.Equal(t, "custom", coll.Name())
	assert.Equal(t, config.FileSource, coll.Source())
	assert.Equal(t, config.RevisionType("v1"), coll.Revision())
	assert.False(t, coll.KeepOrder())
}

func TestStruct_Read(t *testing.T) {
	t.Parallel()

	data := structConfig{
		Listen: "127.0.0.1:3301",
		Log:    structLog{Level: "info", File: "/var/log/app.log"},
		Server: structServer{Port: 8080, Hosts: []string{"a", "b"}, Skipped: "nope"},
	}

	coll := collectors.NewStruct(&data)
	got := collectValues(t, coll.Read(context.Background()))

	assert.Equal(t, "127.0.0.1:3301", got["listen"])
	assert.Equal(t, "info", got["log/level"])
	assert.Equal(t, "/var/log/app.log", got["log/file"])
	assert.EqualValues(t, 8080, got["server/port"])
	assert.NotContains(t, got, "server/skipped")
	assert.NotContains(t, got, "server/private")
}

func TestStruct_Read_NotAStruct(t *testing.T) {
	t.Parallel()

	coll := collectors.NewStruct(42)
	testutil.Drain(t, coll.Read(context.Background()))
}

func TestStruct_Read_Cancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	coll := collectors.NewStruct(structServer{Port: 1, Hosts: []string{"x"}})
	valueCh := coll.Read(ctx)

	_, ok := <-valueCh
	require.True(t, ok)

	cancel()
	testutil.Drain(t, valueCh)
}

func TestStructToMap(t *testing.T) {
	t.Parallel()

	data := structConfig{
		Listen: "addr",
		Log:    structLog{Level: "debug", File: "f"},
		Server: structServer{Port: 9, Hosts: []string{"h1"}},
		Extra:  map[string]any{"k": "v"},
		Tags:   map[int]string{1: "one"},
		Nested: map[string]structLog{"x": {Level: "warn"}},
	}

	got, err := collectors.StructToMap(data)
	require.NoError(t, err)

	assert.Equal(t, "addr", got["listen"])
	assert.Equal(t, map[string]any{"level": "debug", "file": "f"}, got["log"])
	assert.Equal(t, map[string]any{"port": 9, "hosts": []any{"h1"}}, got["server"])
	assert.Equal(t, map[string]any{"k": "v"}, got["extra"])
	assert.Equal(t, map[string]any{"1": "one"}, got["tags"])
	assert.Equal(t, map[string]any{"x": map[string]any{"level": "warn", "file": ""}}, got["nested"])
	assert.NotContains(t, got, "empty") // omitempty drops the zero value.
}

func TestStructToMap_PointerAndNotStruct(t *testing.T) {
	t.Parallel()

	got, err := collectors.StructToMap(&structLog{Level: "err"})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"level": "err", "file": ""}, got)

	_, err = collectors.StructToMap("nope")
	require.ErrorIs(t, err, collectors.ErrNotStruct)

	_, err = collectors.StructToMap((*structLog)(nil))
	require.ErrorIs(t, err, collectors.ErrNotStruct)
}

type inlineBase struct {
	ID int `config:"id"`
}

type withInline struct {
	inlineBase `config:",inline"`

	Name    string         `config:"name"`
	Meta    map[string]any `config:"meta,inline"`
	Wrapped inlineBase     `config:"wrapped"`
}

func TestStructToMap_Inline(t *testing.T) {
	t.Parallel()

	got, err := collectors.StructToMap(withInline{
		inlineBase: inlineBase{ID: 7},
		Name:       "n",
		Meta:       map[string]any{"region": "eu"},
		Wrapped:    inlineBase{ID: 8},
	})
	require.NoError(t, err)

	assert.Equal(t, 7, got["id"])
	assert.Equal(t, "n", got["name"])
	assert.Equal(t, "eu", got["region"])
	assert.Equal(t, map[string]any{"id": 8}, got["wrapped"])
}

type anonInner struct {
	A int `config:"a"`
}

type anonOuter struct {
	anonInner

	B int `config:"b"`
}

func TestStructToMap_AnonymousNoInline(t *testing.T) {
	t.Parallel()

	got, err := collectors.StructToMap(anonOuter{anonInner: anonInner{A: 1}, B: 2})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"anoninner": map[string]any{"a": 1}, "b": 2}, got)
}

func TestStructToMap_Bytes(t *testing.T) {
	t.Parallel()

	type withBytes struct {
		Raw []byte `config:"raw"`
	}

	got, err := collectors.StructToMap(withBytes{Raw: []byte("hi")})
	require.NoError(t, err)
	assert.Equal(t, []byte("hi"), got["raw"])
}

// collectValues drains a value channel into a map keyed by the slash-joined
// key path.
func collectValues(t *testing.T, valueCh <-chan config.Value) map[string]any {
	t.Helper()

	out := map[string]any{}

	for value := range valueCh {
		var dest any

		require.NoError(t, value.Get(&dest))

		out[value.Meta().Key.String()] = dest
	}

	return out
}
