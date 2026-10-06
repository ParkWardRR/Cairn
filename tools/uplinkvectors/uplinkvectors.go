// Package uplinkvectors produces and checks the test vectors for contracts/uplink/v1, and is
// the reference implementation of the request-signing rules in its spec (sections 2.1, 2.2).
//
// Every key here is derived from a published label and protects nothing. Ed25519 is
// deterministic, so every signature in the vectors is "must equal".
package uplinkvectors

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ClockWindowSeconds is how far a request's ts may be from the server's clock.
const ClockWindowSeconds = 120

// NonceTTLSeconds is how long the server remembers (device, nonce).
const NonceTTLSeconds = 240

// Request is the part of an HTTP request the signature covers.
type Request struct {
	Method   string
	Target   string
	Body     []byte
	DeviceID string // 32 lowercase hex
	TS       int64
	Nonce    string // 32 lowercase hex
	Meta     string // value of Cairn-Meta, or ""
}

// SigningString is spec section 2.1: eight lines, no trailing newline.
func SigningString(r Request) string {
	sum := sha256.Sum256(r.Body)
	return strings.Join([]string{
		"CAIRN-UPLINK-V1",
		strings.ToUpper(r.Method),
		r.Target,
		strconv.FormatInt(r.TS, 10),
		r.Nonce,
		hex.EncodeToString(sum[:]),
		r.DeviceID,
		r.Meta,
	}, "\n")
}

// Header is the Authorization header value for a signature.
func Header(r Request, sig []byte) string {
	return fmt.Sprintf("Cairn-Device device=%s,ts=%d,nonce=%s,sig=%s", r.DeviceID, r.TS, r.Nonce, hex.EncodeToString(sig))
}

// ParseHeader reads an Authorization header back into its fields.
func ParseHeader(h string) (device string, ts int64, nonce string, sig []byte, err error) {
	rest, ok := strings.CutPrefix(h, "Cairn-Device ")
	if !ok {
		return "", 0, "", nil, fmt.Errorf("not a Cairn-Device header")
	}
	kv := map[string]string{}
	for _, p := range strings.Split(rest, ",") {
		k, v, ok := strings.Cut(p, "=")
		if !ok {
			return "", 0, "", nil, fmt.Errorf("malformed field %q", p)
		}
		if _, dup := kv[k]; dup {
			return "", 0, "", nil, fmt.Errorf("duplicate field %q", k)
		}
		kv[k] = v
	}
	if len(kv) != 4 {
		return "", 0, "", nil, fmt.Errorf("want device, ts, nonce, sig")
	}
	device, nonce = kv["device"], kv["nonce"]
	if !isLowerHex(device, 32) || !isLowerHex(nonce, 32) || !isLowerHex(kv["sig"], 128) {
		return "", 0, "", nil, fmt.Errorf("device, nonce and sig must be lowercase hex of 32, 32 and 128 characters")
	}
	if ts, err = strconv.ParseInt(kv["ts"], 10, 64); err != nil || strconv.FormatInt(ts, 10) != kv["ts"] || ts < 0 {
		return "", 0, "", nil, fmt.Errorf("ts must be a plain non-negative decimal")
	}
	sig, _ = hex.DecodeString(kv["sig"])
	return device, ts, nonce, sig, nil
}

