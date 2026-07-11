// Package docking implements pure dock-tree geometry, hit testing, mutation,
// and validation. It deliberately has no dependency on Raylib or other native
// UI packages.
package docking

import "errors"

var (
	// ErrInvalidGeometry identifies invalid geometry inputs or dock-tree shape.
	ErrInvalidGeometry = errors.New("invalid dock geometry")
	// ErrInvalidLayout identifies a workspace layout that violates dock invariants.
	ErrInvalidLayout = errors.New("invalid dock layout")
	// ErrNotFound identifies a requested panel, stack, or split that does not exist.
	ErrNotFound = errors.New("dock item not found")
	// ErrDuplicateID identifies a panel, node, or link-group ID collision.
	ErrDuplicateID = errors.New("duplicate dock ID")
	// ErrSingleton identifies an attempt to create a second singleton panel.
	ErrSingleton = errors.New("singleton panel already exists")
	// ErrInvalidMutation identifies a dock operation that cannot be applied.
	ErrInvalidMutation = errors.New("invalid dock mutation")
)
