package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestMacOSPackageMetadata(t *testing.T) {
	versionBytes, err := os.ReadFile("VERSION")
	if err != nil {
		t.Fatalf("read VERSION: %v", err)
	}
	version := strings.TrimSpace(string(versionBytes))
	if version == "" {
		t.Fatal("VERSION is empty")
	}

	configBytes, err := os.ReadFile("wails.json")
	if err != nil {
		t.Fatalf("read wails.json: %v", err)
	}
	var config struct {
		Name           string `json:"name"`
		OutputFilename string `json:"outputfilename"`
		Info           struct {
			ProductName    string  `json:"productName"`
			ProductVersion *string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(configBytes, &config); err != nil {
		t.Fatalf("parse wails.json: %v", err)
	}

	if config.Name != "Launcher" {
		t.Errorf("bundle name = %q, want %q", config.Name, "Launcher")
	}
	if config.OutputFilename != "launcher" {
		t.Errorf("bundle executable = %q, want %q", config.OutputFilename, "launcher")
	}
	if config.Info.ProductName != config.Name {
		t.Errorf(
			"product name = %q, want bundle name %q",
			config.Info.ProductName,
			config.Name,
		)
	}
	if config.Info.ProductVersion != nil {
		t.Errorf(
			"wails.json product version = %q, want VERSION as the only source",
			*config.Info.ProductVersion,
		)
	}

	plistBytes, err := os.ReadFile("build/darwin/Info.plist")
	if err != nil {
		t.Fatalf("read macOS Info.plist template: %v", err)
	}
	plist := string(plistBytes)
	for _, required := range []string{
		"<string>APPL</string>",
		"<string>{{.OutputFilename}}</string>",
		"<string>com.pdparchitect.launcher</string>",
		"<string>iconfile</string>",
	} {
		if !strings.Contains(plist, required) {
			t.Errorf("Info.plist is missing %q", required)
		}
	}

	makefileBytes, err := os.ReadFile("Makefile")
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}
	makefile := string(makefileBytes)
	for _, required := range []string{
		`plutil -replace CFBundleVersion -string "$(VERSION)"`,
		`plutil -replace CFBundleShortVersionString -string "$(VERSION)"`,
		"codesign --force --deep --sign - build/bin/Launcher.app",
	} {
		if !strings.Contains(makefile, required) {
			t.Errorf("Makefile is missing %q", required)
		}
	}
}

func TestMacOSReleaseWorkflow(t *testing.T) {
	workflowBytes, err := os.ReadFile(".github/workflows/release.yaml")
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}
	workflow := string(workflowBytes)

	for _, required := range []string{
		"environment: release",
		"MACOS_CERTIFICATE_P12_BASE64: ${{ secrets.MACOS_CERTIFICATE_P12_BASE64 }}",
		"MACOS_CERTIFICATE_PASSWORD: ${{ secrets.MACOS_CERTIFICATE_PASSWORD }}",
		"APPLE_NOTARY_KEY_P8_BASE64: ${{ secrets.APPLE_NOTARY_KEY_P8_BASE64 }}",
		"APPLE_NOTARY_KEY_ID: ${{ vars.APPLE_NOTARY_KEY_ID }}",
		"APPLE_NOTARY_ISSUER_ID: ${{ vars.APPLE_NOTARY_ISSUER_ID }}",
		"APPLE_TEAM_ID: ${{ vars.APPLE_TEAM_ID }}",
		"Developer ID Application:",
		"--options runtime",
		"--timestamp",
		"notarytool submit",
		"stapler staple",
		"spctl --assess",
		"gh release create",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("release workflow is missing %q", required)
		}
	}

	for _, forbidden := range []string{
		"secrets.APPLE_NOTARY_KEY_ID",
		"secrets.APPLE_NOTARY_ISSUER_ID",
		"secrets.APPLE_TEAM_ID",
	} {
		if strings.Contains(workflow, forbidden) {
			t.Errorf("release workflow should not use %q", forbidden)
		}
	}
}
