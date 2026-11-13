package schemaindex

import (
	"math"
	"sort"
	"strings"

	"minidb/internal/ai"
)

// BM25 参数与各字段权重。权重表达"名字比注释更可信，注释比类型更可信"。
const (
	bm25K1 = 1.2
	bm25B  = 0.75

	weightName    = 3.0
	weightComment = 2.0
	weightContext = 0.5 // 表的列名、列所在的表名：只是线索
	weightType    = 0.3
	prefixFactor  = 0.5 // 查询词只是索引词前缀时的折扣
)

// doc 一个可检索的对象：一张表或一列。
type doc struct {
	table  int // 在 schema.Tables 里的下标
	column int // 列下标；-1 表示这是表本身
	tf     map[string]float64
	length float64
}

// Searcher 在一个库的表和列上做关键词检索（BM25，支持中英文、驼峰、单复数）。
// 它只读，创建后可被多个 goroutine 同时使用。
//
// @author ixxxxoooo
type Searcher struct {
	schema *ai.SchemaContext
	docs   []doc
	df     map[string]int // 词 → 含有它的文档数
	avgLen float64
	vocab  []string // 排序后的词表，用于前缀匹配
	byName map[string]int
}

// TableHit 一张匹配的表。
type TableHit struct {
	Name    string
	Comment string
	Columns int
	Score   float64
}

// ColumnHit 一个匹配的列。
type ColumnHit struct {
	Table   string
	Column  string
	Type    string
	Comment string
	Score   float64
}

// NewSearcher 为 schema 建立检索结构。
func NewSearcher(schema *ai.SchemaContext) *Searcher {
	s := &Searcher{schema: schema, df: map[string]int{}, byName: map[string]int{}}
	for ti, t := range schema.Tables {
		s.byName[strings.ToLower(t.Name)] = ti
		s.add(ti, -1, tableTerms(t))
		for ci, c := range t.Columns {
			s.add(ti, ci, columnTerms(t, c))
		}
	}
	total := 0.0
	for _, d := range s.docs {
		total += d.length
	}
	if len(s.docs) > 0 {
		s.avgLen = total / float64(len(s.docs))
	}
	for term := range s.df {
		s.vocab = append(s.vocab, term)
	}
	sort.Strings(s.vocab)
	return s
}

// tableTerms 表文档的词：表名、表注释，外加各列名与列注释作为线索。
func tableTerms(t ai.TableSchema) map[string]float64 {
	terms := map[string]float64{}
	addTerms(terms, t.Name, weightName)
	addTerms(terms, t.Comment, weightComment)
	for _, c := range t.Columns {
		addTerms(terms, c.Name, weightContext)
		addTerms(terms, c.Comment, weightContext)
	}
	return terms
}

// columnTerms 列文档的词：列名、列注释、类型，外加所在表的名字作为线索。
func columnTerms(t ai.TableSchema, c ai.ColumnSchema) map[string]float64 {
	terms := map[string]float64{}
	addTerms(terms, c.Name, weightName)
	addTerms(terms, c.Comment, weightComment)
	addTerms(terms, c.Type, weightType)
	addTerms(terms, t.Name, weightContext)
	addTerms(terms, t.Comment, weightContext)
	return terms
}

func addTerms(terms map[string]float64, text string, weight float64) {
	for _, tok := range tokenize(text) {
		terms[tok] += weight
	}
}

func (s *Searcher) add(table, column int, terms map[string]float64) {
	length := 0.0
	for term, w := range terms {
		length += w
		s.df[term]++
	}
	s.docs = append(s.docs, doc{table: table, column: column, tf: terms, length: length})
}

// score 计算文档对查询词的 BM25 得分；查询词是索引词前缀时按折扣计入。
func (s *Searcher) score(d doc, query []string) float64 {
	n := float64(len(s.docs))
	total := 0.0
	for _, q := range query {
		for term, factor := range s.matches(q) {
			tf := d.tf[term]
			if tf == 0 {
				continue
			}
			idf := math.Log(1 + (n-float64(s.df[term])+0.5)/(float64(s.df[term])+0.5))
			norm := tf * (bm25K1 + 1) / (tf + bm25K1*(1-bm25B+bm25B*d.length/s.avgLen))
			total += factor * idf * norm
		}
	}
	return total
}

// matches 返回查询词命中的索引词及折扣：完全相同为 1，长度≥3 的前缀命中打折。
func (s *Searcher) matches(q string) map[string]float64 {
	out := map[string]float64{}
	if s.df[q] > 0 {
		out[q] = 1
	}
	if len([]rune(q)) < 3 {
		return out
	}
	i := sort.SearchStrings(s.vocab, q)
	for ; i < len(s.vocab) && strings.HasPrefix(s.vocab[i], q); i++ {
		if _, exact := out[s.vocab[i]]; !exact {
			out[s.vocab[i]] = prefixFactor
		}
	}
	return out
}

// Tables 按相关度返回最多 limit 张表。keywords 为空时返回 nil；
// 表名与某个关键词完全相同的表额外加分，排在同分的前面。
func (s *Searcher) Tables(keywords []string, limit int) []TableHit {
	query := tokenize(strings.Join(keywords, " "))
	if len(query) == 0 {
		return nil
	}
	var hits []TableHit
	for _, d := range s.docs {
		if d.column != -1 {
			continue
		}
		sc := s.score(d, query)
		t := s.schema.Tables[d.table]
		if s.exactName(t.Name, keywords) {
			sc += 100
		}
		if sc > 0 {
			hits = append(hits, TableHit{Name: t.Name, Comment: t.Comment, Columns: len(t.Columns), Score: sc})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Name < hits[j].Name
	})
	return capSlice(hits, limit)
}

// Columns 按相关度返回最多 limit 个列；tables 非空时只在这些表里找。
func (s *Searcher) Columns(keywords []string, tables []string, limit int) []ColumnHit {
	query := tokenize(strings.Join(keywords, " "))
	if len(query) == 0 {
		return nil
	}
	only := map[string]bool{}
	for _, t := range tables {
		only[strings.ToLower(t)] = true
	}
	var hits []ColumnHit
	for _, d := range s.docs {
		if d.column == -1 {
			continue
		}
		t := s.schema.Tables[d.table]
		if len(only) > 0 && !only[strings.ToLower(t.Name)] {
			continue
		}
		c := t.Columns[d.column]
		sc := s.score(d, query)
		if s.exactName(c.Name, keywords) {
			sc += 50
		}
		if sc > 0 {
			hits = append(hits, ColumnHit{Table: t.Name, Column: c.Name, Type: c.Type, Comment: c.Comment, Score: sc})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		if hits[i].Table != hits[j].Table {
			return hits[i].Table < hits[j].Table
		}
		return hits[i].Column < hits[j].Column
	})
	return capSlice(hits, limit)
}

// Table 按名字（不区分大小写）取一张表。
func (s *Searcher) Table(name string) (ai.TableSchema, bool) {
	i, ok := s.byName[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return ai.TableSchema{}, false
	}
	return s.schema.Tables[i], true
}

// exactName 报告 name 是否与某个关键词完全相同（不区分大小写）。
func (s *Searcher) exactName(name string, keywords []string) bool {
	for _, k := range keywords {
		if strings.EqualFold(strings.TrimSpace(k), name) {
			return true
		}
	}
	return false
}

func capSlice[T any](items []T, limit int) []T {
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}
