package auth

import "testing"

func TestPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("s3cret!pw")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(hash, "s3cret!pw") {
		t.Fatal("correct password rejected")
	}
	if CheckPassword(hash, "wrong") {
		t.Fatal("wrong password accepted")
	}
}

func TestTokenAndHash(t *testing.T) {
	tok, err := GenerateToken()
	if err != nil || len(tok) != 64 {
		t.Fatalf("token len = %d err = %v", len(tok), err)
	}
	a, b := SHA256Hex("x"), SHA256Hex("x")
	if a != b || len(a) != 64 {
		t.Fatal("sha256 mismatch")
	}
}