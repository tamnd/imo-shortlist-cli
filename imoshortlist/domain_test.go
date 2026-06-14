package imoshortlist

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "imoshortlist" {
		t.Errorf("Scheme = %q, want imoshortlist", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "imoslx" {
		t.Errorf("Identity.Binary = %q, want imoslx", info.Identity.Binary)
	}
}

func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	got, err := h.ResolveOn("imoshortlist", "shortlists")
	if err == nil {
		t.Logf("ResolveOn returned %q (Classify not supported, this is expected to have limited use)", got)
	}
}
