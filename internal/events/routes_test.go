package events

import "testing"

func TestParseEventPath(t *testing.T) {
	for _, test := range []struct {
		path string
		want string
	}{
		{path: "/v1/events/evt-1", want: "evt-1"},
		{path: "/v1/events/evt-1/", want: "evt-1"},
	} {
		got, ok := parseEventPath(test.path)
		if !ok || got != test.want {
			t.Fatalf("parseEventPath(%q) = %q, %v; want %q, true", test.path, got, ok, test.want)
		}
	}
}

func TestParseEventPathRejectsCollectionAndNestedPaths(t *testing.T) {
	for _, path := range []string{"/v1/events", "/v1/events/", "/v1/events/a/b"} {
		if _, ok := parseEventPath(path); ok {
			t.Fatalf("parseEventPath(%q) accepted an invalid event path", path)
		}
	}
}
