package tapas

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// These tests are offline: they exercise the URI driver's pure string functions
// and the host wiring, which need no network. The client's HTTP behaviour is
// covered in tapas_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "tapas" {
		t.Errorf("Scheme = %q, want tapas", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "tapas" {
		t.Errorf("Identity.Binary = %q, want tapas", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct{ in, typ, id string }{
		{"MATCHPOINT", "series", "MATCHPOINT"},
		{"329873", "series", "329873"},
		{"https://tapas.io/series/MATCHPOINT", "series", "MATCHPOINT"},
		{"https://tapas.io/series/MATCHPOINT/info", "series", "MATCHPOINT"},
		{"https://www.tapas.io/series/LORE-OLYMPUS", "series", "LORE-OLYMPUS"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil || typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q, %v), want (%q, %q, nil)",
				tc.in, typ, id, err, tc.typ, tc.id)
		}
	}
}

func TestLocate(t *testing.T) {
	got, err := Domain{}.Locate("series", "MATCHPOINT")
	want := "https://tapas.io/series/MATCHPOINT/info"
	if err != nil || got != want {
		t.Errorf("Locate = (%q, %v), want (%q, nil)", got, err, want)
	}
}

// TestHostWiring mounts the driver in a kit Host and checks the round trip:
// a record mints to its URI, its body is readable, and a bare id resolves back
// to the same URI.
func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	s := &Series{
		ID:    "329873",
		Slug:  "MATCHPOINT",
		Title: "MATCHPOINT",
		URL:   "https://tapas.io/series/MATCHPOINT/info",
	}
	u, err := h.Mint(s)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if want := "tapas://series/MATCHPOINT"; u.String() != want {
		t.Errorf("Mint = %q, want %q", u.String(), want)
	}

	if body, ok := h.Body(s); !ok || body == "" {
		t.Errorf("Body = (%q, %v), want non-empty", body, ok)
	}

	got, err := h.ResolveOn("tapas", "LORE-OLYMPUS")
	if err != nil || got.String() != "tapas://series/LORE-OLYMPUS" {
		t.Errorf("ResolveOn = (%q, %v), want tapas://series/LORE-OLYMPUS", got.String(), err)
	}
}
