package sqltext

import (
	"strings"
	"testing"
)

func TestLexCoversWholeInput(t *testing.T) {
	sql := "SELECT 'a;b', `c` /* x */ FROM t -- tail\n; $$ body; $$ # h\n"
	pos := 0
	for _, tk := range Lex(sql, EditorLexOptions) {
		if tk.Start != pos {
			t.Fatalf("token 之间有空洞: 期望从 %d 开始, 得到 %d", pos, tk.Start)
		}
		pos = tk.End
	}
	if pos != len(sql) {
		t.Fatalf("token 没有覆盖到结尾: %d != %d", pos, len(sql))
	}
}

func TestSkeletonDialectChoices(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		opt  LexOptions
		want string // 骨架里必须包含 / 不包含
		has  bool
	}{
		{"# 作为注释", "SELECT 1 # DELETE", LexOptions{HashComments: true}, "DELETE", false},
		{"# 按代码保守扫描", "SELECT 1 # DELETE", LexOptions{}, "DELETE", true},
		{"$$ 作为字符串", "SELECT $$ DELETE $$", LexOptions{DollarQuotes: true}, "DELETE", false},
		{"$$ 按代码保守扫描", "SELECT $$ DELETE $$", LexOptions{}, "DELETE", true},
		{"嵌套注释吞掉内层", "SELECT 1 /* a /* b */ DELETE */", LexOptions{NestedComments: true}, "DELETE", false},
		{"非嵌套时内层结束注释", "SELECT 1 /* a /* b */ DELETE */", LexOptions{}, "DELETE", true},
		{"严格 -- 需空白", "SELECT 1 --DELETE", LexOptions{StrictDash: true}, "DELETE", true},
		{"宽松 -- 是注释", "SELECT 1 --DELETE", LexOptions{}, "DELETE", false},
		{"未闭合注释按代码", "SELECT 1 /* DELETE", LexOptions{UnclosedCommentIsCode: true}, "DELETE", true},
		{"未闭合注释被吞掉", "SELECT 1 /* DELETE", LexOptions{}, "DELETE", false},
	}
	for _, c := range cases {
		got, _ := Skeleton(c.sql, c.opt)
		if strings.Contains(got, c.want) != c.has {
			t.Errorf("%s: 骨架 %q, 期望包含 %q = %v", c.name, got, c.want, c.has)
		}
	}
}

func TestSkeletonFlagsExecutableComment(t *testing.T) {
	_, exec := Skeleton("SELECT /*!50000 1 */", LexOptions{})
	if !exec {
		t.Fatal("应识别 MySQL 可执行注释")
	}
	if _, exec := Skeleton("SELECT /* 1 */", LexOptions{}); exec {
		t.Fatal("普通注释不是可执行注释")
	}
}

func TestSplitWithNestedComments(t *testing.T) {
	sql := "SELECT 1 /* a /* x */ ; b */ ; SELECT 2"
	if got := Texts(sql); len(got) != 3 {
		t.Fatalf("默认不嵌套时内层 */ 提前结束注释: %q", got)
	}
	got := SplitWith(sql, LexOptions{NestedComments: true, HashComments: true, DollarQuotes: true, BackslashEscapes: true})
	if len(got) != 2 || got[1].Text != "SELECT 2" {
		t.Fatalf("嵌套注释应整体吞掉: %+v", got)
	}
}
