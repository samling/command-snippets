package theme

import "testing"

func TestTransparentSurfaceOverrides(t *testing.T) {
	for role := range defaults {
		err := Validate("catppuccin-mocha", map[string]string{string(role): "transparent"})
		if (err == nil) != (role == Background || role == Panel) {
			t.Errorf("transparent role %s: %v", role, err)
		}
	}
	for _, value := range []string{"Transparent", "", "none", "#12345g"} {
		if Validate("catppuccin-mocha", map[string]string{"background": value}) == nil {
			t.Errorf("accepted invalid surface color %q", value)
		}
	}
	p := Resolve("catppuccin-mocha", map[string]string{"background": "transparent", "panel": "transparent"})
	if p[Background] != "" || p[Panel] != "" || p[Selection] != "#45475a" || p[Text] != "#cdd6f4" {
		t.Fatalf("transparent surfaces changed other Mocha roles: %v", p)
	}
	if p := Resolve("catppuccin-mocha", nil); p[Background] != "#1e1e2e" || p[Panel] != "#181825" {
		t.Fatal("omitted overrides changed opaque Mocha defaults")
	}
	if p := Resolve("default", nil); p[Background] != "" || p[Panel] != "" {
		t.Fatal("default surfaces changed")
	}
}
