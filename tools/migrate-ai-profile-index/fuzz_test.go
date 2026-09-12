package main

import "testing"

func FuzzParseLegacyNUONNeverPanics(f *testing.F) {
	f.Add(`[[tool,alias,dir,created_at];[claude,personal,/tmp/profile,'2026-01-01T00:00:00']]`)
	f.Add(`[]`)
	f.Add(`[[tool,alias,dir,created_at];]`)
	f.Add(`garbage`)
	f.Fuzz(func(t *testing.T, input string) {
		_, _ = parseLegacyNUON(input)
	})
}
