package storage

import (
	"bytes"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	password := "test-password-123"
	plaintext := []byte("This is a secret message")

	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if bytes.Equal(encrypted, plaintext) {
		t.Error("Encrypted data should not equal plaintext")
	}

	decrypted, err := Decrypt(encrypted, password)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("Decrypted data does not match plaintext.\nExpected: %s\nGot: %s", plaintext, decrypted)
	}
}

func TestDecryptWithWrongPassword(t *testing.T) {
	password := "correct-password"
	wrongPassword := "wrong-password"
	plaintext := []byte("Secret data")

	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = Decrypt(encrypted, wrongPassword)
	if err == nil {
		t.Error("Decrypt should fail with wrong password")
	}
}

func TestDecryptInvalidData(t *testing.T) {
	password := "test-password"
	invalidData := []byte("too short")

	_, err := Decrypt(invalidData, password)
	if err == nil {
		t.Error("Decrypt should fail with invalid data")
	}
}

func TestEncryptDecryptEmptyData(t *testing.T) {
	password := "test-password"
	plaintext := []byte("")

	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := Decrypt(encrypted, password)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Error("Decrypted empty data does not match")
	}
}

func TestEncryptDecryptLargeData(t *testing.T) {
	password := "test-password"
	plaintext := bytes.Repeat([]byte("Large data test. "), 1000)

	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := Decrypt(encrypted, password)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Error("Decrypted large data does not match")
	}
}
