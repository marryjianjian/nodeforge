package parser

type Options struct {
	// DefaultServer 用于从服务端配置反推分享链接时补齐外部地址。
	// 当服务端配置没有可用的外部地址时，会使用这个值。
	DefaultServer string

	// ServerFromFilename 用于在目录输入场景下，从文件名推断服务端域名。
	// 例如 example.com.json 会推断为 example.com。
	ServerFromFilename bool
}
