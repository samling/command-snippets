package models

import "testing"

func TestCanonicalSchemaAndIdentity(t *testing.T) {
	id, err := NewID()
	if err != nil || !ValidID(id) {
		t.Fatalf("id %q: %v", id, err)
	}
	good := Snippet{ID: id, Name: "Friendly name, with spaces", Command: "echo hello", Tags: []string{" Kubernetes ", "Monitoring"}}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
	if good.Tags[0] != "Kubernetes" {
		t.Fatal("tag not trimmed")
	}
	for _, bad := range []Snippet{{Name: "", Command: "echo"}, {Name: "Bad\x1b[0m", Command: "echo"}, {Name: "Bad", Command: "echo", ID: "slug"}, {Name: "Bad", Command: "echo", Inputs: []Input{{Name: "same"}, {Name: "same"}}}, {Name: "Bad", Command: "echo", Inputs: []Input{{Name: "x", Kind: "toggle", Flag: "-x", Default: "true"}}}, {Name: "Bad", Command: "echo", Inputs: []Input{{Name: "x", Validate: &Validation{Range: []int{9, 1}}}}}} {
		if err := bad.Validate(); err == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
}
