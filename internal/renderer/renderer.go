package renderer

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"nodeforge/internal/model"
	"nodeforge/internal/sharelink"
)

type Format string

const (
	FormatClash   Format = "clash"
	FormatSingBox Format = "singbox"
	FormatLinks   Format = "links"
	FormatV2RayN  Format = "v2rayn"
	FormatAll     Format = "all"
)

func ParseFormat(value string) (Format, error) {
	switch Format(strings.ToLower(strings.TrimSpace(value))) {
	case FormatClash:
		return FormatClash, nil
	case FormatSingBox:
		return FormatSingBox, nil
	case FormatLinks:
		return FormatLinks, nil
	case FormatV2RayN:
		return FormatV2RayN, nil
	case FormatAll:
		return FormatAll, nil
	default:
		return "", fmt.Errorf("unsupported output format %q", value)
	}
}

func DefaultFilename(format Format) string {
	switch format {
	case FormatClash:
		return "clash.yaml"
	case FormatSingBox:
		return "singbox.json"
	case FormatLinks:
		return "links.txt"
	case FormatV2RayN:
		return "subscription.txt"
	default:
		return "output.txt"
	}
}

func RenderClash(nodes []model.Node, groupName string) ([]byte, error) {
	config := map[string]any{
		"mixed-port":   7890,
		"allow-lan":    false,
		"mode":         "rule",
		"log-level":    "info",
		"proxies":      buildClashProxies(nodes),
		"proxy-groups": buildClashGroups(nodes, groupName),
		"rules":        buildClashRules(groupName),
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("render clash yaml: %w", err)
	}
	return data, nil
}

func RenderSingBox(nodes []model.Node, groupName string, pretty bool) ([]byte, error) {
	selectors, selectorTags := buildSingBoxSelectors(nodes, groupName)
	outbounds := make([]map[string]any, 0, len(nodes)+len(selectors)+1)
	outbounds = append(outbounds, selectors...)
	for _, node := range nodes {
		outbounds = append(outbounds, buildSingBoxOutbound(node))
	}
	outbounds = append(outbounds, map[string]any{
		"type": "direct",
		"tag":  "direct",
	})

	config := map[string]any{
		"log": map[string]any{
			"level": "info",
		},
		"inbounds": []map[string]any{
			{
				"type":        "mixed",
				"tag":         "mixed-in",
				"listen":      "127.0.0.1",
				"listen_port": 2080,
			},
		},
		"outbounds": outbounds,
		"route": map[string]any{
			"auto_detect_interface": true,
			"final":                 groupName,
		},
	}

	if len(selectorTags) > 0 {
		config["experimental"] = map[string]any{
			"cache_file": map[string]any{
				"enabled": true,
				"path":    "cache.db",
			},
		}
	}

	if pretty {
		return json.MarshalIndent(config, "", "  ")
	}
	return json.Marshal(config)
}

func RenderLinks(nodes []model.Node) ([]byte, error) {
	links := make([]string, 0, len(nodes))
	for _, node := range nodes {
		link, err := sharelink.Encode(node)
		if err != nil {
			return nil, fmt.Errorf("encode %s: %w", node.Name, err)
		}
		links = append(links, link)
	}
	return []byte(strings.Join(links, "\n") + "\n"), nil
}

func RenderV2RayNSubscription(nodes []model.Node) ([]byte, error) {
	linkBytes, err := RenderLinks(nodes)
	if err != nil {
		return nil, err
	}
	encoded := base64.StdEncoding.EncodeToString(linkBytes)
	return []byte(encoded + "\n"), nil
}

func buildClashRules(groupName string) []string {
	return []string{
		"IP-CIDR,127.0.0.0/8,DIRECT,no-resolve",
		"IP-CIDR,10.0.0.0/8,DIRECT,no-resolve",
		"IP-CIDR,172.16.0.0/12,DIRECT,no-resolve",
		"IP-CIDR,192.168.0.0/16,DIRECT,no-resolve",
		"IP-CIDR,169.254.0.0/16,DIRECT,no-resolve",
		"IP-CIDR,100.64.0.0/10,DIRECT,no-resolve",
		"IP-CIDR6,::1/128,DIRECT,no-resolve",
		"IP-CIDR6,fc00::/7,DIRECT,no-resolve",
		"IP-CIDR6,fe80::/10,DIRECT,no-resolve",
		"GEOIP,CN,DIRECT,no-resolve",
		fmt.Sprintf("MATCH,%s", groupName),
	}
}

func buildClashProxies(nodes []model.Node) []map[string]any {
	proxies := make([]map[string]any, 0, len(nodes))
	for _, node := range nodes {
		proxies = append(proxies, buildClashProxy(node))
	}
	return proxies
}

