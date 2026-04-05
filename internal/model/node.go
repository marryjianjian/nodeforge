package model

import (
	"fmt"
	"strings"
)

type Protocol string

const (
	ProtocolVMess  Protocol = "vmess"
	ProtocolVLESS  Protocol = "vless"
	ProtocolTrojan Protocol = "trojan"
	ProtocolSS     Protocol = "ss"
)

type Node struct {
	Name        string            `json:"name" yaml:"name"`
	Type        Protocol          `json:"type" yaml:"type"`
	Server      string            `json:"server" yaml:"server"`
	Port        int               `json:"port" yaml:"port"`
	UUID        string            `json:"uuid,omitempty" yaml:"uuid,omitempty"`
	Password    string            `json:"password,omitempty" yaml:"password,omitempty"`
	Cipher      string            `json:"cipher,omitempty" yaml:"cipher,omitempty"`
	TLS         bool              `json:"tls,omitempty" yaml:"tls,omitempty"`
	SNI         string            `json:"sni,omitempty" yaml:"sni,omitempty"`
	ALPN        []string          `json:"alpn,omitempty" yaml:"alpn,omitempty"`
	Network     string            `json:"network,omitempty" yaml:"network,omitempty"`
	Host        string            `json:"host,omitempty" yaml:"host,omitempty"`
	Path        string            `json:"path,omitempty" yaml:"path,omitempty"`
	UDP         bool              `json:"udp,omitempty" yaml:"udp,omitempty"`
	Group       string            `json:"group,omitempty" yaml:"group,omitempty"`
	Tag         string            `json:"tag,omitempty" yaml:"tag,omitempty"`
	Flow        string            `json:"flow,omitempty" yaml:"flow,omitempty"`
	ServiceName string            `json:"service_name,omitempty" yaml:"service_name,omitempty"`
	Headers     map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Extra       map[string]string `json:"extra,omitempty" yaml:"extra,omitempty"`
}

type Document struct {
	Group string `json:"group,omitempty" yaml:"group,omitempty"`
	Nodes []Node `json:"nodes" yaml:"nodes"`
}

func NormalizeProtocol(value string) Protocol {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "vmess":
		return ProtocolVMess
	case "vless":
		return ProtocolVLESS
	case "trojan":
		return ProtocolTrojan
	case "ss", "shadowsocks":
		return ProtocolSS
	default:
		return Protocol(strings.ToLower(strings.TrimSpace(value)))
	}
}

func (n *Node) Normalize(defaultGroup string) {
	n.Name = strings.TrimSpace(n.Name)
	n.Type = NormalizeProtocol(string(n.Type))
	n.Server = strings.TrimSpace(n.Server)
	n.UUID = strings.TrimSpace(n.UUID)
	n.Password = strings.TrimSpace(n.Password)
	n.Cipher = strings.TrimSpace(n.Cipher)
	n.SNI = strings.TrimSpace(n.SNI)
	n.Network = strings.ToLower(strings.TrimSpace(n.Network))
	n.Host = strings.TrimSpace(n.Host)
	n.Path = strings.TrimSpace(n.Path)
	n.Group = strings.TrimSpace(n.Group)
	n.Tag = strings.TrimSpace(n.Tag)
	n.Flow = strings.TrimSpace(n.Flow)
	n.ServiceName = strings.TrimSpace(n.ServiceName)

	if n.Group == "" {
		n.Group = strings.TrimSpace(defaultGroup)
	}
	if n.Group == "" {
		n.Group = "Proxy"
	}
	if n.Network == "" {
		n.Network = "tcp"
	}
	if n.Headers == nil {
		n.Headers = map[string]string{}
	}
	if n.Extra == nil {
		n.Extra = map[string]string{}
	}
	if n.Host == "" {
		if host, ok := n.Headers["Host"]; ok {
			n.Host = strings.TrimSpace(host)
		}
	}
	if n.Type == ProtocolVMess && n.Cipher == "" {
		n.Cipher = "auto"
	}
	if n.Name == "" {
		n.Name = defaultNodeName(*n)
	}
	n.ALPN = cleanStrings(n.ALPN)
}

func defaultNodeName(n Node) string {
	protocol := string(n.Type)
	if protocol == "" {
		protocol = "node"
	}
	host := n.Server
	if host == "" {
		host = "unknown"
	}
	if n.Port > 0 {
		return fmt.Sprintf("%s-%s-%d", protocol, host, n.Port)
	}
	return fmt.Sprintf("%s-%s", protocol, host)
}

func cleanStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cleaned := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		cleaned = append(cleaned, value)
	}
	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
}
