// Package ssparse implements SIP002 Shadowsocks URI body parsing.
//
// It extracts server, port, cipher, password, node name, and optional
// plugin specifications from the body portion of an ss:// URI (after
// stripping the scheme prefix). Both base64 and plaintext userinfo
// forms are supported.
//
// This package is a leaf dependency with no internal imports.
package ssparse
