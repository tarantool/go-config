package meta_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tarantool/go-config/keypath"
	"github.com/tarantool/go-config/meta"
)

func TestSourceType_ConstantsExist(t *testing.T) {
	t.Parallel()

	assert.Equal(t, meta.UnknownSource, meta.SourceType(0))
	assert.Equal(t, meta.EnvDefaultSource, meta.SourceType(1))
	assert.Equal(t, meta.StorageSource, meta.SourceType(2))
	assert.Equal(t, meta.FileSource, meta.SourceType(3))
	assert.Equal(t, meta.EnvSource, meta.SourceType(4))
	assert.Equal(t, meta.ModifiedSource, meta.SourceType(5))
}

func TestSourceInfo_ZeroValue(t *testing.T) {
	t.Parallel()

	var si meta.SourceInfo
	assert.Empty(t, si.Name)
	assert.Equal(t, meta.SourceType(0), si.Type)
}

func TestSourceInfo_WithFields(t *testing.T) {
	t.Parallel()

	si := meta.SourceInfo{
		Name: "testfile",
		Type: meta.FileSource,
	}
	assert.Equal(t, "testfile", si.Name)
	assert.Equal(t, meta.FileSource, si.Type)
}

func TestInfo_ZeroValue(t *testing.T) {
	t.Parallel()

	var info meta.Info
	assert.Equal(t, keypath.KeyPath(nil), info.Key)
	assert.Equal(t, meta.SourceInfo{Name: "", Type: meta.UnknownSource}, info.Source)
	assert.Equal(t, meta.RevisionType(""), info.Revision)
}

func TestInfo_WithKey(t *testing.T) {
	t.Parallel()

	key := keypath.NewKeyPath("a/b/c")
	info := meta.Info{
		Key:      key,
		Source:   meta.SourceInfo{Name: "", Type: meta.UnknownSource},
		Revision: "",
	}
	assert.True(t, info.Key.Equals(key))
}

func TestInfo_WithSource(t *testing.T) {
	t.Parallel()

	source := meta.SourceInfo{
		Name: "env",
		Type: meta.EnvSource,
	}
	info := meta.Info{
		Key:      nil,
		Source:   source,
		Revision: "",
	}
	assert.Equal(t, "env", info.Source.Name)
	assert.Equal(t, meta.EnvSource, info.Source.Type)
}

func TestInfo_WithRevision(t *testing.T) {
	t.Parallel()

	info := meta.Info{
		Key:      nil,
		Source:   meta.SourceInfo{Name: "", Type: meta.UnknownSource},
		Revision: "v1.0.0",
	}
	assert.Equal(t, meta.RevisionType("v1.0.0"), info.Revision)
}

func TestInfo_FullConstruction(t *testing.T) {
	t.Parallel()

	key := keypath.NewKeyPath("server/port")
	source := meta.SourceInfo{
		Name: "config.yaml",
		Type: meta.FileSource,
	}
	info := meta.Info{
		Key:      key,
		Source:   source,
		Revision: "abc123",
	}
	assert.True(t, info.Key.Equals(key))
	assert.Equal(t, "config.yaml", info.Source.Name)
	assert.Equal(t, meta.FileSource, info.Source.Type)
	assert.Equal(t, meta.RevisionType("abc123"), info.Revision)
}
