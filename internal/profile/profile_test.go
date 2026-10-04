package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPersistenceAndPersonalBests(t *testing.T) {
	s := Store{filepath.Join(t.TempDir(), "nested", "profile.json")}
	p, err := s.Load()
	if err != nil || p.Language != "en" || p.Theme != "mono" || p.Share {
		t.Fatal(p, err)
	}
	p.Language = "tr"
	p.Theme = "midnight"
	p.Username = "q_w7"
	p.Identity = Identity{ID: "id", Token: "secret", Endpoint: "https://scores.example"}
	if !p.Record("snake", 0) || !p.Record("snake", 100) || p.Record("snake", 90) || p.Record("snake", 100) || p.Record("snake", 101) || p.Record("bad", 99) {
		t.Fatal("best policy")
	}
	if err = s.Save(p); err != nil {
		t.Fatal(err)
	}
	q, err := s.Load()
	if err != nil || q.Best["snake"] != 100 || q.Identity.Token != "secret" || q.Language != "tr" {
		t.Fatal(q, err)
	}
	q.Best["snake"] = 0
	if p.Best["snake"] != 100 {
		t.Fatal("load aliased bests")
	}
	r := p.Clone()
	r.Best["snake"] = 0
	if p.Best["snake"] != 100 {
		t.Fatal("snapshot aliases bests")
	}
	for _, n := range []int{-1, 1751, 1000000001} {
		if ValidScore("terminalfc", n) {
			t.Error("terminalfc", n)
		}
	}
	for _, n := range []int{0, 500, 1000, 1750} {
		if !ValidScore("terminalfc", n) {
			t.Error("terminalfc valid", n)
		}
	}
}
func TestCorruptProfileIsNotSilentlyOverwritten(t *testing.T) {
	s := Store{filepath.Join(t.TempDir(), "profile.json")}
	original := []byte(`{"version":99,"username":"old"}`)
	if err := os.WriteFile(s.Path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); err == nil {
		t.Fatal("unknown version accepted")
	}
	b, _ := os.ReadFile(s.Path)
	if string(b) != string(original) {
		t.Fatal("load replaced data")
	}
}
func TestUsernameAndScoreBounds(t *testing.T) {
	for _, s := range []string{"abc", "q_w7", "01234567890123456789"} {
		if !ValidUsername(s) {
			t.Error(s)
		}
	}
	for _, s := range []string{"ab", "Aname", "üser", "../foo", "foo\n", "012345678901234567890"} {
		if ValidUsername(s) {
			t.Error(s)
		}
	}
	for _, n := range []int{-1, 6451, 6460, 1000000001} {
		if ValidScore("snake", n) {
			t.Error(n)
		}
	}
}
