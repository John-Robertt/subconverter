// Package ssparse 实现 SIP002 Shadowsocks URI 主体部分的解析。
//
// 它从 ss:// URI 去掉协议前缀后的主体部分中，提取服务器、端口、加密方式、
// 密码、节点名，以及可选的插件声明。userinfo 支持 base64 和明文两种形式。
//
// 本包是叶子依赖，不引用任何内部包。
package ssparse
