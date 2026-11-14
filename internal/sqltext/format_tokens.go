package sqltext

import (
	"strings"
	"unicode"
)

type tokKind int

const (
	tkSpace tokKind = iota
	tkWord
	tkString // '…' "…" `…`
	tkComment
	tkSymbol
	tkNumber
	tkParam // :name {name} {{name}} ${name} 这类参数占位符，整体不可拆开
)

type token struct {
	kind tokKind
	text string
	// ownLine 仅对注释有意义：注释前面（同一行内）没有任何代码，即源码里它独占一行
	ownLine bool
}

func isWordStart(c rune) bool { return unicode.IsLetter(c) || c == '_' || c == '@' || c == '$' }

func isWordPart(c rune) bool {
	return unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_' || c == '$' || c == '@'
}

// tokenize 把 SQL 切成词法单元；空白也保留为单元，供格式化器判断注释是否独占一行。
func tokenize(sql string) []token {
	var out []token
	rs := []rune(sql)
	atLineStart := true // 当前行到目前为止只有空白
	for i := 0; i < len(rs); {
		c := rs[i]
		start := i
		switch {
		case unicode.IsSpace(c):
			for i < len(rs) && unicode.IsSpace(rs[i]) {
				if rs[i] == '\n' {
					atLineStart = true
				}
				i++
			}
			out = append(out, token{kind: tkSpace, text: string(rs[start:i])})
			continue
		case c == '-' && i+1 < len(rs) && rs[i+1] == '-', c == '#':
			for i < len(rs) && rs[i] != '\n' {
				i++
			}
			out = append(out, token{kind: tkComment, text: string(rs[start:i]), ownLine: atLineStart})
		case c == '/' && i+1 < len(rs) && rs[i+1] == '*':
			i += 2
			for i+1 < len(rs) && !(rs[i] == '*' && rs[i+1] == '/') {
				i++
			}
			i = min(i+2, len(rs))
			out = append(out, token{kind: tkComment, text: string(rs[start:i]), ownLine: atLineStart})
		case c == '\'' || c == '"' || c == '`':
			i++
			for i < len(rs) {
				if c != '`' && rs[i] == '\\' && i+1 < len(rs) {
					i += 2
					continue
				}
				if rs[i] == c {
					if i+1 < len(rs) && rs[i+1] == c {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			out = append(out, token{kind: tkString, text: string(rs[start:min(i, len(rs))])})
		case unicode.IsDigit(c) || (c == '.' && i+1 < len(rs) && unicode.IsDigit(rs[i+1]) && !prevIsWordLike(out)):
			// 数字：整数、小数、科学计数法、0x1F、1_000，整体算一个词
			for i < len(rs) && (isWordPart(rs[i]) || rs[i] == '.') {
				if (rs[i] == 'e' || rs[i] == 'E') && i+1 < len(rs) && (rs[i+1] == '+' || rs[i+1] == '-') && i > start {
					i++ // 1e+5 里的符号属于数字
				}
				i++
			}
			out = append(out, token{kind: tkNumber, text: string(rs[start:i])})
		case c == '{' || (c == '$' && i+1 < len(rs) && rs[i+1] == '{'):
			// {name} {{name}} ${name}：到配对的右花括号为止，不跨行
			j := i
			if rs[j] == '$' {
				j++
			}
			depth := 0
			end := -1
			for k := j; k < len(rs) && rs[k] != '\n'; k++ {
				if rs[k] == '{' {
					depth++
				} else if rs[k] == '}' {
					depth--
					if depth == 0 {
						end = k + 1
						break
					}
				}
			}
			if end < 0 {
				i++
				out = append(out, token{kind: tkSymbol, text: string(rs[start:i])})
			} else {
				i = end
				out = append(out, token{kind: tkParam, text: string(rs[start:i])})
			}
		case c == ':' && i+1 < len(rs) && isWordStart(rs[i+1]) && rs[i+1] != '@' && rs[i+1] != '$' && !(i > 0 && rs[i-1] == ':'):
			// :name 参数（不是 :: 类型转换）
			i++
			for i < len(rs) && isWordPart(rs[i]) {
				i++
			}
			out = append(out, token{kind: tkParam, text: string(rs[start:i])})
		case isWordStart(c):
			for i < len(rs) && isWordPart(rs[i]) {
				i++
			}
			out = append(out, token{kind: tkWord, text: string(rs[start:i])})
		default:
			i++
			if i+1 < len(rs) {
				switch three := string(rs[start : i+2]); three {
				case "->>", "<=>", "#>>", "!~*":
					i += 2
					out = append(out, token{kind: tkSymbol, text: three})
					atLineStart = false
					continue
				}
			}
			if i < len(rs) {
				switch two := string(rs[start : i+1]); two {
				case "<=", ">=", "<>", "!=", "||", "::", "->", ":=", "<<", ">>", "&&", "@>", "<@", "!~", "~*", "#>":
					i++
				}
			}
			out = append(out, token{kind: tkSymbol, text: string(rs[start:i])})
		}
		atLineStart = false
	}
	return out
}

// prevIsWordLike 判断前一个有效单元是不是词/右括号：此时 ".5" 里的点是成员访问而不是小数点。
func prevIsWordLike(toks []token) bool {
	if len(toks) == 0 || toks[len(toks)-1].kind == tkSpace { // 点前有空白：当作小数
		return false
	}
	last := toks[len(toks)-1]
	return last.kind == tkWord || last.kind == tkString || last.text == ")"
}

func words(s string) map[string]bool {
	m := map[string]bool{}
	for _, k := range strings.Fields(s) {
		m[k] = true
	}
	return m
}

// keywords 在任何位置都会被转成大写。
var keywords = words(`SELECT FROM WHERE GROUP BY ORDER HAVING LIMIT OFFSET JOIN INNER LEFT RIGHT FULL OUTER CROSS NATURAL ON USING
	AS AND OR NOT IN IS NULL LIKE BETWEEN EXISTS CASE WHEN THEN ELSE END DISTINCT UNION ALL INSERT INTO VALUES UPDATE SET DELETE CREATE TABLE
	ALTER DROP INDEX PRIMARY KEY FOREIGN REFERENCES DEFAULT ASC DESC WITH RECURSIVE OVER PARTITION IF TRUE FALSE EXPLAIN ANALYZE SHOW DESCRIBE
	USE BEGIN COMMIT ROLLBACK TRUNCATE VIEW UNIQUE CONSTRAINT CHECK COLLATE INTERVAL CAST ILIKE RETURNING EXCEPT INTERSECT
	ADD COLUMN MODIFY CHANGE RENAME TO TEMPORARY TEMP REPLACE CURRENT_TIMESTAMP CURRENT_DATE CURRENT_TIME CASCADE RESTRICT
	DUPLICATE CONFLICT DO NOTHING WINDOW FETCH NEXT ROWS ONLY FIRST LAST NULLS ANY SOME ESCAPE SIMILAR`)

// typeWords 是列定义里"列名之后"的类型词，只在 CREATE/ALTER 的列定义里转成大写，
// 以免把名叫 date、text 的普通列改了大小写。
var typeWords = words(`INT INTEGER TINYINT SMALLINT MEDIUMINT BIGINT DECIMAL NUMERIC FLOAT DOUBLE REAL BIT BOOL BOOLEAN CHAR VARCHAR NCHAR
	NVARCHAR TEXT TINYTEXT MEDIUMTEXT LONGTEXT BLOB TINYBLOB MEDIUMBLOB LONGBLOB BINARY VARBINARY DATE TIME DATETIME TIMESTAMP YEAR JSON
	JSONB UUID SERIAL BIGSERIAL SMALLSERIAL ENUM BYTEA MONEY CITEXT GEOMETRY POINT CHARACTER VARYING PRECISION INT2 INT4 INT8 FLOAT4 FLOAT8
	TIMESTAMPTZ TIMETZ`)

// ddlWords 是列定义与表选项里的修饰词（列名与类型之后），同样只在 DDL 里转大写。
var ddlWords = words(`UNSIGNED ZEROFILL AUTO_INCREMENT AUTOINCREMENT COMMENT CHARSET CHARACTER COLLATE GENERATED ALWAYS STORED VIRTUAL IDENTITY
	WITHOUT TIME ZONE ACTION NO ENGINE ROW_FORMAT TABLESPACE INHERITS USING BTREE HASH VISIBLE INVISIBLE FIRST AFTER`)

// constraintStarts 在列定义列表里出现在第一个位置时，说明这一项是约束而不是列。
var constraintStarts = words(`PRIMARY KEY UNIQUE INDEX CONSTRAINT FOREIGN CHECK FULLTEXT SPATIAL EXCLUDE LIKE PERIOD`)

// alterActions 是 ALTER TABLE 表名之后的动作。
var alterActions = words(`ADD DROP MODIFY CHANGE RENAME ALTER ENABLE DISABLE OWNER CONVERT ENGINE AUTO_INCREMENT COMMENT`)

// joinWords 开启一个 JOIN 子句。
var joinWords = words(`JOIN INNER LEFT RIGHT FULL CROSS NATURAL`)

// clauseStarts 在语句层独占一行。
var clauseStarts = words(`FROM WHERE HAVING LIMIT OFFSET UNION EXCEPT INTERSECT RETURNING WINDOW QUALIFY`)

// spaceBeforeParen 后面跟 "(" 时要留空格的关键字；其他词（函数名、类型名）则紧贴括号。
var spaceBeforeParen = words(`IN NOT AND OR ON USING VALUES WHERE AS EXISTS SELECT FROM JOIN UNION ALL ANY SOME OVER WHEN THEN ELSE CASE HAVING BY SET
	DEFAULT KEY INDEX UNIQUE PRIMARY FOREIGN CHECK REFERENCES CONSTRAINT INTO TABLE LIKE ILIKE BETWEEN IS DO CONFLICT PARTITION EXCEPT INTERSECT RETURNING
	WITH RECURSIVE INSERT UPDATE DELETE REPLACE LATERAL ADD`)
