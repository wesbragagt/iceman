package nixsearch

import "testing"

func TestFuzzyMatchNoMatch(t *testing.T) {
	for _, tc := range []struct{ query, target string }{
		{"zzz", "ripgrep"},
		{"pgri", "ripgrep"},
		{"gooo", "go"},
	} {
		if score, ok := FuzzyMatch(tc.query, tc.target); ok {
			t.Errorf("FuzzyMatch(%q, %q) = %d, true; want ok=false", tc.query, tc.target, score)
		}
	}
}

func TestFuzzyMatchCaseInsensitive(t *testing.T) {
	if _, ok := FuzzyMatch("RIP", "ripgrep"); !ok {
		t.Fatal("expected case-insensitive match")
	}
}

func TestFuzzyMatchExactBeatsFuzzy(t *testing.T) {
	exact, ok := FuzzyMatch("go", "go")
	if !ok {
		t.Fatal("exact match should match")
	}
	fuzzy, ok := FuzzyMatch("go", "gopls")
	if !ok {
		t.Fatal("prefix match should match")
	}
	if exact <= fuzzy {
		t.Errorf("exact %d should beat fuzzy %d", exact, fuzzy)
	}
}

func TestFuzzyMatchContiguousBeatsScattered(t *testing.T) {
	contig, ok := FuzzyMatch("grep", "grepper")
	if !ok {
		t.Fatal("contiguous should match")
	}
	scattered, ok := FuzzyMatch("grep", "gaarbage-rich-elephant-pack")
	if !ok {
		t.Fatal("scattered should match")
	}
	if contig <= scattered {
		t.Errorf("contiguous %d should beat scattered %d", contig, scattered)
	}
}

func TestFuzzyMatchEmptyQuery(t *testing.T) {
	if score, ok := FuzzyMatch("", "anything"); !ok || score != 0 {
		t.Errorf("empty query = %d, %v; want 0, true", score, ok)
	}
}

func TestMatchPositions(t *testing.T) {
	if got := MatchPositions("", "ripgrep"); got != nil {
		t.Errorf("empty query = %v; want nil", got)
	}
	if got := MatchPositions("zzz", "ripgrep"); got != nil {
		t.Errorf("no-match = %v; want nil", got)
	}
	got := MatchPositions("rgp", "ripgrep")
	want := []int{0, 3, 6}
	if len(got) != len(want) {
		t.Fatalf("MatchPositions = %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("MatchPositions = %v; want %v", got, want)
		}
	}
}

func testPackages() []Package {
	return []Package{
		{AttrPath: "legacyPackages.x86_64-linux.ripgrep", PName: "ripgrep", Description: "fast line-oriented search tool"},
		{AttrPath: "legacyPackages.x86_64-linux.ripgrep-all", PName: "ripgrep-all", Description: "ripgrep for pdfs and more"},
		{AttrPath: "legacyPackages.x86_64-linux.fd", PName: "fd", Description: "simple alternative to find"},
		{AttrPath: "legacyPackages.x86_64-linux.silver-searcher", PName: "ag", Description: "a code searching tool similar to ack, but faster (ripgrep rival)"},
		{AttrPath: "legacyPackages.x86_64-linux.go", PName: "go", Description: "the Go programming language"},
	}
}

func TestRankPackagesNameBeatsDescription(t *testing.T) {
	got := RankPackages("ripgrep", testPackages(), 0)
	if len(got) < 3 {
		t.Fatalf("expected at least 3 matches, got %d", len(got))
	}
	if got[0].PName != "ripgrep" {
		t.Errorf("first = %q; want ripgrep", got[0].PName)
	}
	if got[1].PName != "ripgrep-all" {
		t.Errorf("second = %q; want ripgrep-all", got[1].PName)
	}
	// `ag` only matches in its description, so it must rank below both names.
	last := got[len(got)-1]
	if last.PName != "ag" {
		t.Errorf("description-only match should rank last, got %q", last.PName)
	}
}

func TestRankPackagesLimit(t *testing.T) {
	if got := RankPackages("r", testPackages(), 2); len(got) != 2 {
		t.Errorf("limit 2 returned %d results", len(got))
	}
	if got := RankPackages("", testPackages(), 0); len(got) != 5 {
		t.Errorf("empty query returned %d results; want all 5", len(got))
	}
}

func TestRankPackagesExcludesNonMatches(t *testing.T) {
	for _, p := range RankPackages("ripgrep", testPackages(), 0) {
		if p.PName == "fd" {
			t.Fatal("fd should not match query 'ripgrep'")
		}
	}
}

func TestRankPackagesStable(t *testing.T) {
	first := RankPackages("re", testPackages(), 0)
	for i := 0; i < 5; i++ {
		got := RankPackages("re", testPackages(), 0)
		if len(got) != len(first) {
			t.Fatal("unstable result length")
		}
		for j := range got {
			if got[j].AttrPath != first[j].AttrPath {
				t.Fatalf("unstable order at %d: %q vs %q", j, got[j].AttrPath, first[j].AttrPath)
			}
		}
	}
}

func TestInstallNameStripsSystemPrefix(t *testing.T) {
	p := Package{AttrPath: "legacyPackages.x86_64-linux.python3Packages.requests"}
	if got := p.InstallName(); got != "python3Packages.requests" {
		t.Errorf("InstallName() = %q", got)
	}
	plain := Package{AttrPath: "ripgrep"}
	if got := plain.InstallName(); got != "ripgrep" {
		t.Errorf("InstallName() = %q", got)
	}
}
