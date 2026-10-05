package assets

import "testing"

// Run with -tags release to check that the local key opens the sealed message.
func TestFinalMessage(t *testing.T) {
	msg, err := FinalMessage()
	if Key() == nil {
		if err != ErrNoKey {
			t.Fatalf("without a key want ErrNoKey, got %v", err)
		}
		return
	}
	if err != nil || msg == "" {
		t.Fatalf("release key does not open the sealed message: %v", err)
	}
}
