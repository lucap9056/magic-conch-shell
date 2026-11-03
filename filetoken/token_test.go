package filetoken

import (
	"bytes"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"
)

const (
	testURI  = "https://generativelanguage.googleapis.com/v1beta/files/abc123"
	testMIME = "image/png"
)

var baseTime = time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

func testKey(id string, fill byte) Key {
	return Key{ID: id, Secret: bytes.Repeat([]byte{fill}, MinSecretSize)}
}

func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func mustSigner(t *testing.T, key Key, opts ...SignerOption) *Signer {
	t.Helper()
	signer, err := NewSigner(key, append([]SignerOption{WithSignerClock(fixedClock(baseTime))}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func mustVerifier(t *testing.T, now time.Time, keys ...Key) *Verifier {
	t.Helper()
	verifier, err := NewVerifier(keys, WithVerifierClock(fixedClock(now)))
	if err != nil {
		t.Fatal(err)
	}
	return verifier
}

func mustSign(t *testing.T, signer *Signer) string {
	t.Helper()
	token, err := signer.Sign(testURI, testMIME)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func TestRoundTrip(t *testing.T) {
	key := testKey("k1", 'a')
	token := mustSign(t, mustSigner(t, key))

	if !IsToken(token) {
		t.Fatalf("IsToken(%q) = false", token)
	}

	claims, err := mustVerifier(t, baseTime, key).Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.URI != testURI || claims.MIMEType != testMIME {
		t.Errorf("claims = %+v", claims)
	}
	if want := baseTime.Add(DefaultTTL); !claims.ExpiresAt.Equal(want) {
		t.Errorf("ExpiresAt = %v, want %v", claims.ExpiresAt, want)
	}
}

func TestExpiry(t *testing.T) {
	key := testKey("k1", 'a')
	token := mustSign(t, mustSigner(t, key, WithTTL(time.Hour)))

	if _, err := mustVerifier(t, baseTime.Add(time.Hour-time.Second), key).Verify(token); err != nil {
		t.Fatalf("Verify before expiry: %v", err)
	}
	if _, err := mustVerifier(t, baseTime.Add(time.Hour), key).Verify(token); !errors.Is(err, ErrExpired) {
		t.Fatalf("Verify at expiry error = %v, want ErrExpired", err)
	}
}

func TestKeyRotation(t *testing.T) {
	oldKey, newKey := testKey("old", 'a'), testKey("new", 'b')
	oldToken := mustSign(t, mustSigner(t, oldKey))
	newToken := mustSign(t, mustSigner(t, newKey))

	verifier := mustVerifier(t, baseTime, newKey, oldKey)
	for _, token := range []string{oldToken, newToken} {
		if _, err := verifier.Verify(token); err != nil {
			t.Errorf("Verify(%q) = %v", token, err)
		}
	}

	if _, err := mustVerifier(t, baseTime, newKey).Verify(oldToken); !errors.Is(err, ErrUnknownKey) {
		t.Errorf("retired key error = %v, want ErrUnknownKey", err)
	}
}

func TestSameKeyIDDifferentSecret(t *testing.T) {
	token := mustSign(t, mustSigner(t, testKey("k1", 'a')))
	if _, err := mustVerifier(t, baseTime, testKey("k1", 'b')).Verify(token); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("error = %v, want ErrInvalidSignature", err)
	}
}

func TestTampering(t *testing.T) {
	key := testKey("k1", 'a')
	token := mustSign(t, mustSigner(t, key))
	verifier := mustVerifier(t, baseTime, key)

	segments := strings.Split(strings.TrimPrefix(token, Prefix), ".")
	forgedBody := base64.RawURLEncoding.EncodeToString([]byte(`{"u":"https://evil.example/x","m":"image/png","e":9999999999}`))
	flippedSignature := []byte(segments[2])
	flippedSignature[0] ^= 1

	tests := map[string]string{
		"forged payload":    Prefix + segments[0] + "." + forgedBody + "." + segments[2],
		"flipped signature": Prefix + segments[0] + "." + segments[1] + "." + string(flippedSignature),
		"truncated":         token[:len(token)-4],
	}
	for name, tampered := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := verifier.Verify(tampered)
			if !errors.Is(err, ErrInvalidSignature) && !errors.Is(err, ErrMalformed) {
				t.Fatalf("error = %v, want ErrInvalidSignature or ErrMalformed", err)
			}
		})
	}
}

