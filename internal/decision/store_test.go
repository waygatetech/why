package decision

import (
	"reflect"
	"strings"
	"testing"
)

func TestNextID(t *testing.T) {
	tests := []struct {
		name   string
		ids    []string
		ticket string
		want   string
	}{
		{"empty", nil, "why-1", "why-1-1"},
		{"after max", []string{"why-1-1", "why-1-10", "why-1-2"}, "why-1", "why-1-11"},
		{"other tickets ignored", []string{"why-12-5", "why-2-3"}, "why-1", "why-1-1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ds []Decision
			for _, id := range tt.ids {
				ds = append(ds, Decision{ID: id})
			}
			if got := NextID(ds, tt.ticket); got != tt.want {
				t.Errorf("NextID = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestActive(t *testing.T) {
	ds := []Decision{{ID: "a"}, {ID: "b", Supersedes: []string{"a"}}, {ID: "c"}}
	var got []string
	for _, d := range Active(ds) {
		got = append(got, d.ID)
	}
	if want := []string{"b", "c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Active = %v, want %v", got, want)
	}
}

func TestWriteLoad(t *testing.T) {
	dir := t.TempDir() + "/decisions"
	if ds, err := Load(dir); err != nil || ds != nil {
		t.Fatalf("Load(missing) = %v, %v; want nil, nil", ds, err)
	}
	d := Decision{ID: "why-1-1", Ticket: "why-1", Date: "2026-10-08", DecidedBy: "agent", Ruling: "Yes."}
	if _, err := Write(dir, d); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(dir, d); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("second Write error = %v, want already exists", err)
	}
	if _, err := Write(dir, Decision{ID: "../x"}); err == nil {
		t.Error("Write with path in id succeeded")
	}
	ds, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ds, []Decision{d}) {
		t.Errorf("Load = %#v, want %#v", ds, []Decision{d})
	}
}
