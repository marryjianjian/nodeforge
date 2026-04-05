package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseYAMLDocument(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "nodes.yaml")
	content := `
group: Demo
nodes:
  - name: hk-vmess
    type: vmess
    server: hk.example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    cipher: auto
    tls: true
    network: ws
    path: /vmess
`
	if err := os.WriteFile(inputPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write yaml input: %v", err)
	}

	result, err := ParseFile(inputPath, Options{})
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}
	if result.SourceGroup != "Demo" {
		t.Fatalf("unexpected group: %s", result.SourceGroup)
	}
	if len(result.Nodes) != 1 {
		t.Fatalf("unexpected node count: %d", len(result.Nodes))
	}
	if result.Nodes[0].Server != "hk.example.com" {
		t.Fatalf("unexpected server: %s", result.Nodes[0].Server)
	}
}

func TestParseLinksFileSkipsInvalidEntries(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "links.txt")
	content := `
vless://22222222-2222-2222-2222-222222222222@jp1.example.com:8443?encryption=none&security=tls&sni=jp1.example.com&type=grpc&serviceName=grpc#jp-vless-grpc
not-a-link
`
	if err := os.WriteFile(inputPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write links input: %v", err)
	}

	result, err := ParseFile(inputPath, Options{})
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}
	if len(result.Nodes) != 1 {
		t.Fatalf("unexpected valid node count: %d", len(result.Nodes))
	}
	if len(result.Errors) != 1 {
		t.Fatalf("unexpected parse error count: %d", len(result.Errors))
	}
}

func TestParseDirectoryWithV2RayConfigs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	fileOne := filepath.Join(dir, "1_config.json")
	fileTwo := filepath.Join(dir, "2_config.json")
	contentOne := `{
  "inbounds": [{
    "port": 2333,
    "protocol": "vmess",
    "settings": {
      "clients": [{"id": "1xxx", "alterId": 64}]
    }
  }]
}`
	contentTwo := `{
  "inbounds": [{
    "port": 2444,
    "protocol": "vmess",
    "settings": {
      "clients": [{"id": "2xxx", "alterId": 64}]
    }
  }]
}`
	if err := os.WriteFile(fileOne, []byte(contentOne), 0o644); err != nil {
		t.Fatalf("write fileOne: %v", err)
	}
	if err := os.WriteFile(fileTwo, []byte(contentTwo), 0o644); err != nil {
		t.Fatalf("write fileTwo: %v", err)
	}

	result, err := ParsePath(dir, Options{DefaultServer: "example.com"})
	if err != nil {
		t.Fatalf("ParsePath returned error: %v", err)
	}
	if len(result.Nodes) != 2 {
		t.Fatalf("unexpected node count: %d", len(result.Nodes))
	}
	if result.Nodes[0].Server != "example.com" {
		t.Fatalf("unexpected default server: %s", result.Nodes[0].Server)
	}
	if result.Nodes[0].UUID != "1xxx" || result.Nodes[1].UUID != "2xxx" {
		t.Fatalf("unexpected uuids: %#v", result.Nodes)
	}
}
