// Package assets owns the frontend's embedded fonts, icon atlas, and notices.
package assets

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"errors"
	"fmt"
	"image/png"
)

const (
	// AtlasCellSize is the width and height of one icon in the atlas.
	AtlasCellSize = 24
	// AtlasIconCount is the number of cells in the bundled atlas.
	AtlasIconCount = 4
)

//go:generate go run generate.go

var (
	//go:embed fonts/InterVariable.ttf
	interFont []byte

	//go:embed fonts/JetBrainsMono-Regular.ttf
	jetBrainsMonoFont []byte

	//go:embed icons/lucide-atlas.png
	iconAtlas []byte

	//go:embed licenses/Inter-OFL-1.1.txt
	interLicense string

	//go:embed licenses/JetBrainsMono-OFL-1.1.txt
	jetBrainsMonoLicense string

	//go:embed licenses/Lucide-ISC-MIT.txt
	lucideLicense string
)

// Set is a validated collection of immutable embedded frontend assets.
// Accessors return copies so callers cannot modify process-wide embedded data.
type Set struct {
	interFont            []byte
	jetBrainsMonoFont    []byte
	iconAtlas            []byte
	interLicense         string
	jetBrainsMonoLicense string
	lucideLicense        string
}

// Load validates all required assets without invoking a native graphics API.
func Load() (Set, error) {
	set := Set{
		interFont:            interFont,
		jetBrainsMonoFont:    jetBrainsMonoFont,
		iconAtlas:            iconAtlas,
		interLicense:         interLicense,
		jetBrainsMonoLicense: jetBrainsMonoLicense,
		lucideLicense:        lucideLicense,
	}
	if err := set.validate(); err != nil {
		return Set{}, err
	}
	return set, nil
}

// InterFont returns a copy of the bundled Inter font bytes.
func (set Set) InterFont() []byte {
	return bytes.Clone(set.interFont)
}

// JetBrainsMonoFont returns a copy of the bundled JetBrains Mono font bytes.
func (set Set) JetBrainsMonoFont() []byte {
	return bytes.Clone(set.jetBrainsMonoFont)
}

// IconAtlas returns a copy of the bundled Lucide-derived PNG atlas.
func (set Set) IconAtlas() []byte {
	return bytes.Clone(set.iconAtlas)
}

// Fingerprint returns a stable identity for every rasterized frontend asset.
func (set Set) Fingerprint() string {
	digest := sha256.New()
	_, _ = digest.Write(set.interFont)
	_, _ = digest.Write(set.jetBrainsMonoFont)
	_, _ = digest.Write(set.iconAtlas)
	return fmt.Sprintf("sha256:%x", digest.Sum(nil))
}

func (set Set) validate() error {
	if err := validateTrueType("Inter", set.interFont); err != nil {
		return err
	}
	if err := validateTrueType("JetBrains Mono", set.jetBrainsMonoFont); err != nil {
		return err
	}
	config, err := png.DecodeConfig(bytes.NewReader(set.iconAtlas))
	if err != nil {
		return fmt.Errorf("validate Lucide icon atlas: %w", err)
	}
	if config.Width != AtlasCellSize*AtlasIconCount || config.Height != AtlasCellSize {
		return fmt.Errorf(
			"validate Lucide icon atlas: got %dx%d, want %dx%d",
			config.Width,
			config.Height,
			AtlasCellSize*AtlasIconCount,
			AtlasCellSize,
		)
	}
	if set.interLicense == "" || set.jetBrainsMonoLicense == "" || set.lucideLicense == "" {
		return errors.New("validate frontend asset licenses: notice is empty")
	}
	return nil
}

func validateTrueType(name string, data []byte) error {
	if len(data) < 12 {
		return fmt.Errorf("validate %s font: file is too short", name)
	}
	scalerType := binary.BigEndian.Uint32(data[:4])
	if scalerType != 0x00010000 && string(data[:4]) != "OTTO" {
		return fmt.Errorf("validate %s font: unsupported scaler type %#08x", name, scalerType)
	}
	if binary.BigEndian.Uint16(data[4:6]) == 0 {
		return fmt.Errorf("validate %s font: file has no tables", name)
	}
	return nil
}
