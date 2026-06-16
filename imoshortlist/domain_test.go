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
	if info.Identity.Binary != "imoshortlist" {
		t.Errorf("Identity.Binary = %q, want imoshortlist", info.Identity.Binary)
	}
}

func TestDomainClassify_year(t *testing.T) {
	uriType, id, err := Domain{}.Classify("2023")
	if err != nil {
		t.Fatal(err)
	}
	if uriType != "edition" {
		t.Errorf("uriType = %q, want edition", uriType)
	}
	if id != "2023" {
		t.Errorf("id = %q, want 2023", id)
	}
}

func TestDomainClassify_problem(t *testing.T) {
	uriType, id, err := Domain{}.Classify("2023/A3")
	if err != nil {
		t.Fatal(err)
	}
	if uriType != "problem" {
		t.Errorf("uriType = %q, want problem", uriType)
	}
	if id != "2023/A3" {
		t.Errorf("id = %q, want 2023/A3", id)
	}
}

func TestDomainLocate_edition(t *testing.T) {
	url, err := Domain{}.Locate("edition", "2023")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://www.imo-official.org/problems/IMO2023SL.pdf"
	if url != want {
		t.Errorf("Locate = %q, want %q", url, want)
	}
}

func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	got, err := h.ResolveOn("imoshortlist", "list")
	if err == nil {
		t.Logf("ResolveOn returned %q", got)
	}
}
