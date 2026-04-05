package parser

import (
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"nodeforge/internal/model"
)

type v2rayServiceConfig struct {
	Inbounds []v2rayInbound `json:"inbounds" yaml:"inbounds"`
}

type v2rayInbound struct {
	Port           int                  `json:"port" yaml:"port"`
	Listen         string               `json:"listen" yaml:"listen"`
	Protocol       string               `json:"protocol" yaml:"protocol"`
	Tag            string               `json:"tag" yaml:"tag"`
	Settings       v2rayInboundSettings `json:"settings" yaml:"settings"`
	StreamSettings v2rayStreamSettings  `json:"streamSettings" yaml:"streamSettings"`
}

type v2rayInboundSettings struct {
	Clients  []v2rayClient `json:"clients" yaml:"clients"`
	Method   string        `json:"method" yaml:"method"`
	Password string        `json:"password" yaml:"password"`
}

type v2rayClient struct {
	ID       string `json:"id" yaml:"id"`
	Email    string `json:"email" yaml:"email"`
	AlterID  int    `json:"alterId" yaml:"alterId"`
	Flow     string `json:"flow" yaml:"flow"`
	Password string `json:"password" yaml:"password"`
}

type v2rayStreamSettings struct {
	Network         string                  `json:"network" yaml:"network"`
	Security        string                  `json:"security" yaml:"security"`
	TLSSettings     v2rayTLSSettings        `json:"tlsSettings" yaml:"tlsSettings"`
	RealitySettings v2rayRealitySettings    `json:"realitySettings" yaml:"realitySettings"`
	WSSettings      v2rayWSSettings         `json:"wsSettings" yaml:"wsSettings"`
	GRPCSettings    v2rayGRPCSettings       `json:"grpcSettings" yaml:"grpcSettings"`
	HTTPSettings    v2rayHTTPRequestSetting `json:"httpSettings" yaml:"httpSettings"`
}

type v2rayTLSSettings struct {
	ServerName string   `json:"serverName" yaml:"serverName"`
	ALPN       []string `json:"alpn" yaml:"alpn"`
}

type v2rayRealitySettings struct {
	ServerName string `json:"serverName" yaml:"serverName"`
}

type v2rayWSSettings struct {
	Path    string            `json:"path" yaml:"path"`
	Headers map[string]string `json:"headers" yaml:"headers"`
}

type v2rayGRPCSettings struct {
	ServiceName string `json:"serviceName" yaml:"serviceName"`
}

type v2rayHTTPRequestSetting struct {
	Host []string `json:"host" yaml:"host"`
	Path string   `json:"path" yaml:"path"`
}

func parseV2RayServiceConfig(content []byte, useYAML bool, sourcePath string, opts Options) (Result, error) {
	var cfg v2rayServiceConfig
	var err error
	if useYAML {
		err = yaml.Unmarshal(content, &cfg)
	} else {
		err = json.Unmarshal(content, &cfg)
	}
	if err != nil {
		return Result{}, fmt.Errorf("parse v2ray service config: %w", err)
	}
	if len(cfg.Inbounds) == 0 {
		return Result{}, fmt.Errorf("not a node document or supported v2ray service config")
	}

	baseName := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
	defaultServer := firstNonEmpty(opts.DefaultServer, "127.0.0.1")
	nodes := make([]model.Node, 0)
	var parseErrors []error

	for inboundIndex, inbound := range cfg.Inbounds {
		inboundNodes, skipped, err := nodesFromInbound(inbound, inboundIndex, baseName, defaultServer)
		if err != nil {
			return Result{}, err
		}
		nodes = append(nodes, inboundNodes...)
		parseErrors = append(parseErrors, skipped...)
	}
	if len(nodes) == 0 {
		return Result{}, fmt.Errorf("no supported inbound clients found in v2ray service config")
	}
	return Result{Nodes: nodes, Errors: parseErrors}, nil
}

func nodesFromInbound(inbound v2rayInbound, inboundIndex int, baseName, defaultServer string) ([]model.Node, []error, error) {
	protocol := model.NormalizeProtocol(inbound.Protocol)
	switch protocol {
	case model.ProtocolVMess, model.ProtocolVLESS:
		return vmessLikeNodesFromInbound(inbound, inboundIndex, baseName, defaultServer, protocol)
	case model.ProtocolTrojan:
		return trojanNodesFromInbound(inbound, inboundIndex, baseName, defaultServer)
	case model.ProtocolSS:
		return shadowsocksNodesFromInbound(inbound, inboundIndex, baseName, defaultServer)
	default:
		return nil, []error{fmt.Errorf("skip unsupported inbound protocol %q", inbound.Protocol)}, nil
	}
}

