package structtag_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tarantool/go-config/internal/structtag"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		tag      string
		wantName string
		wantOpts []string
	}{
		{name: "empty", tag: "", wantName: "", wantOpts: nil},
		{name: "name only", tag: "id", wantName: "id", wantOpts: nil},
		{name: "name and option", tag: "id,omitempty", wantName: "id", wantOpts: []string{"omitempty"}},
		{name: "option only", tag: ",inline", wantName: "", wantOpts: []string{"inline"}},
		{
			name: "multiple options", tag: "id,omitempty,inline",
			wantName: "id", wantOpts: []string{"omitempty", "inline"},
		},
		{name: "trailing comma", tag: "id,", wantName: "id", wantOpts: nil},
		{name: "empty options between commas", tag: "id,,inline", wantName: "id", wantOpts: []string{"inline"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			name, opts := structtag.Parse(tt.tag)
			assert.Equal(t, tt.wantName, name)

			for _, opt := range tt.wantOpts {
				assert.True(t, opts.Has(opt), "expected option %q", opt)
			}

			assert.Len(t, opts, len(tt.wantOpts))
		})
	}
}

func TestOptions_Has_Absent(t *testing.T) {
	t.Parallel()

	_, opts := structtag.Parse("id,omitempty")
	assert.False(t, opts.Has("inline"))
}
