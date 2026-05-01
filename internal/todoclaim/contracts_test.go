package todoclaim

import "testing"

func TestForKindKnown(t *testing.T) {
	if got := ForKind("dev"); got.Name != Dev {
		t.Errorf("ForKind(dev).Name = %q, want %q", got.Name, Dev)
	}
}

func TestForKindUnknownDefaultsToMetadata(t *testing.T) {
	if got := ForKind("unknown"); got.Name != Metadata {
		t.Errorf("ForKind(unknown).Name = %q, want %q", got.Name, Metadata)
	}
}
