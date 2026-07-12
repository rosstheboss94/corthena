package drafts

import "testing"

func FuzzDecode(f *testing.F) {
	seed, err := Encode(Snapshot{Revision: 1, Draft: validDraft(1)})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte(`{"schema_version":1,"revision":1,"draft":{}}`))
	f.Fuzz(func(t *testing.T, document []byte) {
		snapshot, err := Decode(document)
		if err != nil {
			return
		}
		encoded, err := Encode(snapshot)
		if err != nil {
			t.Fatalf("decoded snapshot cannot encode: %v", err)
		}
		if _, err := Decode(encoded); err != nil {
			t.Fatalf("canonical document cannot decode: %v", err)
		}
	})
}
