package main

import "fmt"

// Edge case: 二进制数据伪装成 Go 文件
// 这些数据看起来像二进制乱码，但实际是合法的 Go 代码

func BinaryLookAlike() []byte {
	// 返回包含空字节的数据
	data := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0x7F}
	return data
}

/* 二进制数据块（看起来像乱码，但在注释中）
\x00\x01\x02\x03\xFF\xFE\xFD\xFC
\xAB\xCD\xEF\x12\x34\x56\x78\x90
*/

func RealFuncHere() string {
	return "I'm real"
}
