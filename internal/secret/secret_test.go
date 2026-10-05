package secret

import "testing"

func TestSealOpen(t *testing.T) {
	k, _ := NewKey()
	sealed, err := Seal(k, []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := Open(k, sealed)
	if err != nil || string(got) != "hello" {
		t.Fatalf("open = %q, %v", got, err)
	}
	other, _ := NewKey()
	if _, err := Open(other, sealed); err == nil {
		t.Fatal("wrong key must fail")
	}
	sealed[len(sealed)-1] ^= 1
	if _, err := Open(k, sealed); err == nil {
		t.Fatal("tampered ciphertext must fail")
	}
}

func TestSignVerify(t *testing.T) {
	k, _ := NewKey()
	sig := Sign(k, "smoking|123")
	if !Verify(k, "smoking|123", sig) {
		t.Fatal("valid signature rejected")
	}
	if Verify(k, "smoking|122", sig) || Verify(k, "smoking|123", "") || Verify(k, "smoking|123", "zz") {
		t.Fatal("bad signature accepted")
	}
}
