package collectors_test

import (
	"context"
	"log"
	"testing"

	"github.com/shoenig/test"
	"github.com/shoenig/test/must"

	"github.com/tarantool/go-config"
	"github.com/tarantool/go-config/collectors"
)

const configFile = "testdata/config.yaml"

func TestNewFile(t *testing.T) {
	t.Parallel()

	fc := collectors.NewFileCollector(configFile)
	must.NotNil(t, fc)
	test.Eq(t, "file", fc.Name())
	test.Eq(t, config.FileSource, fc.Source())
	test.Eq(t, "", fc.Revision())
	test.True(t, fc.KeepOrder())
}

func TestNewFile_Unexist(t *testing.T) {
	t.Parallel()

	fc := collectors.NewFileCollector("unexist.file")
	must.Nil(t, fc)
}

func TestFile_WithName(t *testing.T) {
	t.Parallel()

	fc := collectors.NewFileCollector(configFile).WithName("custom")
	test.Eq(t, "custom", fc.Name())
}

func TestFile_WithSourceType(t *testing.T) {
	t.Parallel()

	fc := collectors.NewFileCollector(configFile).WithSourceType(config.FileSource)
	test.Eq(t, config.FileSource, fc.Source())
}

func TestFile_WithRevision(t *testing.T) {
	t.Parallel()

	fc := collectors.NewFileCollector(configFile).WithRevision("v1.0.0")
	test.Eq(t, "v1.0.0", fc.Revision())
}

func TestFile_WithKeepOrder(t *testing.T) {
	t.Parallel()

	fc := collectors.NewFileCollector(configFile).WithKeepOrder(false)
	test.False(t, fc.KeepOrder())
}

func TestFile_Read_Basic(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	fc := collectors.NewFileCollector(configFile)
	must.NotNil(t, fc)

	ch := fc.Read(ctx)

	values := make([]config.Value, 0, 512)
	for val := range ch {
		values = append(values, val)
	}

	// Verify values can be extracted.
	var length int

	for _, val := range values {
		var dest any

		err := val.Get(&dest)
		must.NoError(t, err)

		// Debug print.
		log.Println(val.Meta().Key, dest)

		length++
	}

	must.Eq(t, length, 39)
	must.Len(t, length, values)
}
