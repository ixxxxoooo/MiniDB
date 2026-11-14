package ui

import (
	"context"
	"time"

	"minidb/internal/app"
	"minidb/internal/model"
	"minidb/locales"
)

// 执行计划：在查询标签里对选中的（或光标所在的）语句生成计划，显示在结果区的「执行计划」页。

// planState 是一次解释的状态与结果。
type planState struct {
	stmt    string
	res     *model.ExplainResult
	hot     *model.PlanNode // 最费时的节点，树里高亮
	err     string
	loading bool
	tab     int // 0 计划树，1 原文
	dur     time.Duration
}

// explain 解释光标所在（或选中）的那一条语句；plain 解释不会执行语句。
func (q *queryTab) explain() {
	if q.plan != nil && q.plan.loading {
		return
	}
	stmts := q.statements(false)
	switch {
	case len(stmts) == 0:
		q.w.sh.toastWarn(locales.QueryEmpty())
	case len(stmts) > 1:
		q.w.sh.toastWarn(locales.ExplainOneStmt())
	default:
		q.explainStmt(stmts[0], q.planAnalyze)
	}
}

// explainStmt 解释 stmt。ANALYZE 会真实执行语句，所以先让用户确认。
func (q *queryTab) explainStmt(stmt string, analyze bool) {
	sh, w := q.w.sh, q.w
	if !app.CanExplain(w.cfg.Type) {
		sh.toastWarn(locales.ExplainUnsupported(app.DialectLabel(w.cfg.Type)))
		return
	}
	analyze = analyze && app.CanExplainAnalyze(w.cfg.Type)
	if analyze {
		sh.askWarn(locales.ExplainAnalyzeTitle(), locales.ExplainAnalyzeMsg(), locales.ExplainAnalyzeRun(), func() { q.runExplain(stmt, true) })
		return
	}
	q.runExplain(stmt, false)
}

func (q *queryTab) runExplain(stmt string, analyze bool) {
	w := q.w
	ps := &planState{stmt: stmt, loading: true}
	q.plan, q.page, q.hideResults = ps, pagePlan, false
	started, db := time.Now(), w.db
	var res *model.ExplainResult
	var err error
	w.sh.up.bg(func() {
		res, err = w.sh.app.Explain.Explain(context.Background(), w.cfg.ID, db, stmt, analyze)
	}, func() {
		if q.plan != ps { // 期间又发起了新的解释，或计划页已被关闭
			return
		}
		ps.loading, ps.dur = false, time.Since(started)
		if err != nil {
			ps.err = errMessage(err)
			return
		}
		ps.res, ps.hot = res, res.Hottest()
		if len(res.Roots) == 0 { // 没解析成树：直接看原文
			ps.tab = 1
		}
	})
}

// closePlan 关掉执行计划页，回到数据页。
func (q *queryTab) closePlan() {
	q.plan = nil
	if q.page == pagePlan {
		q.page = pageData
	}
}

// cmdExplain 是「执行计划」命令（菜单、命令面板、快捷键共用）。
func (s *Shell) cmdExplain() {
	if q := s.activeQuery(); q != nil {
		q.explain()
		s.redraw()
	}
}
