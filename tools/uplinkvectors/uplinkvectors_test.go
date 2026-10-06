package uplinkvectors

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

const vectorsPath = "../../contracts/uplink/v1/vectors/vectors.json"

func devices(f File) map[string]ed25519.PublicKey {
	pub, _ := hex.DecodeString(f.DevicePublicKey)
	return map[string]ed25519.PublicKey{f.DeviceID: pub}
}

// The checked-in file is exactly what the generator produces.
func TestVectorsFileIsCurrent(t *testing.T) {
	want := JSON(Build())
	got, err := os.ReadFile(vectorsPath)
	if err != nil {
		t.Fatalf("%v (generate it: go run ./cmd/uplink-vectors)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("contracts/uplink/v1/vectors/vectors.json differs from the generator's output; run: go run ./cmd/uplink-vectors")
	}
}

// Every positive case: the signing string has eight lines and no trailing newline, the
// stored fields reproduce, and the reference server accepts it.
func TestPositiveCases(t *testing.T) {
	f := Build()
	for _, c := range f.Positive {
		body, _ := hex.DecodeString(c.BodyHex)
		r := Request{Method: c.Method, Target: c.Target, Body: body, DeviceID: f.DeviceID, TS: c.TS, Nonce: c.Nonce, Meta: c.Meta}
		ss := SigningString(r)
		if ss != c.SigningString {
			t.Errorf("%s: signing string differs", c.Name)
		}
		if n := strings.Count(ss, "\n"); n != 7 || strings.HasSuffix(ss, "\n") && c.Meta != "" {
			t.Errorf("%s: want 8 lines (7 separators), got %d separators", c.Name, n)
		}
		srv := NewServer(devices(f))
		if v := srv.Check(c.Authorization, c.Method, c.Target, body, c.Meta, f.ServerNow); v != OK {
			t.Errorf("%s: verdict %s, want ok", c.Name, v)
		}
	}
}

// Every negative case is refused with exactly the stated verdict.
func TestNegativeCases(t *testing.T) {
	f := Build()
	byName := map[string]PositiveCase{}
	for _, c := range f.Positive {
		byName[c.Name] = c
	}
	for _, n := range f.Negative {
		c := byName[n.Base]
		body, _ := hex.DecodeString(c.BodyHex)
		method, target, meta, auth := c.Method, c.Target, c.Meta, c.Authorization
		devs := devices(f)
		k, v, _ := strings.Cut(n.Mutation, "=")
		switch k {
		case "meta":
			meta = v
		case "target":
			target = v
		case "method":
			method = v
		case "body_flip_first_byte":
			body = append([]byte{body[0] ^ 1}, body[1:]...)
		case "device":
			auth = strings.Replace(auth, "device="+f.DeviceID, "device="+v, 1)
		case "device_unenrolled":
			devs = map[string]ed25519.PublicKey{}
		case "sig_flip_bit":
			i := strings.Index(auth, "sig=") + 4
			b := []byte(auth)
			if b[i] == '0' {
				b[i] = '1'
			} else {
				b[i] = '0'
			}
			auth = string(b)
		case "none":
		default:
			t.Fatalf("%s: unknown mutation %q", n.Name, n.Mutation)
		}
		srv := NewServer(devs)
		if got := srv.Check(auth, method, target, body, meta, n.ServerNow); got != n.WantVerdict {
			t.Errorf("%s: verdict %s, want %s", n.Name, got, n.WantVerdict)
		}
		if n.DeliverTwo {
			if got := srv.Check(auth, method, target, body, meta, n.ServerNow); got != n.WantSecond {
				t.Errorf("%s: second delivery %s, want %s", n.Name, got, n.WantSecond)
			}
		}
	}
}

// A nonce is forgotten once its time to live (counted from acceptance) has passed, but by then the request is out of the
// clock window, so it is still refused.
func TestReplayAfterTTLIsStillRefused(t *testing.T) {
	f := Build()
	c := f.Positive[0]
	body, _ := hex.DecodeString(c.BodyHex)
	srv := NewServer(devices(f))
	if v := srv.Check(c.Authorization, c.Method, c.Target, body, c.Meta, f.ServerNow); v != OK {
		t.Fatal(v)
	}
	if v := srv.Check(c.Authorization, c.Method, c.Target, body, c.Meta, f.ServerNow+NonceTTLSeconds+1); v != ClockSkew {
		t.Fatalf("after the nonce expired: %s, want clock_skew (out of window)", v)
	}
}

// A signature made for a different purpose is not valid as an uplink request: the signing
// string's first line is the domain separator.
func TestDomainSeparation(t *testing.T) {
	f := Build()
	seed, priv, id := TestDevice()
	_ = seed
	body := []byte("not a request")
	sig := ed25519.Sign(priv, body) // e.g. something that looks like a manifest
	r := Request{Method: "POST", Target: "/v1/uplink/bundles/offer", Body: body, DeviceID: id, TS: baseTS, Nonce: strings.Repeat("0", 32)}
	if v := NewServer(devices(f)).Check(Header(r, sig), r.Method, r.Target, body, "", baseTS); v != Unauthenticated {
		t.Fatalf("a signature over other bytes was accepted: %s", v)
	}
}

func TestParseHeaderRejectsMalformed(t *testing.T) {
	for _, h := range []string{
		"", "Bearer x", "Cairn-Device device=zz,ts=1,nonce=00,sig=00",
		"Cairn-Device device=" + strings.Repeat("a", 32) + ",ts=01,nonce=" + strings.Repeat("a", 32) + ",sig=" + strings.Repeat("a", 128),
		"Cairn-Device device=" + strings.Repeat("A", 32) + ",ts=1,nonce=" + strings.Repeat("a", 32) + ",sig=" + strings.Repeat("a", 128),
		"Cairn-Device device=" + strings.Repeat("a", 32) + ",device=" + strings.Repeat("a", 32) + ",ts=1,nonce=" + strings.Repeat("a", 32),
	} {
		if _, _, _, _, err := ParseHeader(h); err == nil {
			t.Errorf("accepted %q", h)
		}
	}
}
