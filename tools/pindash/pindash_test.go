package pindash

import (
	"strings"
	"testing"
)

const contracts = `{"contracts":{"repo":"https://x/Cairn","tag":"contracts-v0.1.0","commit":"56d980bd071cc0a9c22b1813af7a4fcd7f68b4f8","protocols":{"format":"v3","ble":"v1"}}}`

func TestRenderShowsEveryPin(t *testing.T) {
	got, err := Render([]Repo{
		{Name: "server", URL: "https://x/server", Contracts: []byte(contracts), Interop: []byte(`{"firmware":{"commit":"a3fe7ef2dd5e70c5d274ccdacb4b717b70a90dab"}}`)},
		{Name: "web", URL: "https://x/web", Contracts: []byte(contracts), Server: []byte(`{"server":{"commit":"b0deb01d4aa118c4dfd9b3a5bc0b601fb4fa476d"}}`)},
		{Name: "firmware", URL: "https://x/fw", Contracts: []byte(contracts)},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"[server](https://x/server) | `contracts-v0.1.0` (`56d980b`) | ble v1, format v3 | firmware `a3fe7ef` (interop)",
		"[web](https://x/web) | `contracts-v0.1.0` (`56d980b`) | ble v1, format v3 | vehicle server `b0deb01`",
		"[firmware](https://x/fw) | `contracts-v0.1.0` (`56d980b`) | ble v1, format v3 | none",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing row %q in:\n%s", want, got)
		}
	}
}

func TestRenderRefusesAMissingOrBrokenLock(t *testing.T) {
	if _, err := Render([]Repo{{Name: "x"}}); err == nil {
		t.Error("a repository with no contracts.lock must be an error")
	}
	if _, err := Render([]Repo{{Name: "x", Contracts: []byte(`{nope`)}}); err == nil {
		t.Error("broken JSON must be an error")
	}
	if _, err := Render([]Repo{{Name: "x", Contracts: []byte(`{"other":{}}`)}}); err == nil {
		t.Error("a lock without the contracts object must be an error")
	}
}

func TestReplaceOnlyTouchesTheBlock(t *testing.T) {
	doc := "before\n" + Start + "\nold\n" + End + "\nafter\n"
	out, changed, err := Replace(doc, "NEW\n")
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	if out != "before\n"+Start+"\nNEW\n"+End+"\nafter\n" {
		t.Fatalf("got %q", out)
	}
	if _, changed, _ := Replace(out, "NEW\n"); changed {
		t.Error("replacing with the same table must report no change")
	}
	if _, _, err := Replace("no markers", "x"); err == nil {
		t.Error("a document without the markers must be an error")
	}
}
