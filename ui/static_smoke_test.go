package ui

import (
	"os/exec"
	"strings"
	"testing"
)

func TestStaticAssetsHaveNoExternalRuntimeDependencies(t *testing.T) {
	app, err := staticFiles.ReadFile("static/app.js")
	if err != nil {
		t.Fatal(err)
	}
	index, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		t.Fatal(err)
	}
	combined := string(app) + "\n" + string(index)
	for _, forbidden := range []string{
		"react/jsx-runtime",
		"https://",
		"http://",
		"esm.sh",
		"unpkg.com",
		"cdn.jsdelivr.net",
	} {
		if strings.Contains(combined, forbidden) {
			t.Fatalf("static UI contains forbidden dependency %q", forbidden)
		}
	}
	for _, required := range []string{
		"Overview",
		"Request Preview",
		"Response Preview",
		"Try It Out",
		"Compare Versions",
		"Examples",
	} {
		if !strings.Contains(combined, required) {
			t.Fatalf("static UI missing %q", required)
		}
	}
}

func TestStaticJavaScriptParsesWhenNodeIsAvailable(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}
	cmd := exec.Command("node", "--check", "static/app.js")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("node --check failed: %v\n%s", err, out)
	}
}
