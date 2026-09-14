package version

import "testing"

func TestParseToolManifest(t *testing.T) {
	m, err := ParseToolManifest([]byte(`{"schema_version":1,"name":"tool","version":"0.2.0-beta.1","stability":"beta"}`), "tool")
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != "0.2.0-beta.1" || m.Name != "tool" {
		t.Fatalf("manifest=%+v", m)
	}
	for _, raw := range []string{
		`{"schema_version":2,"name":"tool","version":"0.2.0","stability":"beta"}`,
		`{"schema_version":1,"name":"other","version":"0.2.0","stability":"beta"}`,
		`{"schema_version":1,"name":"tool","version":"v0.2.0","stability":"beta"}`,
		`{"schema_version":1,"name":"tool","version":"0.2.0","stability":"unknown"}`,
		`{"schema_version":1,"name":"tool","version":"0.2.0","stability":"beta","extra":true}`,
	} {
		if _, err := ParseToolManifest([]byte(raw), "tool"); err == nil {
			t.Fatalf("expected invalid manifest to fail: %s", raw)
		}
	}
}
