package app

import (
	"context"
	"strings"

	"minidb/internal/database"
	"minidb/internal/logger"
	"minidb/internal/model"
)

// ExplainService 生成执行计划。界面的「解释」按钮和 AI 的 sql_explain_plan 工具都走它，行为一致。
type ExplainService struct {
	manager *database.Manager
}

func NewExplainService(manager *database.Manager) *ExplainService {
	return &ExplainService{manager: manager}
}

// explainVerbs 是可以被解释的语句类型。
var explainVerbs = map[string]bool{
	"SELECT": true, "WITH": true, "INSERT": true, "UPDATE": true, "DELETE": true, "REPLACE": true, "VALUES": true, "TABLE": true,
}

// checkExplainable 校验语句能不能解释：必须是单条语句，且是查询或增删改。
// 普通 EXPLAIN 不会执行语句，所以增删改也可以看计划；
// ANALYZE 会真实执行，因此只放行没有写入、加锁、危险函数的 SELECT / WITH 查询。
func checkExplainable(sqlText string, analyze bool) error {
	r := database.CheckReadOnlySQL(sqlText)
	verb := strings.ToUpper(r.Verb)
	switch {
	case r.Allowed && explainVerbs[verb]:
		return nil
	case !analyze && r.ReasonCode == "risky_sql" && explainVerbs[verb]:
		return nil
	case r.ReasonCode == "empty_sql":
		return model.Errf(model.CodeExplainRejected, "语句为空")
	case r.ReasonCode == "multi_sql":
		return model.Errf(model.CodeExplainRejected, "一次只能解释一条语句，请选中其中一条")
	case analyze:
		return model.Errf(model.CodeExplainRejected, "ANALYZE 会真实执行语句，只允许没有写入的 SELECT 查询")
	default:
		return model.Errf(model.CodeExplainRejected, "只能解释 SELECT / INSERT / UPDATE / DELETE 语句")
	}
}

// Explain 为 sqlText 生成执行计划（可带或不带 EXPLAIN 前缀）。analyze 为 true 时会真实执行语句：
// 支持只读事务的库放在只读事务里跑，写入会被数据库本身拒绝。
func (s *ExplainService) Explain(ctx context.Context, connID, dbName, sqlText string, analyze bool) (*model.ExplainResult, error) {
	defer logger.Perf("app.Explain.Explain", "conn", connID, "db", dbName)()
	if ctx == nil {
		ctx = context.Background()
	}
	cfg, ok := s.manager.GetConfig(connID)
	if !ok {
		return nil, model.Errf(model.CodeConnNotFound, "连接配置不存在: %s", connID)
	}
	if !database.CapsOf(cfg.Type).Explain {
		return nil, model.Errf(model.CodeExplainUnsupported, "%s 不支持执行计划", DialectLabel(cfg.Type))
	}
	sqlText = database.StripExplainPrefix(sqlText)
	if err := checkExplainable(sqlText, analyze); err != nil {
		return nil, err
	}
	logger.Info("[ExplainService] 执行计划: connID=%s db=%s analyze=%v sql_len=%d", connID, dbName, analyze, len(sqlText))
	ctx, cancel := context.WithTimeout(ctx, statementTimeout)
	defer cancel()

	var res *model.ExplainResult
	err := withSession(ctx, s.manager, connID, dbName, func(session *database.Session, cfg *database.ConnectionConfig) error {
		var db database.Executor = session
		if analyze && database.CapsOf(cfg.Type).ReadOnlyTx {
			tx, err := session.BeginReadOnly()
			if err != nil {
				return err
			}
			defer tx.Rollback()
			db = tx
		}
		var err error
		res, err = database.ExplainQuery(ctx, db, cfg.Type, sqlText, analyze)
		return err
	})
	return res, err
}

// CanExplain 该库是否支持执行计划；CanExplainAnalyze 是否支持 ANALYZE。
func CanExplain(t database.DBType) bool        { return database.CapsOf(t).Explain }
func CanExplainAnalyze(t database.DBType) bool { return database.CapsOf(t).ExplainAnalyze }
