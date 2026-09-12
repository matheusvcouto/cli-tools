package aiprofile

import "testing"

func FuzzValidateAlias(f *testing.F) {
	for _, seed := range []string{"personal", "work_2", "alpha-beta", "", "../escape", "á"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, alias string) {
		err := ValidateAlias(alias)
		valid := alias != ""
		for i := 0; i < len(alias) && valid; i++ {
			c := alias[i]
			valid = (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '-'
		}
		if valid && err != nil {
			t.Fatalf("valid alias %q rejected: %v", alias, err)
		}
		if !valid && err == nil {
			t.Fatalf("invalid alias %q accepted", alias)
		}
	})
}
