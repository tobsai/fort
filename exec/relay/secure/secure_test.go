package secure

import (
	"bytes"
	"testing"
)

// handshake drives initiator/responder to completion, returning both sessions.
func handshake(t *testing.T, daemon, client Keypair, pinnedPub []byte) (cli, srv *Session, err error) {
	t.Helper()
	init, err := NewInitiator(client, pinnedPub)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := NewResponder(daemon)
	if err != nil {
		t.Fatal(err)
	}
	m1, err := init.WriteMessage(nil)
	if err != nil {
		return nil, nil, err
	}
	if _, err = resp.ReadMessage(m1); err != nil {
		return nil, nil, err
	}
	m2, err := resp.WriteMessage(nil)
	if err != nil {
		return nil, nil, err
	}
	if _, err = init.ReadMessage(m2); err != nil {
		return nil, nil, err
	}
	return init.Session(), resp.Session(), nil
}

func TestHandshakeAndRoundTrip(t *testing.T) {
	d, _ := GenerateKeypair()
	c, _ := GenerateKeypair()
	cli, srv, err := handshake(t, d, c, d.Public)
	if err != nil {
		t.Fatalf("handshake: %v", err)
	}
	ct := cli.Seal([]byte("GET /api/board"))
	if bytes.Contains(ct, []byte("api/board")) {
		t.Fatal("ciphertext leaks plaintext")
	}
	pt, err := srv.Open(ct)
	if err != nil || string(pt) != "GET /api/board" {
		t.Fatalf("open: %q err=%v", pt, err)
	}
	// and the reverse direction
	ct2 := srv.Seal([]byte("200 ok"))
	pt2, err := cli.Open(ct2)
	if err != nil || string(pt2) != "200 ok" {
		t.Fatalf("reverse: %q err=%v", pt2, err)
	}
}

func TestTamperedFrameRejected(t *testing.T) {
	d, _ := GenerateKeypair()
	c, _ := GenerateKeypair()
	cli, srv, err := handshake(t, d, c, d.Public)
	if err != nil {
		t.Fatal(err)
	}
	ct := cli.Seal([]byte("secret"))
	ct[len(ct)-1] ^= 0x01
	if _, err := srv.Open(ct); err == nil {
		t.Fatal("tampered ciphertext must be rejected")
	}
}

func TestWrongPinnedKeyFailsHandshake(t *testing.T) {
	d, _ := GenerateKeypair()
	evil, _ := GenerateKeypair() // a MITM broker's substituted key
	c, _ := GenerateKeypair()
	if _, _, err := handshake(t, d, c, evil.Public); err == nil {
		t.Fatal("handshake against a non-pinned static key must fail")
	}
}

func TestFingerprintStable(t *testing.T) {
	k, _ := GenerateKeypair()
	f1, f2 := k.Fingerprint(), k.Fingerprint()
	if f1 == "" || f1 != f2 || len(f1) < 12 {
		t.Fatalf("fingerprint %q/%q", f1, f2)
	}
	other, _ := GenerateKeypair()
	if other.Fingerprint() == f1 {
		t.Fatal("distinct keys must have distinct fingerprints")
	}
}

func TestPassiveObserverCannotDecrypt(t *testing.T) {
	d, _ := GenerateKeypair()
	c, _ := GenerateKeypair()
	cli, _, err := handshake(t, d, c, d.Public)
	if err != nil {
		t.Fatal(err)
	}
	// an observer with only the daemon's PUBLIC key and the frames
	obs, _ := GenerateKeypair()
	_, obsSess, err := handshake(t, obs, c, obs.Public) // unrelated session
	if err != nil {
		t.Fatal(err)
	}
	ct := cli.Seal([]byte("private"))
	if pt, err := obsSess.Open(ct); err == nil {
		t.Fatalf("observer decrypted: %q", pt)
	}
}
