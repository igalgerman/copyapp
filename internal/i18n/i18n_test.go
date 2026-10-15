package i18n

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

var placeholder = regexp.MustCompile(`%[-+# 0-9.]*[a-zA-Z]|\{[a-zA-Z]+\}`)

func places(s string) []string {
	p := placeholder.FindAllString(s, -1)
	slices.Sort(p)
	return p
}

// Every key exists in every language with the same placeholders.
func TestTableComplete(t *testing.T) {
	if len(table) != len(Langs) {
		t.Fatalf("table has %d languages, Langs has %d", len(table), len(Langs))
	}
	for _, l := range Langs {
		for _, other := range Langs {
			for k, v := range table[l] {
				w, ok := table[other][k]
				if !ok {
					t.Errorf("%s: key %q missing (present in %s)", other, k, l)
					continue
				}
				if !slices.Equal(places(v), places(w)) {
					t.Errorf("%q: placeholders differ: %s %v, %s %v", k, l, places(v), other, places(w))
				}
				if strings.TrimSpace(w) == "" {
					t.Errorf("%s: %q is empty", other, k)
				}
			}
		}
	}
}

func TestSetLoadSave(t *testing.T) {
	defer Set(Default)
	dir := t.TempDir()
	Load(dir)
	if Lang() != "he" {
		t.Fatalf("default %q", Lang())
	}
	if Set("xx") || Lang() != "he" {
		t.Fatal("unknown language accepted")
	}
	Set("ru")
	if err := Save(dir); err != nil {
		t.Fatal(err)
	}
	Set("en")
	Load(dir)
	if Lang() != "ru" {
		t.Fatalf("loaded %q", Lang())
	}
	if got := In("en", "toast.savedMore", "a.txt", 2); got != "\u2068a.txt\u2069 and 2 more" { // file names are bidi-isolated
		t.Fatal(got)
	}
	if In("en", "no.such.key") != "no.such.key" || Dir("he") != "rtl" || Dir("ru") != "ltr" {
		t.Fatal("fallbacks")
	}
}
