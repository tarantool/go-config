package meta_test

import (
	"testing"

	"github.com/shoenig/test"

	"github.com/tarantool/go-config/keypath"
	"github.com/tarantool/go-config/meta"
)

func TestSourceType_ConstantsExist(t *testing.T) {
	t.Parallel()

	// Verify all constants are defined.
	test.Eq(t, meta.UnknownSource, meta.SourceType(0))
	test.Eq(t, meta.EnvDefaultSource, meta.SourceType(1))
	test.Eq(t, meta.StorageSource, meta.SourceType(2))
	test.Eq(t, meta.FileSource, meta.SourceType(3))
	test.Eq(t, meta.EnvSource, meta.SourceType(4))
	test.Eq(t, meta.ModifiedSource, meta.SourceType(5))
}

func TestSourceInfo_ZeroValue(t *testing.T) {
	t.Parallel()

	var si meta.SourceInfo
	test.Eq(t, "", si.Name)
	test.Eq(t, meta.SourceType(0), si.Type)
}

func TestSourceInfo_WithFields(t *testing.T) {
	t.Parallel()

	si := meta.SourceInfo{
		Name: "testfile",
		Type: meta.FileSource,
	}
	test.Eq(t, "testfile", si.Name)
	test.Eq(t, meta.FileSource, si.Type)
}

func TestInfo_ZeroValue(t *testing.T) {
	t.Parallel()

	var info meta.Info
	test.Eq(t, keypath.KeyPath(nil), info.Key)
	test.Eq(t, meta.SourceInfo{Name: "", Type: meta.UnknownSource}, info.Source)
	test.Eq(t, meta.RevisionType(""), info.Revision)
}

func TestInfo_WithKey(t *testing.T) {
	t.Parallel()

	key := keypath.NewKeyPath("a/b/c")
	info := meta.Info{
		Key:      key,
		Source:   meta.SourceInfo{Name: "", Type: meta.UnknownSource},
		Revision: "",
	}
	test.True(t, info.Key.Equals(key))
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
	test.Eq(t, "env", info.Source.Name)
	test.Eq(t, meta.EnvSource, info.Source.Type)
}

func TestInfo_WithRevision(t *testing.T) {
	t.Parallel()

	info := meta.Info{
		Key:      nil,
		Source:   meta.SourceInfo{Name: "", Type: meta.UnknownSource},
		Revision: "v1.0.0",
	}
	test.Eq(t, meta.RevisionType("v1.0.0"), info.Revision)
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
	test.True(t, info.Key.Equals(key))
	test.Eq(t, "config.yaml", info.Source.Name)
	test.Eq(t, meta.FileSource, info.Source.Type)
	test.Eq(t, meta.RevisionType("abc123"), info.Revision)
}