func buildClashProxy(node model.Node) map[string]any {
	proxy := map[string]any{
		"name":   node.Name,
		"type":   clashType(node.Type),
		"server": node.Server,
		"port":   node.Port,
	}

	switch node.Type {
	case model.ProtocolVMess, model.ProtocolVLESS:
		proxy["uuid"] = node.UUID
	case model.ProtocolTrojan:
		proxy["password"] = node.Password
	case model.ProtocolSS:
		proxy["cipher"] = node.Cipher
		proxy["password"] = node.Password
	}

	if node.Type == model.ProtocolVMess {
		proxy["cipher"] = firstNonEmpty(node.Cipher, "auto")
		proxy["alterId"] = 0
		if alterID := node.Extra["alter_id"]; alterID != "" {
			if parsed, err := strconv.Atoi(alterID); err == nil {
				proxy["alterId"] = parsed
			}
		}
	}
	if node.UDP {
		proxy["udp"] = true
	}
	if node.TLS {
		proxy["tls"] = true
	}
	if node.SNI != "" {
		proxy["servername"] = node.SNI
	}
	if len(node.ALPN) > 0 {
		proxy["alpn"] = node.ALPN
	}
	if node.Network != "" && node.Network != "tcp" {
		proxy["network"] = node.Network
	}
	if node.Flow != "" {
		proxy["flow"] = node.Flow
	}
	if node.Network == "ws" {
		wsHeaders := map[string]string{}
		if node.Host != "" {
			wsHeaders["Host"] = node.Host
		}
		wsOpts := map[string]any{
			"path": node.Path,
		}
		if len(wsHeaders) > 0 {
			wsOpts["headers"] = wsHeaders
		}
		proxy["ws-opts"] = wsOpts
	}
	if node.Network == "grpc" && node.ServiceName != "" {
		proxy["grpc-opts"] = map[string]any{
			"grpc-service-name": node.ServiceName,
		}
	}

	// Extra 保留给后续 Reality、UTLS、插件等字段扩展。
	if security := node.Extra["security"]; security == "reality" {
		proxy["client-fingerprint"] = firstNonEmpty(node.Extra["fingerprint"], "chrome")
	}

	return proxy
}

func buildClashGroups(nodes []model.Node, groupName string) []map[string]any {
	groups := []map[string]any{
		{
			"name":    groupName,
			"type":    "select",
			"proxies": append(allNodeNames(nodes), "DIRECT"),
		},
	}

	grouped := groupNodes(nodes)
	groupNames := make([]string, 0, len(grouped))
	for name := range grouped {
		if name == "" || name == groupName {
			continue
		}
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)
	for _, name := range groupNames {
		groups = append(groups, map[string]any{
			"name":    name,
			"type":    "select",
			"proxies": append(allNodeNames(grouped[name]), "DIRECT"),
		})
	}

	return groups
}

func buildSingBoxSelectors(nodes []model.Node, groupName string) ([]map[string]any, []string) {
	grouped := groupNodes(nodes)
	selectorNames := make([]string, 0, len(grouped))
	for name := range grouped {
		if name == "" || name == groupName {
			continue
		}
		selectorNames = append(selectorNames, name)
	}
	sort.Strings(selectorNames)

	selectors := []map[string]any{
		{
			"type":      "selector",
			"tag":       groupName,
			"outbounds": append(allNodeNames(nodes), "direct"),
			"default":   firstNodeName(nodes),
		},
	}
	for _, name := range selectorNames {
		selectors = append(selectors, map[string]any{
			"type":      "selector",
			"tag":       name,
			"outbounds": append(allNodeNames(grouped[name]), "direct"),
			"default":   firstNodeName(grouped[name]),
		})
	}
	return selectors, selectorNames
}

func buildSingBoxOutbound(node model.Node) map[string]any {
	outbound := map[string]any{
		"type":        singBoxType(node.Type),
		"tag":         node.Name,
		"server":      node.Server,
		"server_port": node.Port,
	}

	switch node.Type {
	case model.ProtocolVMess:
		outbound["uuid"] = node.UUID
		outbound["security"] = firstNonEmpty(node.Cipher, "auto")
	case model.ProtocolVLESS:
		outbound["uuid"] = node.UUID
	case model.ProtocolTrojan:
		outbound["password"] = node.Password
	case model.ProtocolSS:
		outbound["method"] = node.Cipher
		outbound["password"] = node.Password
	}

	if node.TLS {
		tlsBlock := map[string]any{
			"enabled": true,
		}
		if node.SNI != "" {
			tlsBlock["server_name"] = node.SNI
		}
		if len(node.ALPN) > 0 {
			tlsBlock["alpn"] = node.ALPN
		}
		if fingerprint := node.Extra["fingerprint"]; fingerprint != "" {
			tlsBlock["utls"] = map[string]any{
				"enabled":     true,
				"fingerprint": fingerprint,
			}
		}
		outbound["tls"] = tlsBlock
	}
	if node.Flow != "" {
		outbound["flow"] = node.Flow
	}

	if transport := buildSingBoxTransport(node); len(transport) > 0 {
		outbound["transport"] = transport
	}

	// Extra 保留给后续 Reality / multiplex / plugin 等字段扩展。
	return outbound
}

func buildSingBoxTransport(node model.Node) map[string]any {
	switch node.Network {
	case "", "tcp":
		return nil
	case "ws":
		transport := map[string]any{
			"type": "ws",
			"path": node.Path,
		}
		if node.Host != "" {
			transport["headers"] = map[string]any{"Host": node.Host}
		}
		return transport
	case "grpc":
		transport := map[string]any{
			"type": "grpc",
		}
		if node.ServiceName != "" {
			transport["service_name"] = node.ServiceName
		}
		return transport
	default:
		return map[string]any{
			"type": node.Network,
		}
	}
}

func clashType(protocol model.Protocol) string {
	if protocol == model.ProtocolSS {
		return "ss"
	}
	return string(protocol)
}

func singBoxType(protocol model.Protocol) string {
	if protocol == model.ProtocolSS {
		return "shadowsocks"
	}
	return string(protocol)
}

func groupNodes(nodes []model.Node) map[string][]model.Node {
	grouped := make(map[string][]model.Node)
	for _, node := range nodes {
		grouped[node.Group] = append(grouped[node.Group], node)
	}
	return grouped
}

func allNodeNames(nodes []model.Node) []string {
	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		names = append(names, node.Name)
	}
	return names
}

func firstNodeName(nodes []model.Node) string {
	if len(nodes) == 0 {
		return "direct"
	}
	return nodes[0].Name
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
