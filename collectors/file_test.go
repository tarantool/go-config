package collectors_test

import (
	"testing"

	"github.com/shoenig/test"
	"github.com/shoenig/test/must"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
)

var config_yaml = `
credentials:
  users:
    guest:
      roles: [super]
`

func TestNewFile(t *testing.T) {
	t.Parallel()

	fc := collectors.NewFile()
	must.NotNil(t, fc)
	test.Eq(t, "file", fc.Name())
	test.Eq(t, config.FileSource, fc.Source())
	test.Eq(t, "", fc.Revision())
	test.True(t, fc.KeepOrder())
}

func TestFile_WithName(t *testing.T) {
	t.Parallel()

	fc := collectors.NewFile().WithName("custom")
	test.Eq(t, "custom", fc.Name())
}

func TestFile_WithSourceType(t *testing.T) {
	t.Parallel()

	fc := collectors.NewFile().WithSourceType(config.UnknownSource)
	test.Eq(t, config.UnknownSource, fc.Source())
}

func TestFile_WithRevision(t *testing.T) {
	t.Parallel()

	fc := collectors.NewFile().WithRevision("v1.0.0")
	test.Eq(t, "v1.0.0", fc.Revision())
}

func TestFile_WithKeepOrder(t *testing.T) {
	t.Parallel()

	fc := collectors.NewFile().WithKeepOrder(false)
	test.False(t, fc.KeepOrder())
}