func isLowerHex(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// Verdicts a server gives (spec section 2.2).
const (
	OK              = "ok"
	Unauthenticated = "unauthenticated"
	ClockSkew       = "clock_skew"
)

// Server is the reference check of spec section 2.2, with only the state it needs.
type Server struct {
	Devices map[string]ed25519.PublicKey // enrolled, not revoked: device_id hex -> key
	seen    map[string]int64             // device|nonce -> expiry (unix seconds)
}

// NewServer returns a server that knows the given devices.
func NewServer(devices map[string]ed25519.PublicKey) *Server {
	return &Server{Devices: devices, seen: map[string]int64{}}
}

// Check applies section 2.2 to one request carrying header h, at server time now.
// Unknown device, bad signature and replay are all the same verdict, on purpose.
func (s *Server) Check(h string, method, target string, body []byte, meta string, now int64) string {
	device, ts, nonce, sig, err := ParseHeader(h)
	if err != nil {
		return Unauthenticated
	}
	pub, known := s.Devices[device]
	req := Request{Method: method, Target: target, Body: body, DeviceID: device, TS: ts, Nonce: nonce, Meta: meta}
	sigOK := known && ed25519.Verify(pub, []byte(SigningString(req)), sig)
	for k, exp := range s.seen { // forget expired nonces
		if exp <= now {
			delete(s.seen, k)
		}
	}
	key := device + "|" + nonce
	if !sigOK {
		return Unauthenticated
	}
	if _, replay := s.seen[key]; replay {
		return Unauthenticated
	}
	if d := now - ts; d > ClockWindowSeconds || d < -ClockWindowSeconds {
		return ClockSkew
	}
	s.seen[key] = now + NonceTTLSeconds
	return OK
}

// ---- the vectors ---------------------------------------------------------------------

type PositiveCase struct {
	Name          string `json:"name"`
	Method        string `json:"method"`
	Target        string `json:"target"`
	BodyHex       string `json:"body_hex"`
	Meta          string `json:"meta"`
	TS            int64  `json:"ts"`
	Nonce         string `json:"nonce"`
	BodySHA256    string `json:"body_sha256"`
	SigningString string `json:"signing_string"`
	Signature     string `json:"signature"`
	Authorization string `json:"authorization"`
}

type NegativeCase struct {
	Name        string `json:"name"`
	Base        string `json:"base"`
	Mutation    string `json:"mutation"`
	ServerNow   int64  `json:"server_now"`
	DeliverTwo  bool   `json:"deliver_twice,omitempty"`
	WantVerdict string `json:"want"`
	// WantSecond is the verdict of the second delivery when DeliverTwo is set.
	WantSecond string `json:"want_second,omitempty"`
}

type File struct {
	Description     string         `json:"description"`
	DeviceSeed      string         `json:"device_seed"`
	DevicePublicKey string         `json:"device_public_key"`
	DeviceID        string         `json:"device_id"`
	ServerNow       int64          `json:"server_now"`
	Positive        []PositiveCase `json:"positive"`
	Negative        []NegativeCase `json:"negative"`
}

func label(s string) []byte { h := sha256.Sum256([]byte(s)); return h[:] }

// TestDevice returns the public test key and id the vectors use.
func TestDevice() (seed []byte, priv ed25519.PrivateKey, deviceID string) {
	seed = label("cairn/uplink-v1/test-device-seed")
	priv = ed25519.NewKeyFromSeed(seed)
	deviceID = hex.EncodeToString(label("cairn/uplink-v1/test-device-id")[:16])
	return
}

const baseTS = 1790000000

func pattern(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*7 + 3)
	}
	return b
}

