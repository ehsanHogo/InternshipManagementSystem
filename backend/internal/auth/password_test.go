package auth

import "testing"

func TestPasswordHashAndComparison(t *testing.T) {
	hash, err := HashPassword("Demo123!")
	if err != nil {
		t.Fatalf("HashPassword returned an error: %v", err)
	}
	if hash == "Demo123!" {
		t.Fatal("password was stored as plaintext")
	}
	if !CheckPassword(hash, "Demo123!") {
		t.Fatal("valid password did not match")
	}
	if CheckPassword(hash, "wrong password") {
		t.Fatal("invalid password matched")
	}
}
