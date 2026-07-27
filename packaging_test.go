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

	configBytes, err := os.ReadFile("wails.json")
	if err != nil {
		t.Fatalf("read wails.json: %v", err)
	}
	var config struct {
		Name           string `json:"name"`
		OutputFilename string `json:"outputfilename"`
		Info           struct {
			ProductName    string `json:"productName"`
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(configBytes, &config); err != nil {
		t.Fatalf("parse wails.json: %v", err)
	}

	if config.Name != "Agent Launcher" {
		t.Errorf("bundle name = %q, want %q", config.Name, "Agent Launcher")
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
	if config.Info.ProductVersion != version {
		t.Errorf(
			"product version = %q, want VERSION %q",
			config.Info.ProductVersion,
			version,
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
}
