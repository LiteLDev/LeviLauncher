package txn

import (
	"regexp"
	"testing"
)

func TestNewGUID(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{12}4[0-9a-f]{3}[89ab][0-9a-f]{15}$`)
	seen := map[string]bool{}
	for range 100 {
		g := NewGUID()
		if !re.MatchString(g) {
			t.Fatalf("NewGUID() = %q", g)
		}
		if seen[g] {
			t.Fatalf("NewGUID() repeated %q", g)
		}
		seen[g] = true
	}
}
