package decision

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		d    Decision
	}{
		{
			name: "full",
			d: Decision{
				ID: "d-1", Ticket: "why-590", Concepts: []string{"decision-format"},
				Date: "2026-10-08", DecidedBy: "brian", Supersedes: []string{"d-0"},
				Question: "Where does prose live?", Ruling: "Markdown sections.",
				Why: "Readable in review.\n\nSecond paragraph.",
			},
		},
		{
			name: "minimal",
			d:    Decision{ID: "d-2", Date: "2026-10-08", DecidedBy: "brian", Ruling: "Yes."},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.d.Marshal()
			if err != nil {
				t.Fatal(err)
			}
			got, err := Parse(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("Parse(%s): %v", data, err)
			}
			if !reflect.DeepEqual(got, tt.d) {
				t.Errorf("round trip:\n got %#v\nwant %#v\nfile:\n%s", got, tt.d, data)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"no frontmatter", "## Question\nx\n", "missing frontmatter"},
		{"unterminated", "---\nid: x\n", "unterminated"},
		{"unknown key", "---\nid: x\nbogus: y\n---\n", "bogus"},
		{"unknown section", "---\nid: x\n---\n## Notes\nx\n", "unknown section"},
		{"duplicate section", "---\nid: x\n---\n## Why\na\n## Why\nb\n", "duplicate section"},
		{"text outside section", "---\nid: x\n---\nstray\n## Why\na\n", "outside a section"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tt.in))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Parse error = %v, want containing %q", err, tt.want)
			}
		})
	}
}
