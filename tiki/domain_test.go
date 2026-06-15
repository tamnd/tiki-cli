package tiki

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "tiki" {
		t.Errorf("Scheme = %q, want tiki", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "tiki" {
		t.Errorf("Binary = %q, want tiki", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		in      string
		wantTyp string
		wantID  string
		wantErr bool
	}{
		{"https://tiki.vn/iphone-15-pro-max/p54538887.html", "product", "54538887", false},
		{"54538887", "product", "54538887", false},
		{"", "", "", true},
		{"not-a-product-id", "", "", true},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("Classify(%q): want error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("Classify(%q): %v", tc.in, err)
			continue
		}
		if typ != tc.wantTyp || id != tc.wantID {
			t.Errorf("Classify(%q) = (%q,%q), want (%q,%q)", tc.in, typ, id, tc.wantTyp, tc.wantID)
		}
	}
}

func TestLocate(t *testing.T) {
	cases := []struct {
		typ, id, want string
		wantErr       bool
	}{
		{"product", "54538887", "https://tiki.vn/p54538887.html", false},
		{"category", "1789", "https://tiki.vn/c1789", false},
		{"unknown", "x", "", true},
	}
	for _, tc := range cases {
		got, err := Domain{}.Locate(tc.typ, tc.id)
		if tc.wantErr {
			if err == nil {
				t.Errorf("Locate(%q,%q): want error", tc.typ, tc.id)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("Locate(%q,%q) = (%q,%v), want (%q,nil)", tc.typ, tc.id, got, err, tc.want)
		}
	}
}

func TestExtractProductID(t *testing.T) {
	cases := []struct{ url, want string }{
		{"https://tiki.vn/iphone-15-pro-max/p54538887.html", "54538887"},
		{"https://tiki.vn/samsung-galaxy-s24/p12345678.html?ref=abc", "12345678"},
		{"https://tiki.vn/c1789", ""},
	}
	for _, tc := range cases {
		got := extractProductID(tc.url)
		if got != tc.want {
			t.Errorf("extractProductID(%q) = %q, want %q", tc.url, got, tc.want)
		}
	}
}

func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	p := &Product{ID: "54538887", Name: "iPhone 15", URL: "https://tiki.vn/iphone-15/p54538887.html"}
	u, err := h.Mint(p)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if want := "tiki://product/54538887"; u.String() != want {
		t.Errorf("Mint = %q, want %q", u.String(), want)
	}
}