func vmessLikeNodesFromInbound(inbound v2rayInbound, inboundIndex int, baseName, defaultServer string, protocol model.Protocol) ([]model.Node, []error, error) {
	if len(inbound.Settings.Clients) == 0 {
		return nil, nil, fmt.Errorf("inbound %d has no clients", inboundIndex+1)
	}

	nodes := make([]model.Node, 0, len(inbound.Settings.Clients))
	for clientIndex, client := range inbound.Settings.Clients {
		node := buildBaseInboundNode(inbound, inboundIndex, baseName, defaultServer)
		node.Type = protocol
		node.UUID = client.ID
		node.Flow = client.Flow
		node.Name = buildClientName(baseName, inbound, protocol, clientIndex, client.Email)
		if protocol == model.ProtocolVMess {
			node.Cipher = "auto"
			if client.AlterID > 0 {
				node.Extra["alter_id"] = strconv.Itoa(client.AlterID)
			}
		}
		nodes = append(nodes, node)
	}
	return nodes, nil, nil
}

func trojanNodesFromInbound(inbound v2rayInbound, inboundIndex int, baseName, defaultServer string) ([]model.Node, []error, error) {
	if len(inbound.Settings.Clients) == 0 {
		return nil, nil, fmt.Errorf("inbound %d has no clients", inboundIndex+1)
	}

	nodes := make([]model.Node, 0, len(inbound.Settings.Clients))
	for clientIndex, client := range inbound.Settings.Clients {
		node := buildBaseInboundNode(inbound, inboundIndex, baseName, defaultServer)
		node.Type = model.ProtocolTrojan
		node.Password = firstNonEmpty(client.Password, client.ID)
		node.Name = buildClientName(baseName, inbound, model.ProtocolTrojan, clientIndex, client.Email)
		nodes = append(nodes, node)
	}
	return nodes, nil, nil
}

func shadowsocksNodesFromInbound(inbound v2rayInbound, inboundIndex int, baseName, defaultServer string) ([]model.Node, []error, error) {
	node := buildBaseInboundNode(inbound, inboundIndex, baseName, defaultServer)
	node.Type = model.ProtocolSS
	node.Name = buildClientName(baseName, inbound, model.ProtocolSS, 0, "")
	node.Cipher = inbound.Settings.Method
	node.Password = inbound.Settings.Password
	return []model.Node{node}, nil, nil
}

func buildBaseInboundNode(inbound v2rayInbound, inboundIndex int, baseName, defaultServer string) model.Node {
	stream := inbound.StreamSettings
	node := model.Node{
		Server:      resolveInboundServer(inbound.Listen, defaultServer),
		Port:        inbound.Port,
		TLS:         stream.Security == "tls" || stream.Security == "reality",
		SNI:         firstNonEmpty(stream.TLSSettings.ServerName, stream.RealitySettings.ServerName),
		ALPN:        stream.TLSSettings.ALPN,
		Network:     firstNonEmpty(stream.Network, "tcp"),
		Path:        firstNonEmpty(stream.WSSettings.Path, stream.HTTPSettings.Path),
		ServiceName: stream.GRPCSettings.ServiceName,
		UDP:         true,
		Group:       "Proxy",
		Headers:     map[string]string{},
		Extra:       map[string]string{},
		Tag:         inbound.Tag,
	}
	if host := firstNonEmpty(stream.WSSettings.Headers["Host"], firstHTTPHost(stream.HTTPSettings.Host)); host != "" {
		node.Host = host
		node.Headers["Host"] = host
	}
	if stream.Security == "reality" {
		node.Extra["security"] = "reality"
	}
	if node.Name == "" {
		node.Name = fmt.Sprintf("%s-%d", baseName, inboundIndex+1)
	}
	return node
}

func resolveInboundServer(listen, fallback string) string {
	listen = strings.TrimSpace(listen)
	if listen == "" {
		return fallback
	}
	if host, _, err := net.SplitHostPort(listen); err == nil {
		listen = host
	}
	switch listen {
	case "", "0.0.0.0", "::", "::0", "127.0.0.1", "::1":
		return fallback
	default:
		return listen
	}
}

func buildClientName(baseName string, inbound v2rayInbound, protocol model.Protocol, clientIndex int, email string) string {
	if strings.TrimSpace(email) != "" {
		return email
	}
	if strings.TrimSpace(inbound.Tag) != "" {
		if len(inbound.Settings.Clients) == 1 {
			return inbound.Tag
		}
		return fmt.Sprintf("%s-%d", inbound.Tag, clientIndex+1)
	}
	if len(inbound.Settings.Clients) == 1 {
		return fmt.Sprintf("%s-%s", baseName, protocol)
	}
	return fmt.Sprintf("%s-%s-%d", baseName, protocol, clientIndex+1)
}

func firstHTTPHost(hosts []string) string {
	for _, host := range hosts {
		host = strings.TrimSpace(host)
		if host != "" {
			return host
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
