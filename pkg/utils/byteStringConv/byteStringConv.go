package byteStringConv

import (
	"reflect"
	"unsafe"
)

// BytesToString 注意内存安全，该方法 string 与 byte 指向同个内存地址
func BytesToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// StringToBytes 注意内存安全，该方法 string 与 byte 指向同个内存地址
func StringToBytes(s string) (b []byte) {
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh.Data = sh.Data
	bh.Cap = sh.Len
	bh.Len = sh.Len
	return b
}
