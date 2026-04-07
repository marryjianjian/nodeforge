# nodeforge

[简体中文](./README.zh-CN.md)

`nodeforge` is a local Go CLI that converts a small set of self-hosted node definitions or share links into a unified internal model, then renders them into client-ready configuration files or subscription content.

The current scope is intentionally narrow: local conversion and export only. It does not provide a web panel, hosted subscription service, speed tests, traffic accounting, or any background service.

## Features

- Input sources:
  - YAML node files
  - JSON node files
  - Directory input, aggregating multiple `.yaml`, `.yml`, `.json`, and `.txt` files
  - `links.txt` files with one share link per line
  - V2Ray server-side configuration files
- Supported protocols:
  - `vmess`
  - `vless`
  - `trojan`
  - `ss` / `shadowsocks`
- Output formats:
  - Clash / Mihomo YAML
  - sing-box JSON
  - normalized share-link list in `links.txt`
  - Base64 subscription output for `v2rayN` in `subscription.txt`
- Behavior:
  - unified intermediate model
  - field validation
  - invalid-node skipping with warnings
  - success/failure summary reporting

## Project Layout

```text
cmd/convert/            CLI entrypoint
internal/model/         Unified intermediate model
internal/parser/        Input parsers
internal/renderer/      Output renderers
internal/sharelink/     Share-link encode/decode helpers
internal/validate/      Field validation
examples/               Example inputs
test/                   Basic CLI integration tests
```

## Input Model

For YAML and JSON, the recommended format is:

```yaml
group: Demo
nodes:
  - name: hk-vmess-ws
    type: vmess
    server: hk1.example.com
    port: 443
    uuid: 11111111-1111-1111-1111-111111111111
    cipher: auto
    tls: true
    sni: hk1.example.com
    alpn: [h2, http/1.1]
    network: ws
    host: cdn.example.com
    path: /vmess
    udp: true
    group: HK
```

Supported normalized fields include:

- `name`
- `type`
- `server`
- `port`
- `uuid`
- `password`
- `cipher`
- `tls`
- `sni`
- `alpn`
- `network`
- `host`
- `path`
- `udp`
- `group`
- `tag`
- `flow`
- `service_name`
- `headers`
- `extra`

Notes:

- `group` is the node group. The CLI flag `--group` is only used as a fallback default group.
- `headers` and `extra` are reserved extension points for future Reality, plugin, UTLS, and transport-specific fields.
- `links.txt` accepts one share link per line and currently supports `vmess://`, `vless://`, `trojan://`, and `ss://`.
- For V2Ray server configs, `nodeforge` extracts supported inbounds and converts them into nodes.
- If a V2Ray server config does not contain an externally reachable host, you can:
  - use `--server` to provide one fixed fallback host for all matching files
  - use `--server-from-filename` when `-i` points to a directory, so each file name becomes that file's fallback host
- `--server` and `--server-from-filename` are mutually exclusive.

Examples:

- [examples/nodes.yaml](/Users/plack/code/nodeforge/examples/nodes.yaml)
- [examples/nodes.json](/Users/plack/code/nodeforge/examples/nodes.json)
- [examples/links.txt](/Users/plack/code/nodeforge/examples/links.txt)

## Output Design

### Clash / Mihomo

- Generates `proxies`
- Generates at least one manual `select` group
- Generates a minimal `rules` section
- Includes a minimal runnable config with `mixed-port: 7890`

### sing-box

- Generates `outbounds`
- Generates a `selector`
- Generates a local `mixed` inbound listening on `127.0.0.1:2080`
- Generates a minimal `route.final`

### links.txt

- Outputs normalized share links, one per line

### v2rayN Subscription

- Builds a normalized multi-line share-link list
- Base64-encodes the whole payload
- Produces subscription content suitable for self-hosted `v2rayN` subscriptions

## Build

```bash
go mod tidy
go build -o ./bin/convert ./cmd/convert
```

## Usage

Run from source:

