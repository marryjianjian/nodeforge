# nodeforge

`nodeforge` 是一个本地 Go CLI 工具，用来把少量自建节点的原始配置或分享链接，转换成统一内部结构后，再导出为多种常见客户端可直接使用的配置文件。

当前版本聚焦于“小而稳、方便扩展”的本地转换流程，不提供 Web 面板、远程订阅托管、测速、流量统计或后台服务。

主 README 英文版见 [README.md](/Users/plack/code/nodeforge/README.md)。

## 功能范围

- 输入支持：
  - YAML 节点文件
  - JSON 节点文件
  - 目录输入，自动聚合目录中的多个 `.yaml` / `.yml` / `.json` / `.txt` 文件
  - `links.txt` 分享链接列表
  - V2Ray 服务端配置文件
- 当前已支持协议：
  - `vmess`
  - `vless`
  - `trojan`
  - `ss` / `shadowsocks`
- 输出支持：
  - Clash / Mihomo 风格 YAML
  - sing-box 风格 JSON
  - 标准化分享链接列表 `links.txt`
  - `v2rayN` 可用的 Base64 订阅内容 `subscription.txt`
- 行为特性：
  - 统一中间模型
  - 基本字段校验
  - 非法节点跳过
  - 输出成功/失败统计

## 目录结构

```text
cmd/convert/            CLI 入口
internal/model/         统一中间模型
internal/parser/        输入解析器
internal/renderer/      输出渲染器
internal/sharelink/     分享链接编解码
internal/validate/      字段校验
examples/               示例输入
test/                   基础 CLI 集成测试
```

## 输入格式

YAML 与 JSON 推荐使用如下结构：

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

支持的统一字段包括：

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

说明：

- `group` 是节点分组，CLI 的 `--group` 只作为缺省分组名。
- `headers`、`extra` 是后续扩展 Reality、插件、UTLS、更多传输层参数的预留位。
- `links.txt` 输入时，每行一个分享链接，支持 `vmess://`、`vless://`、`trojan://`、`ss://`。
- 对于 V2Ray 服务端配置，当前会从 `inbounds` 中提取常见协议入站并生成节点。
- 如果配置文件本身没有外部可访问地址，可以：
  - 使用 `--server`，为所有匹配文件提供同一个兜底主机名
  - 当 `-i` 指向目录时使用 `--server-from-filename`，让每个文件名成为该文件的兜底主机名
- `--server` 与 `--server-from-filename` 互斥，不能同时使用。
- 如果源配置文件的文件名本身看起来就是域名，在没有显式 `server` 覆盖时，`nodeforge` 会自动把这个域名作为兜底 `server` 值。
- 对于 `vmess + ws` 的 V2Ray 服务端配置，如果 TLS 和 WS `Host` 没有显式给出，`nodeforge` 会默认按 `vmess + ws + tls` 处理。
- 当配置文件名本身看起来就是域名时，在这种隐式 `vmess + ws + tls` 场景下，`nodeforge` 会默认把该域名回填到 `server` 和 WS `Host`。
- 如果最终推导出的节点启用了 TLS，且原始入站 `listen` 是 `127.0.0.1`，`nodeforge` 会默认把导出端口处理为 `443`。

示例文件见：

- [examples/nodes.yaml](/Users/plack/code/nodeforge/examples/nodes.yaml)
- [examples/nodes.json](/Users/plack/code/nodeforge/examples/nodes.json)
- [examples/links.txt](/Users/plack/code/nodeforge/examples/links.txt)

## 输出设计

### Clash / Mihomo

- 生成 `proxies`
- 生成至少一个手动 `select` 组
- 生成最小 `rules`
- 默认包含 `mixed-port: 7890`

### sing-box

- 生成 `outbounds`
- 生成 `selector`
- 生成本地 `mixed` inbound，默认监听 `127.0.0.1:2080`
- 生成最小 `route.final`

### links.txt

- 输出标准化后的分享链接列表

### v2rayN 订阅

- 先生成多行标准化分享链接
- 再整体做 Base64 编码
- 适合后续自己托管为订阅文件

## 使用方式

构建：

```bash
go mod tidy
go build -o ./bin/convert ./cmd/convert
```

直接运行：

