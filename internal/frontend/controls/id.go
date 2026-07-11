package controls

import "strconv"

// WidgetID is a stable hierarchical widget identity. Its encoded form is
// collision-free even when segments contain separators or are empty.
type WidgetID string

// NewWidgetID constructs an ID from string hierarchy segments. Supplying no
// segments returns the invalid zero ID.
func NewWidgetID(segments ...string) WidgetID {
	var id WidgetID
	for _, segment := range segments {
		id = id.Child(segment)
	}
	return id
}

// Child returns a child ID beneath id using one string segment. Calling Child
// on the zero ID constructs a root ID.
func (id WidgetID) Child(segment string) WidgetID {
	encoded := "s" + strconv.Itoa(len(segment)) + ":" + segment
	return WidgetID(string(id) + encoded)
}

// ChildIndex returns a child ID beneath id using a numeric segment. Numeric
// segments cannot collide with string segments containing the same digits.
func (id WidgetID) ChildIndex(index int) WidgetID {
	encoded := "i" + strconv.Itoa(index) + ";"
	return WidgetID(string(id) + encoded)
}

// Valid reports whether id is not the reserved zero ID.
func (id WidgetID) Valid() bool {
	return id != ""
}

// String returns the stable encoded ID.
func (id WidgetID) String() string {
	return string(id)
}

// DescendsFrom reports whether id is a proper child of ancestor.
func (id WidgetID) DescendsFrom(ancestor WidgetID) bool {
	if !id.Valid() || !ancestor.Valid() || len(id) <= len(ancestor) {
		return false
	}
	return string(id[:len(ancestor)]) == string(ancestor)
}
