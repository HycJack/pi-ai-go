package main

import (
	_ "embed"

	"golang.org/x/image/font/gofont/goregular"
)

// 内置字体资源。
//
//go:embed assets/LXGWWenKai-Regular.ttf
var lxgwWenKaiTTF []byte

// goregularTTF 是内置的 Go 字体（goregular），作为西文回退。
var goregularTTF = goregular.TTF

// defaultFontBytes 返回默认字体（霞鹜文楷 LXGW WenKai），支持中文且为手写风格。
func defaultFontBytes() []byte {
	return lxgwWenKaiTTF
}
