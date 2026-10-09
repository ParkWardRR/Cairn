package pindash

import (
	"strings"
	"testing"
)

const contracts = `{"contracts":{"repo":"https://x/Cairn","tag":"contracts-v0.1.0","commit":"56d980bd071cc0a9c22b1813af7a4fcd7f68b4f8","protocols":{"format":"v3","ble":"v1"}}}`

func pinnedTo(tag string) []byte {
	return []byte(strings.Replace(contracts, "contracts-v0.1.0", tag, 1))
}

func TestRenderShowsEveryPin(t *testing.T) {
	got, err := Render([]Repo{
		{Name: "server", URL: "https://x/server", Contracts: []byte(contracts), Interop: []byte(`{"firmware":{"commit":"a3fe7ef2dd5e70c5d274ccdacb4b717b70a90dab"}}`)},
		{Name: "web", URL: "https://x/web", Contracts: []byte(contracts), Server: []byte(`{"server":{"commit":"b0deb01d4aa118c4dfd9b3a5bc0b601fb4fa476d"}}`)},
		{Name: "firmware", URL: "https://x/fw", Contracts: []byte(contracts)},
	}, nil)
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
	if _, err := Render([]Repo{{Name: "x"}}, nil); err == nil {
		t.Error("a repository with no contracts.lock must be an error")
	}
	if _, err := Render([]Repo{{Name: "x", Contracts: []byte(`{nope`)}}, nil); err == nil {
		t.Error("broken JSON must be an error")
	}
	if _, err := Render([]Repo{{Name: "x", Contracts: []byte(`{"other":{}}`)}}, nil); err == nil {
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

// The drift column is the whole point of the table being generated: a pin nobody looks at
// is how the web layer sat four releases behind for days.
func TestBehindCountsTheReleasesAfterAPin(t *testing.T) {
	rel := Releases{"contracts-v0.1.0", "contracts-v0.2.0", "contracts-v0.3.0"}
	for _, c := range []struct {
		tag  string
		want int
		ok   bool
	}{
		{"contracts-v0.3.0", 0, true},
		{"contracts-v0.2.0", 1, true},
		{"contracts-v0.1.0", 2, true},
		{"contracts-v9.9.9", 0, false},
	} {
		got, ok := rel.Behind(c.tag)
		if got != c.want || ok != c.ok {
			t.Errorf("Behind(%s) = %d, %v; want %d, %v", c.tag, got, ok, c.want, c.ok)
		}
	}
}

func TestRenderShowsTheDriftWhenTheReleasesAreKnown(t *testing.T) {
	rel := Releases{"contracts-v0.1.0", "contracts-v0.2.0", "contracts-v0.3.0"}
	got, err := Render([]Repo{
		{Name: "stale", URL: "https://x/a", Contracts: pinnedTo("contracts-v0.1.0")},
		{Name: "one-behind", URL: "https://x/b", Contracts: pinnedTo("contracts-v0.2.0")},
		{Name: "current", URL: "https://x/c", Contracts: pinnedTo("contracts-v0.3.0")},
		{Name: "unreleased", URL: "https://x/d", Contracts: pinnedTo("contracts-v0.9.9")},
	}, rel)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "| Behind |") {
		t.Fatalf("no Behind column in:\n%s", got)
	}
	for _, want := range []string{
		"[stale](https://x/a) | `contracts-v0.1.0` (`56d980b`) | **2 releases**",
		"[one-behind](https://x/b) | `contracts-v0.2.0` (`56d980b`) | **1 release**",
		"[current](https://x/c) | `contracts-v0.3.0` (`56d980b`) | current",
		"[unreleased](https://x/d) | `contracts-v0.9.9` (`56d980b`) | **not a release**",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// With no release list the column is omitted rather than filled in with a guess: claiming
// every pin is current is the one wrong answer that would stop anyone looking.
func TestRenderOmitsTheColumnWhenTheReleasesAreUnknown(t *testing.T) {
	got, err := Render([]Repo{{Name: "x", URL: "https://x/x", Contracts: []byte(contracts)}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "Behind") || strings.Contains(got, "current") {
		t.Fatalf("the column should be absent, got:\n%s", got)
	}
}
