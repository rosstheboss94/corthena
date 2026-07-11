package assets

import (
	"bytes"
	"testing"
)

func TestLoadValidatesBundledAssets(t *testing.T) {
	t.Parallel()

	set, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(set.InterFont()) == 0 {
		t.Fatal("Inter font is empty")
	}
	if len(set.JetBrainsMonoFont()) == 0 {
		t.Fatal("JetBrains Mono font is empty")
	}
	if len(set.IconAtlas()) == 0 {
		t.Fatal("icon atlas is empty")
	}
}

func TestAssetAccessorsReturnCopies(t *testing.T) {
	t.Parallel()

	set, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	first := set.InterFont()
	second := set.InterFont()
	first[0] ^= 0xff
	if bytes.Equal(first, second) {
		t.Fatal("mutating one font result changed another result")
	}
}
