package idmap

import (
	"bytes"
	"strconv"
)

// violateIDFragments 是需要从虚拟ID中排除的违规数字片段。
var violateIDFragments = [...][]byte{
	[]byte("8964"),
	[]byte("89064"),
	[]byte("89604"),
	[]byte("890604"),
}

// IsViolateID 检测虚拟ID中是否包含违规数字。使用栈缓冲区避免热路径产生内存分配。
func IsViolateID(id int64) bool {
	if id <= 0 {
		return false
	}
	var buffer [20]byte
	digits := strconv.AppendInt(buffer[:0], id, 10)
	for _, fragment := range violateIDFragments {
		if bytes.Contains(digits, fragment) {
			return true
		}
	}
	return false
}