```bash
go run ./cmd/convert -i ./examples/nodes.yaml -f clash -o ./out/clash.yaml
go run ./cmd/convert -i ./examples/nodes.yaml -f singbox -o ./out/singbox.json --pretty
go run ./cmd/convert -i ./examples/links.txt -f links -o ./out/links.txt
go run ./cmd/convert -i ./examples/nodes.yaml -f v2rayn -o ./out/subscription.txt
go run ./cmd/convert -i ./examples/nodes.yaml -f all -o ./out --pretty
go run ./cmd/convert -i ./test/data -f v2rayn -o ./out/test-subscription.txt --server demo.example.com
go run ./cmd/convert -i ./configs -f v2rayn -o ./out/subscription.txt --server-from-filename
```

Run the built binary:

```bash
./bin/convert -i ./examples/nodes.yaml -f clash -o ./out/clash.yaml
./bin/convert -i ./examples/nodes.yaml -f singbox -o ./out/singbox.json --pretty
./bin/convert -i ./examples/links.txt -f links -o ./out/links.txt
./bin/convert -i ./examples/nodes.yaml -f v2rayn -o ./out/subscription.txt
./bin/convert -i ./examples/nodes.yaml -f all -o ./out --pretty
./bin/convert -i ./test/data -f v2rayn -o ./out/test-subscription.txt --server demo.example.com
./bin/convert -i ./configs -f v2rayn -o ./out/subscription.txt --server-from-filename
```

## CLI Flags

- `-i`, `--input`: input file path or directory path
- `-f`, `--format`: output format, one of `clash`, `singbox`, `links`, `v2rayn`, `all`
- `-o`, `--output`: output file path or output directory
- `--pretty`: pretty-print JSON output
- `--group`: fallback default group name
- `--server`: fallback external server address when server-side configs do not contain one
- `--server-from-filename`: when `--input` is a directory, derive the fallback server host from each config file name

Rules for server fallback options:

- `--server-from-filename` only works when `-i/--input` points to a directory
- `--server-from-filename` and `--server` cannot be used together
- File names are mapped by stripping the extension only
  - `edge.example.com.json` becomes `edge.example.com`
  - `hk-gateway.internal.yaml` becomes `hk-gateway.internal`

## Validation Rules

- `server` must not be empty
- `port` must be within `1-65535`
- `vmess` / `vless` require `uuid`
- `trojan` requires `password`
- `ss` requires both `cipher` and `password`
- `network=ws` requires `path`
- `network=grpc` requires `service_name`

Invalid nodes do not crash the whole run:

- warnings are printed
- invalid nodes are skipped
- a final success/failure summary is shown

## Development Notes

The generated config outputs were validated with:

```bash
sing-box check -c ./out/singbox.json
mihomo -t -d ./out/mihomo-home -f ./out/clash.yaml
```

## Extension Points

- Add protocol-specific explicit fields in [internal/model/node.go](/Users/plack/code/nodeforge/internal/model/node.go) instead of overloading `extra`
- Add new share-link encoders/decoders in [internal/sharelink/sharelink.go](/Users/plack/code/nodeforge/internal/sharelink/sharelink.go)
- Add new output formats in [internal/renderer/renderer.go](/Users/plack/code/nodeforge/internal/renderer/renderer.go), such as `surge`, `loon`, or `xray`
- Add stronger protocol-level validation in [internal/validate/node.go](/Users/plack/code/nodeforge/internal/validate/node.go)
- Extend V2Ray server-side config extraction in [internal/parser/v2ray.go](/Users/plack/code/nodeforge/internal/parser/v2ray.go)

## Current Boundaries

- The current implementation focuses on common protocols and a minimum viable field mapping
- Advanced Reality, UTLS, multiplex, plugin, and transport details are still reserved as extension points
- Protocols such as `hysteria2`, `tuic`, `wireguard`, and `ssh` are not implemented yet
- sing-box and Clash advanced features are not fully aligned; only a minimal practical first version is generated
- When deriving subscriptions from V2Ray server configs, you must provide either `--server` or directory-based `--server-from-filename` if the original config does not expose a public host or IP