```bash
go run ./cmd/convert -i ./examples/nodes.yaml -f clash -o ./out/clash.yaml
go run ./cmd/convert -i ./examples/nodes.yaml -f singbox -o ./out/singbox.json --pretty
go run ./cmd/convert -i ./examples/links.txt -f links -o ./out/links.txt
go run ./cmd/convert -i ./examples/nodes.yaml -f v2rayn -o ./out/subscription.txt
go run ./cmd/convert -i ./examples/nodes.yaml -f all -o ./out --pretty
go run ./cmd/convert -i ./test/data -f v2rayn -o ./out/test-subscription.txt --server demo.example.com
go run ./cmd/convert -i ./configs -f v2rayn -o ./out/subscription.txt --server-from-filename
```

如果已经构建二进制：

```bash
./bin/convert -i ./examples/nodes.yaml -f clash -o ./out/clash.yaml
./bin/convert -i ./examples/nodes.yaml -f singbox -o ./out/singbox.json --pretty
./bin/convert -i ./examples/links.txt -f links -o ./out/links.txt
./bin/convert -i ./examples/nodes.yaml -f v2rayn -o ./out/subscription.txt
./bin/convert -i ./examples/nodes.yaml -f all -o ./out --pretty
./bin/convert -i ./test/data -f v2rayn -o ./out/test-subscription.txt --server demo.example.com
./bin/convert -i ./configs -f v2rayn -o ./out/subscription.txt --server-from-filename
```

## 参数说明

- `-i`, `--input`：输入文件路径或目录路径
- `-f`, `--format`：输出格式，支持 `clash`、`singbox`、`links`、`v2rayn`、`all`
- `-o`, `--output`：输出文件或输出目录
- `--pretty`：格式化 JSON 输出
- `--group`：节点缺省分组名
- `--server`：当输入是服务端配置且没有外部地址时，补齐默认服务器地址
- `--server-from-filename`：当 `--input` 是目录时，从每个配置文件的文件名推断兜底服务器主机名

`server` 兜底选项的规则：

- `--server-from-filename` 只能和目录输入一起使用
- `--server-from-filename` 和 `--server` 不能同时使用
- 文件名映射规则只会去掉扩展名
  - `edge.example.com.json` 会变成 `edge.example.com`
  - `hk-gateway.internal.yaml` 会变成 `hk-gateway.internal`

## 校验规则

- `server` 不能为空
- `port` 必须在 `1-65535`
- `vmess` / `vless` 需要 `uuid`
- `trojan` 需要 `password`
- `ss` 需要 `cipher` 与 `password`
- `network=ws` 时需要 `path`
- `network=grpc` 时需要 `service_name`

遇到非法节点时：

- 输出 warning
- 跳过非法节点
- 最终输出成功/失败统计

## 扩展建议

- 在 [internal/model/node.go](/Users/plack/code/nodeforge/internal/model/node.go) 中为新协议补充显式字段，而不是把所有差异都塞进 `extra`
- 在 [internal/sharelink/sharelink.go](/Users/plack/code/nodeforge/internal/sharelink/sharelink.go) 中补充新的分享链接编解码逻辑
- 在 [internal/renderer/renderer.go](/Users/plack/code/nodeforge/internal/renderer/renderer.go) 中新增更多输出格式，如 `surge`、`loon`、`xray`
- 在 [internal/validate/node.go](/Users/plack/code/nodeforge/internal/validate/node.go) 中增加协议级校验规则
- 在 [internal/parser/v2ray.go](/Users/plack/code/nodeforge/internal/parser/v2ray.go) 中继续扩展更多服务端配置字段映射

## 当前已知边界

- 目前只覆盖常见基础协议和基础字段映射
- `Reality`、高级 `UTLS`、`multiplex`、`plugin`、`transport` 细节仍以预留扩展位为主
- 还没有支持 `hysteria2`、`tuic`、`wireguard`、`ssh` 等更多协议
- `sing-box` 与 `Clash` 的高级特性暂未完全对齐，只实现第一版最小可用配置
- 从 V2Ray 服务端配置反推订阅时，如果原配置没有公网域名或 IP，需要通过 `--server` 或目录模式下的 `--server-from-filename` 明确指定
