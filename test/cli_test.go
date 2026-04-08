package test

import (
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"nodeforge/internal/sharelink"
)

func TestConvertCLIAllFormats(t *testing.T) {
	t.Parallel()

	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "nodes.json")
	outputDir := filepath.Join(tempDir, "out")
	input := `{
  "group": "Demo",
  "nodes": [
    {
      "name": "json-vmess",
      "type": "vmess",
      "server": "json1.example.com",
      "port": 443,
      "uuid": "33333333-3333-3333-3333-333333333333",
      "cipher": "auto",
      "tls": true,
      "network": "ws",
      "host": "cdn-json.example.com",
      "path": "/ws"
    },
    {
      "name": "broken-ss",
      "type": "ss",
      "server": "",
      "port": 8388,
      "cipher": "aes-256-gcm"
    }
  ]
}`
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input file: %v", err)
	}

	cmd := exec.Command("go", "run", "./cmd/convert", "-i", inputPath, "-f", "all", "-o", outputDir, "--pretty")
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "summary: total=2 valid=1 failed=1") {
		t.Fatalf("unexpected summary output: %s", output)
	}

	for _, name := range []string{"clash.yaml", "singbox.json", "links.txt", "subscription.txt"} {
		path := filepath.Join(outputDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected output file %s: %v", path, err)
		}
	}
}

func TestConvertDirectoryToV2RayNSubscription(t *testing.T) {
	t.Parallel()

	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	inputDir := t.TempDir()
	outputFile := filepath.Join(t.TempDir(), "subscription.txt")
	configOne := `{
  "inbounds": [{
    "port": 2333,
    "protocol": "vmess",
    "settings": {
      "clients": [{"id": "1xxx", "alterId": 64}]
    }
  }]
}`
	configTwo := `{
  "inbounds": [{
    "port": 2333,
    "protocol": "vmess",
    "settings": {
      "clients": [{"id": "2xxx", "alterId": 64}]
    }
  }]
}`
	if err := os.WriteFile(filepath.Join(inputDir, "1_config.json"), []byte(configOne), 0o644); err != nil {
		t.Fatalf("write first config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inputDir, "2_config.json"), []byte(configTwo), 0o644); err != nil {
		t.Fatalf("write second config: %v", err)
	}

	cmd := exec.Command("go", "run", "./cmd/convert", "-i", inputDir, "-f", "v2rayn", "-o", outputFile, "--server", "demo.example.com")
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "summary: total=2 valid=2 failed=0") {
		t.Fatalf("unexpected summary output: %s", output)
	}

	subscriptionBytes, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("read subscription file: %v", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(subscriptionBytes)))
	if err != nil {
		t.Fatalf("decode subscription: %v", err)
	}
	decodedText := string(decoded)
	if !strings.Contains(decodedText, "vmess://") {
		t.Fatalf("expected vmess links in subscription: %s", decodedText)
	}

	lines := strings.Split(strings.TrimSpace(decodedText), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected two links, got %d: %s", len(lines), decodedText)
	}

	var ids []string
	for _, line := range lines {
		node, err := sharelink.Parse(strings.TrimSpace(line))
		if err != nil {
			t.Fatalf("parse share link %q: %v", line, err)
		}
		ids = append(ids, node.UUID)
		if node.Server != "demo.example.com" {
			t.Fatalf("expected overridden server, got %s", node.Server)
		}
	}
	sort.Strings(ids)
	if ids[0] != "1xxx" || ids[1] != "2xxx" {
		t.Fatalf("unexpected client ids: %#v", ids)
	}
}

