package sqltext

import (
	"fmt"
	"io"
)

// splitChunk 流式分句每次至少多读这么多字节。
const splitChunk = 1 << 20

// SplitReader 从 r 里流式切出语句，逐条交给 fn；fn 返回错误时立即停下并原样返回。
// 切分规则与 SplitWith 完全一致，但只把"还没切完的那一条"留在内存里，
// 所以几百 MB 的转储文件也不需要整个读进来。
//
// 参数：maxStmt 是单条语句允许的最大字节数（0 不限），超过就报错而不是无限攒内存；
// 语句里的 Start / End 是语句在整个输入流里的字节偏移。
//
// 原理：词法是从左向右确定的——缓冲区里最后一个 ';' 之前的切分，不会因为后面还有数据而改变，
// 所以每轮只需要把"最后一条"留到下一轮，其余的都已经是完整语句。
func SplitReader(r io.Reader, o LexOptions, maxStmt int, fn func(Statement) error) error {
	var carry []byte // 上一轮留下的、还没切完的那一段
	base := 0        // carry[0] 在整个输入流里的偏移
	tmp := make([]byte, splitChunk)
	eof := false
	for !eof {
		// 一条语句特别长时缓冲区会越来越大：每轮读入量跟着缓冲区大小走，保证总耗时是线性的
		want := max(splitChunk, len(carry))
		for got := 0; got < want && !eof; {
			n, err := r.Read(tmp)
			carry = append(carry, tmp[:n]...)
			got += n
			if err == io.EOF {
				eof = true
			} else if err != nil {
				return err
			}
		}
		text := string(carry)
		stmts := SplitWith(text, o)
		keepFrom := len(text) // 读到头：全部切出来，什么都不留
		if !eof {
			keepFrom = 0
			if n := len(stmts); n > 0 {
				keepFrom = stmts[n-1].Start
				stmts = stmts[:n-1]
			}
		}
		for _, s := range stmts {
			s.Start, s.End = s.Start+base, s.End+base
			if err := fn(s); err != nil {
				return err
			}
		}
		carry = append(carry[:0], carry[keepFrom:]...)
		base += keepFrom
		if maxStmt > 0 && len(carry) > maxStmt {
			return fmt.Errorf("单条语句超过 %d MB，请用命令行工具导入", maxStmt>>20)
		}
	}
	return nil
}
