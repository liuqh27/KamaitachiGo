package common

// ByteView 字节视图，只读的字节切片包装
type ByteView struct {
	b []byte
}

// NewByteView 创建字节视图
func NewByteView(b []byte) ByteView {
	return ByteView{b: cloneBytes(b)}
}

// Len 返回视图长度
func (v ByteView) Len() int {
	return len(v.b)
}

// ByteSlice 返回字节切片的拷贝
func (v ByteView) ByteSlice() []byte {
	return cloneBytes(v.b)
}

// String 以字符串形式返回数据
func (v ByteView) String() string {
	return string(v.b)
}

// cloneBytes 克隆字节切片
func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}

