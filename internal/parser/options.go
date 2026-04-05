package parser

type Options struct {
	// DefaultServer 用于从服务端配置反推分享链接时补齐外部地址。
	// 当服务端配置没有可用的外部地址时，会使用这个值。
	DefaultServer string
}
