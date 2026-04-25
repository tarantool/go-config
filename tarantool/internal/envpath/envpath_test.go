package envpath_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/tarantool/internal/envpath"
)

func loadFixtureSchema(t *testing.T) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", "config.schema.json"))
	require.NoError(t, err)

	return data
}

func TestTrie_SimpleKey(t *testing.T) {
	t.Parallel()

	root, err := envpath.Build(loadFixtureSchema(t))
	require.NoError(t, err)

	got := root.Resolve("REPLICATION_FAILOVER")
	assert.Equal(t, config.NewKeyPathFromSegments([]string{"replication", "failover"}), got)
}

func TestTrie_CompoundKey_AuditLog(t *testing.T) {
	t.Parallel()

	root, err := envpath.Build(loadFixtureSchema(t))
	require.NoError(t, err)

	got := root.Resolve("AUDIT_LOG_NONBLOCK")
	assert.Equal(t, config.NewKeyPathFromSegments([]string{"audit_log", "nonblock"}), got)
}

func TestTrie_CompoundKey_LongestWins(t *testing.T) {
	t.Parallel()

	root, err := envpath.Build(loadFixtureSchema(t))
	require.NoError(t, err)

	gotLong := root.Resolve("WAL_QUEUE_MAX_SIZE")
	assert.Equal(t, config.NewKeyPathFromSegments([]string{"wal_queue_max_size"}), gotLong)

	gotShort := root.Resolve("WAL_DIR")
	assert.Equal(t, config.NewKeyPathFromSegments([]string{"wal", "dir"}), gotShort)
}

func TestTrie_Wildcard(t *testing.T) {
	t.Parallel()

	root, err := envpath.Build(loadFixtureSchema(t))
	require.NoError(t, err)

	got := root.Resolve("GROUPS_FOO_REPLICASETS_BAR_INSTANCES_BAZ_IPROTO_LISTEN")
	assert.Equal(t, config.NewKeyPathFromSegments([]string{
		"groups", "foo", "replicasets", "bar", "instances", "baz", "iproto", "listen",
	}), got)
}

func TestTrie_Unknown(t *testing.T) {
	t.Parallel()

	root, err := envpath.Build(loadFixtureSchema(t))
	require.NoError(t, err)

	got := root.Resolve("UNKNOWN_THING")
	assert.Nil(t, got)
}

func TestTrie_EmptyKey(t *testing.T) {
	t.Parallel()

	root, err := envpath.Build(loadFixtureSchema(t))
	require.NoError(t, err)

	got := root.Resolve("")
	assert.Nil(t, got)
}

func TestTrie_MalformedSchema(t *testing.T) {
	t.Parallel()

	_, err := envpath.Build([]byte("not json"))
	require.Error(t, err)
}

func TestTrie_RefCycle(t *testing.T) {
	t.Parallel()

	// $defs.node references itself via a nested property — the walker
	// must not infinite-recurse.
	schema := []byte(`{
		"type": "object",
		"$defs": {
			"node": {
				"properties": {
					"child": { "$ref": "#/$defs/node" },
					"leaf": { "type": "string" }
				}
			}
		},
		"properties": {
			"root": { "$ref": "#/$defs/node" }
		}
	}`)

	// Build must terminate (the cycle was previously a stack overflow
	// at Build time).
	root, err := envpath.Build(schema)
	require.NoError(t, err)

	// Properties siblings of the cyclic ref stay reachable.
	got := root.Resolve("ROOT_LEAF")
	assert.Equal(t, config.NewKeyPathFromSegments([]string{"root", "leaf"}), got)

	// Paths through the cycle are pruned at the first re-entry — the
	// env var is silently dropped rather than expanded indefinitely.
	got = root.Resolve("ROOT_CHILD_LEAF")
	assert.Nil(t, got)
}

func TestTrie_Ref(t *testing.T) {
	t.Parallel()

	schema := []byte(`{
		"type": "object",
		"$defs": {
			"netbox": {
				"type": "object",
				"properties": {
					"listen": { "type": "string" }
				}
			}
		},
		"properties": {
			"iproto": { "$ref": "#/$defs/netbox" }
		}
	}`)

	root, err := envpath.Build(schema)
	require.NoError(t, err)

	got := root.Resolve("IPROTO_LISTEN")
	assert.Equal(t, config.NewKeyPathFromSegments([]string{"iproto", "listen"}), got)
}
