package main

import (
	"os"
	"strings"
)

// Clones the current environment, replacing (or appending) any values
// set in override with the mapped values.
func CloneFreshEnv(override map[string]string) (out []string) {
	for _, src := range os.Environ() {
		kv := strings.SplitN(src, "=", 2)
		if newval, ok := override[kv[0]]; ok {
			out = append(out, kv[0]+"="+newval)
			delete(override, kv[0])
		} else {
			out = append(out, src)
		}
	}
	for k, v := range override {
		out = append(out, k+"="+v)
	}
	return
}
