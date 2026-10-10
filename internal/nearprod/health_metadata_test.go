package nearprod

import "testing"

func TestContainerHealthConfigurationEvidence(t *testing.T) {
	for _, tc := range []struct {
		name       string
		check      J
		health     string
		configured bool
	}{
		{"absent", nil, "", false},
		{"disabled", J{"Test": A{"NONE"}}, "", false},
		{"configured-without-result", J{"Test": A{"CMD", "check"}}, "", true},
		{"configured-healthy", J{"Test": A{"CMD-SHELL", "check"}}, "healthy", true},
		{"observed-result", nil, "unhealthy", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := J{"Config": J{}, "State": J{"Running": true, "Status": "running"}}
			if tc.check != nil {
				obj(raw["Config"])["Healthcheck"] = tc.check
			}
			if tc.health != "" {
				obj(raw["State"])["Health"] = J{"Status": tc.health}
			}
			got := summarizeContainer(raw)
			if truth(got["hasHealthcheck"]) != tc.configured {
				t.Fatalf("configuration: %v", got)
			}
			want := tc.health
			if want == "" {
				want = "unchecked"
			}
			if str(got["health"]) != want {
				t.Fatalf("health: %v", got)
			}
		})
	}
}
