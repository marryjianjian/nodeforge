package renderer

import (
	"testing"

	"gopkg.in/yaml.v3"

	"nodeforge/internal/model"
)

func TestRenderClashIncludesBypassRules(t *testing.T) {
	t.Parallel()

	nodes := []model.Node{
		{
			Name:   "demo-vmess",
			Type:   model.ProtocolVMess,
			Server: "demo.example.com",
			Port:   443,
			UUID:   "11111111-1111-1111-1111-111111111111",
			Cipher: "auto",
			TLS:    true,
		},
	}

	data, err := RenderClash(nodes, "Proxy")
	if err != nil {
		t.Fatalf("RenderClash returned error: %v", err)
	}

	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal clash yaml: %v", err)
	}

	rawRules, ok := parsed["rules"].([]any)
	if !ok {
		t.Fatalf("rules missing or invalid type: %#v", parsed["rules"])
	}

	rules := make([]string, 0, len(rawRules))
	for _, item := range rawRules {
		rule, ok := item.(string)
		if !ok {
			t.Fatalf("unexpected rule type: %#v", item)
		}
		rules = append(rules, rule)
	}

	expected := []string{
		"IP-CIDR,127.0.0.0/8,DIRECT,no-resolve",
		"IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
		"IP-CIDR,172.16.0.0/12,DIRECT,no-resolve",
		"IP-CIDR,192.168.0.0/16,DIRECT,no-resolve",
		"IP-CIDR6,fc00::/7,DIRECT,no-resolve",
		"GEOIP,CN,DIRECT,no-resolve",
		"MATCH,Proxy",
	}
	for _, want := range expected {
		if !containsString(rules, want) {
			t.Fatalf("expected rule %q in %#v", want, rules)
		}
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
