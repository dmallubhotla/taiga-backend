package workouts

import "testing"

func TestProcessorVersionTracksDistances(t *testing.T) {
	base := processorVersion(1, StandardDistances)
	if base != ProcessorVersion {
		t.Errorf("ProcessorVersion = %q, want %q", ProcessorVersion, base)
	}
	if processorVersion(1, StandardDistances) != base {
		t.Error("processorVersion isn't deterministic")
	}
	if processorVersion(2, StandardDistances) == base {
		t.Error("bumping the extractor version didn't change ProcessorVersion")
	}
	added := append(append([]NamedDistance{}, StandardDistances...), NamedDistance{"15k", 15000})
	if processorVersion(1, added) == base {
		t.Error("adding a distance didn't change ProcessorVersion")
	}
	renamed := append([]NamedDistance{}, StandardDistances...)
	renamed[0] = NamedDistance{"1km", renamed[0].Meters}
	if processorVersion(1, renamed) == base {
		t.Error("renaming a distance didn't change ProcessorVersion")
	}
}
