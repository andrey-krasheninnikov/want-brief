package crypto

import "testing"

func TestSealOpen(t *testing.T) {
	box, err := NewBox("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := box.Seal("pat-secret")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := box.Open(sealed)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "pat-secret" {
		t.Fatalf("got %q", plain)
	}
}
