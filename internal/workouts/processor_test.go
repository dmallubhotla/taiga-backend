package workouts

import "testing"

func TestProcessorVersionTracksDistances(t *testing.T) {
	base := processorVersion(extractorVersion, StandardDistances)
	if base != ProcessorVersion {
		t.Errorf("ProcessorVersion = %q, want %q", ProcessorVersion, base)
	}
	if processorVersion(extractorVersion, StandardDistances) != base {
		t.Error("processorVersion isn't deterministic")
	}
	if processorVersion(extractorVersion+1, StandardDistances) == base {
		t.Error("bumping the extractor version didn't change ProcessorVersion")
	}
	added := append(append([]NamedDistance{}, StandardDistances...), NamedDistance{"15k", 15000})
	if processorVersion(extractorVersion, added) == base {
		t.Error("adding a distance didn't change ProcessorVersion")
	}
	renamed := append([]NamedDistance{}, StandardDistances...)
	renamed[0] = NamedDistance{"1km", renamed[0].Meters}
	if processorVersion(extractorVersion, renamed) == base {
		t.Error("renaming a distance didn't change ProcessorVersion")
	}
}
