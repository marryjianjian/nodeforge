package sharelink

import (
	"strings"
	"testing"

	"nodeforge/internal/model"
)

func TestParseVLESSLink(t *testing.T) {
	t.Parallel()

	link := "vless://22222222-2222-2222-2222-222222222222@jp1.example.com:8443?encryption=none&security=tls&sni=jp1.example.com&type=grpc&serviceName=grpc#jp-vless-grpc"
	node, err := Parse(link)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if node.Type != model.ProtocolVLESS {
		t.Fatalf("unexpected type: %s", node.Type)
	}
	if node.ServiceName != "grpc" {
		t.Fatalf("unexpected service name: %s", node.ServiceName)
	}
	if !node.TLS {
		t.Fatalf("expected tls to be enabled")
	}
}

func TestEncodeShadowsocksLink(t *testing.T) {
	t.Parallel()

	node := model.Node{
		Name:     "us-ss",
		Type:     model.ProtocolSS,
		Server:   "us1.example.com",
		Port:     8388,
		Cipher:   "aes-256-gcm",
		Password: "ss-password",
	}

	link, err := Encode(node)
	if err != nil {
		t.Fatalf("Encode returned error: %v", err)
	}
	if !strings.HasPrefix(link, "ss://") {
		t.Fatalf("unexpected link: %s", link)
	}
}
