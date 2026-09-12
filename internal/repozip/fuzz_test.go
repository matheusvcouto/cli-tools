package repozip

import (
	"bytes"
	"strings"
	"testing"
)

func FuzzValidateSuffix(f *testing.F) {
	for _, seed := range []string{"v1", "2026.09", "release_1", "", "../x", " bad", "-bad"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		got, err := validateSuffix(value, "--suffix")
		if err != nil {
			return
		}
		if got != value || got == "" || strings.ContainsAny(got, `/\\`) {
			t.Fatalf("unsafe normalized suffix %q -> %q", value, got)
		}
		for i, r := range got {
			valid := (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
			if !valid || (i == 0 && (r == '.' || r == '_' || r == '-')) {
				t.Fatalf("invalid suffix accepted: %q", got)
			}
		}
	})
}

func FuzzVerifyZipNeverPanics(f *testing.F) {
	f.Add([]byte("not a zip"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, raw []byte) {
		reader := bytes.NewReader(raw)
		_ = (Archiver{}).Verify(reader, int64(len(raw)))
	})
}
