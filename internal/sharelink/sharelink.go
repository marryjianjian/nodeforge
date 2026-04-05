package sharelink

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"nodeforge/internal/model"
)

func Parse(raw string) (model.Node, error) {
	raw = strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(raw, "vmess://"):
		return parseVMess(raw)
	case strings.HasPrefix(raw, "vless://"):
		return parseURLStyle(raw, model.ProtocolVLESS)
	case strings.HasPrefix(raw, "trojan://"):
		return parseURLStyle(raw, model.ProtocolTrojan)
	case strings.HasPrefix(raw, "ss://"):
		return parseSS(raw)
	default:
		return model.Node{}, fmt.Errorf("unsupported share link scheme")
	}
}

func Encode(node model.Node) (string, error) {
	switch node.Type {
	case model.ProtocolVMess:
		return encodeVMess(node)
	case model.ProtocolVLESS:
		return encodeVLESS(node)
	case model.ProtocolTrojan:
		return encodeTrojan(node)
	case model.ProtocolSS:
		return encodeSS(node), nil
	default:
		return "", fmt.Errorf("unsupported node type %q", node.Type)
	}
}

func parseVMess(raw string) (model.Node, error) {
	payload := strings.TrimPrefix(raw, "vmess://")
	data, err := decodeBase64Loose(payload)
	if err != nil {
		return model.Node{}, fmt.Errorf("decode vmess payload: %w", err)
	}

	var wire struct {
		Name string `json:"ps"`
		Add  string `json:"add"`
		Port string `json:"port"`
		ID   string `json:"id"`
		Scy  string `json:"scy"`
		Net  string `json:"net"`
		Host string `json:"host"`
		Path string `json:"path"`
		TLS  string `json:"tls"`
		SNI  string `json:"sni"`
		ALPN string `json:"alpn"`
		Aid  string `json:"aid"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return model.Node{}, fmt.Errorf("decode vmess json: %w", err)
	}

	port, err := strconv.Atoi(strings.TrimSpace(wire.Port))
	if err != nil {
		return model.Node{}, fmt.Errorf("invalid vmess port %q", wire.Port)
	}

	node := model.Node{
		Name:    wire.Name,
		Type:    model.ProtocolVMess,
		Server:  wire.Add,
		Port:    port,
		UUID:    wire.ID,
		Cipher:  wire.Scy,
		TLS:     isTLSEnabled(wire.TLS),
		SNI:     wire.SNI,
		ALPN:    splitCSV(wire.ALPN),
		Network: wire.Net,
		Host:    wire.Host,
		Path:    wire.Path,
	}
	if strings.TrimSpace(wire.Aid) != "" && wire.Aid != "0" {
		node.Extra = map[string]string{"alter_id": wire.Aid}
	}
	return node, nil
}

func parseURLStyle(raw string, protocol model.Protocol) (model.Node, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return model.Node{}, fmt.Errorf("parse %s url: %w", protocol, err)
	}

	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		return model.Node{}, fmt.Errorf("invalid %s port %q", protocol, parsed.Port())
	}

	query := parsed.Query()
	node := model.Node{
		Name:        decodeFragment(parsed.Fragment),
		Type:        protocol,
		Server:      parsed.Hostname(),
		Port:        port,
		TLS:         isTLSEnabled(query.Get("security")),
		SNI:         query.Get("sni"),
		ALPN:        splitCSV(query.Get("alpn")),
		Network:     firstNonEmpty(query.Get("type"), "tcp"),
		Host:        query.Get("host"),
		Path:        query.Get("path"),
		Flow:        query.Get("flow"),
		ServiceName: firstNonEmpty(query.Get("serviceName"), query.Get("service_name")),
	}

	if protocol == model.ProtocolTrojan {
		node.Password = parsed.User.Username()
	} else {
		node.UUID = parsed.User.Username()
	}

	extra := map[string]string{}
	for key, value := range map[string]string{
		"security":    query.Get("security"),
		"fingerprint": firstNonEmpty(query.Get("fp"), query.Get("fingerprint")),
		"public_key":  firstNonEmpty(query.Get("pbk"), query.Get("public-key")),
		"short_id":    firstNonEmpty(query.Get("sid"), query.Get("short-id")),
	} {
		if strings.TrimSpace(value) != "" {
			extra[key] = value
		}
	}
	if len(extra) > 0 {
		node.Extra = extra
	}

	return node, nil
}

func parseSS(raw string) (model.Node, error) {
	rest := strings.TrimPrefix(raw, "ss://")
	mainPart, fragment := splitOnce(rest, "#")
	name := decodeFragment(fragment)

	mainPart, queryPart := splitOnce(mainPart, "?")
	extra := map[string]string{}
	if queryPart != "" {
		values, err := url.ParseQuery(queryPart)
		if err == nil {
			if plugin := values.Get("plugin"); strings.TrimSpace(plugin) != "" {
				extra["plugin"] = plugin
			}
		}
	}

	var (
		credentialPart string
		hostPart       string
	)

	if strings.Contains(mainPart, "@") {
		credentialPart, hostPart = splitAtLast(mainPart, "@")
		if !strings.Contains(credentialPart, ":") {
			decoded, err := decodeBase64Loose(credentialPart)
			if err != nil {
				return model.Node{}, fmt.Errorf("decode ss credential: %w", err)
			}
			credentialPart = string(decoded)
		}
	} else {
		decoded, err := decodeBase64Loose(mainPart)
		if err != nil {
			return model.Node{}, fmt.Errorf("decode ss payload: %w", err)
		}
		decodedText := string(decoded)
		credentialPart, hostPart = splitAtLast(decodedText, "@")
	}

	if credentialPart == "" || hostPart == "" {
		return model.Node{}, fmt.Errorf("invalid ss link payload")
	}

	method, password, ok := strings.Cut(credentialPart, ":")
	if !ok {
		return model.Node{}, fmt.Errorf("invalid ss method/password segment")
	}

	host, portText, err := net.SplitHostPort(hostPart)
	if err != nil {
		return model.Node{}, fmt.Errorf("invalid ss host/port: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return model.Node{}, fmt.Errorf("invalid ss port %q", portText)
	}

	node := model.Node{
		Name:     name,
		Type:     model.ProtocolSS,
		Server:   host,
		Port:     port,
		Cipher:   method,
		Password: password,
		Extra:    extra,
	}
	return node, nil
}

func encodeVMess(node model.Node) (string, error) {
	wire := map[string]string{
		"v":    "2",
		"ps":   node.Name,
		"add":  node.Server,
		"port": strconv.Itoa(node.Port),
		"id":   node.UUID,
		"aid":  firstNonEmpty(node.Extra["alter_id"], "0"),
		"scy":  firstNonEmpty(node.Cipher, "auto"),
		"net":  firstNonEmpty(node.Network, "tcp"),
		"type": "none",
		"host": node.Host,
		"path": node.Path,
		"sni":  node.SNI,
	}
	if node.TLS {
		wire["tls"] = "tls"
	}
	if len(node.ALPN) > 0 {
		wire["alpn"] = strings.Join(node.ALPN, ",")
	}

	data, err := json.Marshal(wire)
	if err != nil {
		return "", fmt.Errorf("encode vmess payload: %w", err)
	}
	return "vmess://" + base64.StdEncoding.EncodeToString(data), nil
}

func encodeVLESS(node model.Node) (string, error) {
	values := url.Values{}
	values.Set("type", firstNonEmpty(node.Network, "tcp"))
	values.Set("encryption", "none")
	if node.TLS {
		values.Set("security", "tls")
	} else {
		values.Set("security", "none")
	}
	if node.SNI != "" {
		values.Set("sni", node.SNI)
	}
	if len(node.ALPN) > 0 {
		values.Set("alpn", strings.Join(node.ALPN, ","))
	}
	if node.Host != "" {
		values.Set("host", node.Host)
	}
	if node.Path != "" {
		values.Set("path", node.Path)
	}
	if node.Flow != "" {
		values.Set("flow", node.Flow)
	}
	if node.ServiceName != "" {
		values.Set("serviceName", node.ServiceName)
	}

	link := url.URL{
		Scheme:   "vless",
		User:     url.User(node.UUID),
		Host:     net.JoinHostPort(node.Server, strconv.Itoa(node.Port)),
		RawQuery: values.Encode(),
		Fragment: node.Name,
	}
	return link.String(), nil
}

func encodeTrojan(node model.Node) (string, error) {
	values := url.Values{}
	values.Set("type", firstNonEmpty(node.Network, "tcp"))
	if node.TLS {
		values.Set("security", "tls")
	} else {
		values.Set("security", "none")
	}
	if node.SNI != "" {
		values.Set("sni", node.SNI)
	}
	if len(node.ALPN) > 0 {
		values.Set("alpn", strings.Join(node.ALPN, ","))
	}
	if node.Host != "" {
		values.Set("host", node.Host)
	}
	if node.Path != "" {
		values.Set("path", node.Path)
	}
	if node.ServiceName != "" {
		values.Set("serviceName", node.ServiceName)
	}

	link := url.URL{
		Scheme:   "trojan",
		User:     url.User(node.Password),
		Host:     net.JoinHostPort(node.Server, strconv.Itoa(node.Port)),
		RawQuery: values.Encode(),
		Fragment: node.Name,
	}
	return link.String(), nil
}

func encodeSS(node model.Node) string {
	credential := fmt.Sprintf("%s:%s", node.Cipher, node.Password)
	encoded := base64.RawURLEncoding.EncodeToString([]byte(credential))
	link := fmt.Sprintf("ss://%s@%s", encoded, net.JoinHostPort(node.Server, strconv.Itoa(node.Port)))
	values := url.Values{}
	if plugin := node.Extra["plugin"]; strings.TrimSpace(plugin) != "" {
		values.Set("plugin", plugin)
	}
	if query := values.Encode(); query != "" {
		link += "?" + query
	}
	if node.Name != "" {
		link += "#" + url.QueryEscape(node.Name)
	}
	return link
}

func decodeBase64Loose(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, " ", "")
	paddings := []string{value, padBase64(value)}
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	var lastErr error
	for _, candidate := range paddings {
		for _, encoding := range encodings {
			data, err := encoding.DecodeString(candidate)
			if err == nil {
				return data, nil
			}
			lastErr = err
		}
	}
	return nil, lastErr
}

func padBase64(value string) string {
	switch len(value) % 4 {
	case 2:
		return value + "=="
	case 3:
		return value + "="
	default:
		return value
	}
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func isTLSEnabled(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "tls" || value == "true" || value == "1" || value == "reality"
}

func decodeFragment(fragment string) string {
	if fragment == "" {
		return ""
	}
	decoded, err := url.QueryUnescape(fragment)
	if err != nil {
		return fragment
	}
	return decoded
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func splitOnce(value, sep string) (string, string) {
	left, right, ok := strings.Cut(value, sep)
	if !ok {
		return value, ""
	}
	return left, right
}

func splitAtLast(value, sep string) (string, string) {
	index := strings.LastIndex(value, sep)
	if index < 0 {
		return "", ""
	}
	return value[:index], value[index+len(sep):]
}