func TestMalformed(t *testing.T) {
	key := testKey("k1", 'a')
	verifier := mustVerifier(t, baseTime, key)

	tests := []string{
		"",
		"rdx://rediscache/abc",
		"https://cdn.example.com/a.png",
		Prefix,
		Prefix + "k1",
		Prefix + "k1.body",
		Prefix + "k1.body.sig.extra",
		Prefix + "k1.!!!.sig",
	}
	for _, token := range tests {
		if _, err := verifier.Verify(token); err == nil {
			t.Errorf("Verify(%q) succeeded, want error", token)
		}
	}
}

func TestMissingClaimsRejected(t *testing.T) {
	key := testKey("k1", 'a')
	body := base64.RawURLEncoding.EncodeToString([]byte(`{"u":"","m":"image/png","e":9999999999}`))
	signingInput := Prefix + key.ID + "." + body
	token := signingInput + "." + base64.RawURLEncoding.EncodeToString(sign(key.Secret, signingInput))

	if _, err := mustVerifier(t, baseTime, key).Verify(token); !errors.Is(err, ErrMalformed) {
		t.Fatalf("error = %v, want ErrMalformed", err)
	}
}

func TestSignRejectsEmptyClaims(t *testing.T) {
	signer := mustSigner(t, testKey("k1", 'a'))
	if _, err := signer.Sign("", testMIME); !errors.Is(err, ErrEmptyClaim) {
		t.Errorf("empty uri error = %v", err)
	}
	if _, err := signer.Sign(testURI, ""); !errors.Is(err, ErrEmptyClaim) {
		t.Errorf("empty mime error = %v", err)
	}
}

func TestConstructorValidation(t *testing.T) {
	if _, err := NewSigner(Key{ID: "k1", Secret: []byte("short")}); !errors.Is(err, ErrSecretTooShort) {
		t.Errorf("short secret error = %v", err)
	}
	if _, err := NewSigner(testKey("bad.id", 'a')); !errors.Is(err, ErrInvalidKeyID) {
		t.Errorf("dotted key id error = %v", err)
	}
	if _, err := NewSigner(testKey("k1", 'a'), WithTTL(0)); !errors.Is(err, ErrInvalidTTL) {
		t.Errorf("zero ttl error = %v", err)
	}
	if _, err := NewVerifier(nil); !errors.Is(err, ErrNoKeys) {
		t.Errorf("no keys error = %v", err)
	}
	if _, err := NewVerifier([]Key{testKey("k1", 'a'), testKey("k1", 'b')}); !errors.Is(err, ErrDuplicateKeyID) {
		t.Errorf("duplicate key error = %v", err)
	}
}

func TestKeyIsCopied(t *testing.T) {
	key := testKey("k1", 'a')
	signer := mustSigner(t, key)
	verifier := mustVerifier(t, baseTime, key)
	key.Secret[0] = 'z'

	if _, err := verifier.Verify(mustSign(t, signer)); err != nil {
		t.Fatalf("mutating caller secret affected signer/verifier: %v", err)
	}
}

func TestParseKeys(t *testing.T) {
	secretA := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{'a'}, MinSecretSize))
	secretB := base64.URLEncoding.EncodeToString(bytes.Repeat([]byte{'b'}, MinSecretSize))

	keys, err := ParseKeys(" new:" + secretA + " , old:" + secretB + ",")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 || keys[0].ID != "new" || keys[1].ID != "old" {
		t.Fatalf("keys = %+v", keys)
	}
	if !bytes.Equal(keys[1].Secret, bytes.Repeat([]byte{'b'}, MinSecretSize)) {
		t.Errorf("padded base64 secret decoded incorrectly")
	}

	invalid := []string{
		"no-separator",
		"k1:not*base64",
		"k1:" + base64.RawURLEncoding.EncodeToString([]byte("short")),
		"bad.id:" + secretA,
	}
	for _, s := range invalid {
		if _, err := ParseKey(s); err == nil {
			t.Errorf("ParseKey(%q) succeeded, want error", s)
		}
	}
}
