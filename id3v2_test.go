package id3v2

import (
	"testing"
)

func TestSynchSafeSize(t *testing.T) {
	size := uint32(0x0FFFFFFF)
	synchSafe := uint32(0x7F7F7F7F)

	if s := SynchSafeToSize(synchSafe); s != size {
		t.Errorf("expected SynchSafeToSize(0x%08X) to equal 0x%08X, but got 0x%08X", synchSafe, size, s)
	}

	if ss := SizeToSynchSafe(size); ss != synchSafe {
		t.Errorf("expected SizeToSynchSafe(0x%08X) to equal 0x%08X, but got 0x%08X", size, synchSafe, ss)
	}
}
