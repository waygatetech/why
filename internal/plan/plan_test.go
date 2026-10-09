package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRead(t *testing.T) {
	tests := []struct {
		name    string
		content string
		ticket  string
		q       string
		wantErr string
	}{
		{
			name:    "valid",
			content: "---\nticket: why-1\ndecisions:\n  - q: Which?\n    recommend: A\n---\nbody\n",
			ticket:  "why-1",
			q:       "Which?",
		},
		{
			name:    "crlf",
			content: "---\r\nticket: why-2\r\ndecisions:\r\n  - q: Which?\r\n---\r\nbody\r\n",
			ticket:  "why-2",
			q:       "Which?",
		},
		{
			name:    "missing frontmatter",
			content: "ticket: why-3\n",
			wantErr: "missing frontmatter",
		},
		{
			name:    "unterminated",
			content: "---\nticket: why-4\n",
			wantErr: "unterminated frontmatter",
		},
		{
			name:    "no ticket",
			content: "---\nconcepts: [cli]\n---\n",
			wantErr: "no ticket",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "plan.md")
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatal(err)
			}
			p, err := Read(path)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Read() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Read() error = %v", err)
			}
			if p.Ticket != tt.ticket {
				t.Errorf("Ticket = %q, want %q", p.Ticket, tt.ticket)
			}
			if len(p.Decisions) != 1 || p.Decisions[0].Q != tt.q {
				t.Errorf("Decisions = %+v, want one with q %q", p.Decisions, tt.q)
			}
		})
	}
}
