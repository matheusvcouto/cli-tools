//go:build windows

package aiprofile

import "testing"

func TestProfileDirIdentityWindowsCaseFold(t *testing.T) {
	first := profileDirIdentity(`C:\Users\Example\.ai-profiles\grok-aa`)
	second := profileDirIdentity(`c:\users\example\.AI-PROFILES\GROK-AA`)
	if first != second {
		t.Fatalf("Windows case aliases were distinct: %q vs %q", first, second)
	}
}
