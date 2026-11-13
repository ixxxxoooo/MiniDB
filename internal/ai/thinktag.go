package ai

import "strings"

// thinkSplitter 把流式文本里行内的 <think>…</think> 拆出来。
//
// 不少模型和网关（Ollama、本地部署的 DeepSeek-R1 / QwQ、部分中转）不走单独的推理字段，
// 而是把思考过程包在 <think> 标签里和正文一起流出来。标签可能被切在两个分片中间，
// 所以拿不准的结尾先留着，等下一个分片来了再判断。
//
// @author ixxxxoooo
type thinkSplitter struct {
	inThink bool
	hold    string // 可能是半个标签的结尾，还没有交出去
	started bool   // 已经见过正文里的非空白字符：<think> 只认开头的那一个
}

const (
	thinkOpen  = "<think>"
	thinkClose = "</think>"
)

// feed 喂进一个分片，返回其中属于思考的部分和属于正文的部分。
//
// 参数：delta 模型流出的一段文本。
// 返回值：thinking 思考文字；text 正文文字；两者都可能为空（内容被暂存，等下一个分片）。
func (s *thinkSplitter) feed(delta string) (thinking, text string) {
	buf := s.hold + delta
	s.hold = ""
	var th, tx strings.Builder
	for buf != "" {
		tag := thinkOpen
		if s.inThink {
			tag = thinkClose
		}
		if !s.inThink && s.started {
			tx.WriteString(buf) // 正文已经开始：之后出现的 <think> 字样只是普通文字
			break
		}
		if i := strings.Index(buf, tag); i >= 0 {
			if !s.inThink && strings.TrimSpace(buf[:i]) != "" {
				s.emit(&th, &tx, buf) // 标签前面已经有正文了：它只是普通文字
				break
			}
			s.emit(&th, &tx, buf[:i])
			buf = buf[i+len(tag):]
			s.inThink = !s.inThink
			if !s.inThink {
				buf = strings.TrimLeft(buf, "\n") // </think> 后面的空行不算正文
			}
			continue
		}
		// 没有完整的标签：结尾若是标签的前缀就先留着
		keep := partialSuffix(buf, tag)
		s.emit(&th, &tx, buf[:len(buf)-keep])
		s.hold = buf[len(buf)-keep:]
		break
	}
	return th.String(), tx.String()
}

// flush 在流结束时把暂存的结尾交出来。
func (s *thinkSplitter) flush() (thinking, text string) {
	rest := s.hold
	s.hold = ""
	if s.inThink {
		return rest, ""
	}
	return "", rest
}

func (s *thinkSplitter) emit(th, tx *strings.Builder, part string) {
	if part == "" {
		return
	}
	if s.inThink {
		th.WriteString(part)
		return
	}
	if strings.TrimSpace(part) != "" {
		s.started = true
	}
	tx.WriteString(part)
}

// partialSuffix 返回 buf 结尾有多少个字节是 tag 的前缀（不含完整的 tag）。
func partialSuffix(buf, tag string) int {
	for n := min(len(tag)-1, len(buf)); n > 0; n-- {
		if strings.HasPrefix(tag, buf[len(buf)-n:]) {
			return n
		}
	}
	return 0
}