// Build produces the whole vectors file.
func Build() File {
	seed, priv, id := TestDevice()
	manifest := pattern(180) // a stand-in for manifest.cbor: the protocol signs bytes, not meaning
	chunk := pattern(1000)
	csum := sha256.Sum256(chunk)
	bundle := hex.EncodeToString(label("cairn/uplink-v1/test-bundle")[:16])

	type spec struct {
		name, method, target string
		body                 []byte
		meta                 string
		dt                   int64
	}
	specs := []spec{
		{"offer, over Wi-Fi in slot 3", "POST", "/v1/uplink/bundles/offer", manifest, "path=wifi;slot=3", 0},
		{"chunk upload, over LTE", "PUT", "/v1/uplink/bundles/" + bundle + "/chunks/" + hex.EncodeToString(csum[:]), chunk, "path=lte", 1},
		{"commit, no body and no meta", "POST", "/v1/uplink/bundles/" + bundle + "/commit", nil, "", 2},
		{"receipt fetch with a query-less GET", "GET", "/v1/uplink/bundles/" + bundle + "/receipt", nil, "path=wifi;slot=4", 3},
	}
	f := File{
		Description: "Cairn device uplink v1 request signing (DRAFT). Every key here is a PUBLIC TEST KEY derived from a published label; none protects anything. Signatures are deterministic (Ed25519): must equal.",
		DeviceSeed:  hex.EncodeToString(seed), DevicePublicKey: hex.EncodeToString(priv.Public().(ed25519.PublicKey)),
		DeviceID: id, ServerNow: baseTS + 5,
	}
	for i, sp := range specs {
		r := Request{Method: sp.method, Target: sp.target, Body: sp.body, DeviceID: id, TS: baseTS + sp.dt,
			Nonce: hex.EncodeToString(label(fmt.Sprintf("cairn/uplink-v1/nonce/%d", i))[:16]), Meta: sp.meta}
		ss := SigningString(r)
		sig := ed25519.Sign(priv, []byte(ss))
		sum := sha256.Sum256(sp.body)
		f.Positive = append(f.Positive, PositiveCase{
			Name: sp.name, Method: r.Method, Target: r.Target, BodyHex: hex.EncodeToString(r.Body), Meta: r.Meta,
			TS: r.TS, Nonce: r.Nonce, BodySHA256: hex.EncodeToString(sum[:]), SigningString: ss,
			Signature: hex.EncodeToString(sig), Authorization: Header(r, sig),
		})
	}
	o := f.Positive[0].Name
	f.Negative = []NegativeCase{
		{Name: "meta altered after signing", Base: o, Mutation: "meta=path=lte;slot=3", ServerNow: f.ServerNow, WantVerdict: Unauthenticated},
		{Name: "target altered after signing", Base: o, Mutation: "target=/v1/uplink/bundles/" + bundle + "/commit", ServerNow: f.ServerNow, WantVerdict: Unauthenticated},
		{Name: "method altered after signing", Base: o, Mutation: "method=PUT", ServerNow: f.ServerNow, WantVerdict: Unauthenticated},
		{Name: "body altered after signing", Base: o, Mutation: "body_flip_first_byte", ServerNow: f.ServerNow, WantVerdict: Unauthenticated},
		{Name: "a different device id in the header", Base: o, Mutation: "device=" + hex.EncodeToString(label("cairn/uplink-v1/other-device")[:16]), ServerNow: f.ServerNow, WantVerdict: Unauthenticated},
		{Name: "device not enrolled", Base: o, Mutation: "device_unenrolled", ServerNow: f.ServerNow, WantVerdict: Unauthenticated},
		{Name: "signature with one bit flipped", Base: o, Mutation: "sig_flip_bit", ServerNow: f.ServerNow, WantVerdict: Unauthenticated},
		{Name: "valid signature, server clock 121 s ahead", Base: o, Mutation: "none", ServerNow: baseTS + ClockWindowSeconds + 1, WantVerdict: ClockSkew},
		{Name: "valid signature, server clock 121 s behind", Base: o, Mutation: "none", ServerNow: baseTS - ClockWindowSeconds - 1, WantVerdict: ClockSkew},
		{Name: "valid signature, exactly 120 s skew is accepted", Base: o, Mutation: "none", ServerNow: baseTS + ClockWindowSeconds, WantVerdict: OK},
		{Name: "the same request delivered twice", Base: o, Mutation: "none", ServerNow: f.ServerNow, DeliverTwo: true, WantVerdict: OK, WantSecond: Unauthenticated},
	}
	return f
}

// JSON is the canonical serialisation of the vectors file (what is checked in).
func JSON(f File) []byte {
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		panic(err)
	}
	return append(b, '\n')
}