func TestConvertDomainNamedVMessWSConfigDefaultsToTLSAndHost(t *testing.T) {
	t.Parallel()

	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	outputFile := filepath.Join(t.TempDir(), "links.txt")
	cmd := exec.Command("go", "run", "./cmd/convert", "-i", "./test/data/3.test-config.example.com.json", "-f", "links", "-o", outputFile)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "summary: total=1 valid=1 failed=0") {
		t.Fatalf("unexpected summary output: %s", output)
	}

	linkBytes, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("read links file: %v", err)
	}
	node, err := sharelink.Parse(strings.TrimSpace(string(linkBytes)))
	if err != nil {
		t.Fatalf("parse generated link: %v", err)
	}
	if node.Server != "3.test-config.example.com" {
		t.Fatalf("expected server from filename, got %s", node.Server)
	}
	if node.Host != "3.test-config.example.com" {
		t.Fatalf("expected ws host from filename, got %s", node.Host)
	}
	if node.Network != "ws" {
		t.Fatalf("expected ws network, got %s", node.Network)
	}
	if !node.TLS {
		t.Fatalf("expected implicit tls to be enabled")
	}
	if node.Port != 443 {
		t.Fatalf("expected implicit tls port 443, got %d", node.Port)
	}
	if node.Path != "/o3ray" {
		t.Fatalf("expected ws path /o3ray, got %s", node.Path)
	}
}

func TestConvertDomainNamedConfigUsesFilenameAsServer(t *testing.T) {
	t.Parallel()

	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	outputFile := filepath.Join(t.TempDir(), "links.txt")
	cmd := exec.Command("go", "run", "./cmd/convert", "-i", "./test/data/4.test-config.example.com.json", "-f", "links", "-o", outputFile)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "summary: total=1 valid=1 failed=0") {
		t.Fatalf("unexpected summary output: %s", output)
	}

	linkBytes, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("read links file: %v", err)
	}
	node, err := sharelink.Parse(strings.TrimSpace(string(linkBytes)))
	if err != nil {
		t.Fatalf("parse generated link: %v", err)
	}
	if node.Server != "4.test-config.example.com" {
		t.Fatalf("expected server from filename, got %s", node.Server)
	}
	if node.Port != 2333 {
		t.Fatalf("expected original port 2333, got %d", node.Port)
	}
	if node.Network != "tcp" {
		t.Fatalf("expected default tcp network, got %s", node.Network)
	}
}

func TestConvertDirectoryUsesFilenameAsServer(t *testing.T) {
	t.Parallel()

	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	inputDir := t.TempDir()
	outputFile := filepath.Join(t.TempDir(), "subscription.txt")
	config := `{
  "inbounds": [{
    "port": 2333,
    "protocol": "vmess",
    "settings": {
      "clients": [{"id": "1xxx", "alterId": 64}]
    }
  }]
}`
	filePath := filepath.Join(inputDir, "edge.example.com.json")
	if err := os.WriteFile(filePath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cmd := exec.Command("go", "run", "./cmd/convert", "-i", inputDir, "-f", "v2rayn", "-o", outputFile, "--server-from-filename")
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "summary: total=1 valid=1 failed=0") {
		t.Fatalf("unexpected summary output: %s", output)
	}

	subscriptionBytes, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("read subscription file: %v", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(subscriptionBytes)))
	if err != nil {
		t.Fatalf("decode subscription: %v", err)
	}
	node, err := sharelink.Parse(strings.TrimSpace(string(decoded)))
	if err != nil {
		t.Fatalf("parse share link: %v", err)
	}
	if node.Server != "edge.example.com" {
		t.Fatalf("expected server from filename, got %s", node.Server)
	}
}

func TestServerFromFilenameConflictsWithServer(t *testing.T) {
	t.Parallel()

	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	outputFile := filepath.Join(t.TempDir(), "subscription.txt")
	cmd := exec.Command("go", "run", "./cmd/convert", "-i", "./test/data", "-f", "v2rayn", "-o", outputFile, "--server", "demo.example.com", "--server-from-filename")
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected conflict error, got success: %s", output)
	}
	if !strings.Contains(string(output), "--server and --server-from-filename cannot be used together") {
		t.Fatalf("unexpected error output: %s", output)
	}
}

func TestServerFromFilenameRequiresDirectoryInput(t *testing.T) {
	t.Parallel()

	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	outputFile := filepath.Join(t.TempDir(), "subscription.txt")
	cmd := exec.Command("go", "run", "./cmd/convert", "-i", "./test/data/1_config.json", "-f", "v2rayn", "-o", outputFile, "--server-from-filename")
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected directory-only error, got success: %s", output)
	}
	if !strings.Contains(string(output), "--server-from-filename requires -i/--input to be a directory") {
		t.Fatalf("unexpected error output: %s", output)
	}
}
