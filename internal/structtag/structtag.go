// Package structtag parses the value of a single struct tag into its name
// and options. It targets the near-universal `name,opt1,opt2` encoding used
// by encoding/json, gopkg.in/yaml.v3 and most reflection-based decoders, so
// it is not tied to any one tag key.
package structtag

import "strings"

// Options is the set of comma-separated options that follow the name in a
// struct tag value (for example "omitempty" or "inline").
type Options map[string]struct{}

// Has reports whether opt is present in the set.
func (o Options) Has(opt string) bool {
	_, ok := o[opt]
	return ok
}

// Parse splits a struct tag value of the form "name,opt1,opt2" into the
// name and the set of options. An empty name means the tag carried no
// explicit name (for example ",inline" or the empty tag), leaving it to the
// caller to fall back to the field name.
func Parse(tag string) (string, Options) {
	opts := Options{}

	name := tag
	if before, after, found := strings.Cut(tag, ","); found {
		name = before

		for opt := range strings.SplitSeq(after, ",") {
			if opt != "" {
				opts[opt] = struct{}{}
			}
		}
	}

	return name, opts
}
