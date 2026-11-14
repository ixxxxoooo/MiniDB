package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"minidb/internal/ai"
	"minidb/internal/database"
	"minidb/internal/logger"
	"minidb/internal/schemaindex"
	"minidb/internal/storage"
)

// defaultAIConfig 没有保存过配置时的默认值。
func defaultAIConfig() ai.Config {
	return ai.Config{BaseURL: "https://api.openai.com/v1", Model: "gpt-4o"}
}

// AIService AI 助手服务：对话（带工具的多轮循环）、会话历史与表文档生成。
//
// @author ixxxxoooo
type AIService struct {
	env     envRef
	client  *ai.Client
	manager *database.Manager
	store   *storage.Store
	query   *QueryService   // 供 sql_readonly_execute 等工具执行只读查询
	explain *ExplainService // sql_explain_plan 工具；为空时按需用 manager 临时建一个
	schema  *schemaindex.Manager

	configMu          sync.Mutex
	configFingerprint string // 上次加载的配置指纹：没变化就跳过重复读取与解密

	cancelMu sync.Mutex
	cancels  map[string]context.CancelFunc // requestID → 取消函数
}

// NewAIService 创建 AI 服务，并从存储加载 AI 配置。
func NewAIService(manager *database.Manager, store *storage.Store, query *QueryService, schema *schemaindex.Manager) *AIService {
	cfg := defaultAIConfig()
	var stored ai.Config
	if err := store.Get("settings", "ai_config", &stored); err == nil {
		if loaded, err := openStoredConfig(store, stored); err != nil {
			logger.Error("[AIService] 解密 AI 配置失败: %v", err)
		} else {
			cfg = loaded
		}
	}
	return &AIService{
		client: ai.NewClient(&cfg), manager: manager, store: store, query: query, schema: schema,
		cancels: map[string]context.CancelFunc{},
	}
}

// openStoredConfig 解密存储里的配置；明文存下来的旧配置顺便加密回写。
func openStoredConfig(store *storage.Store, cfg ai.Config) (ai.Config, error) {
	needsMigration := aiConfigNeedsEncryption(cfg.APIKey, cfg.Headers)
	if err := decryptAIClientConfig(store, &cfg); err != nil {
		return ai.Config{}, err
	}
	if needsMigration {
		if encrypted := cfg; encryptAIClientConfig(store, &encrypted) == nil {
			if err := store.Put("settings", "ai_config", encrypted); err != nil {
				logger.Warn("[AIService] 迁移 AI 配置为密文失败（下次读取会重试）: %v", err)
			}
		}
	}
	return cfg, nil
}

// SetEnv 注入平台与事件总线。
func (s *AIService) SetEnv(env *Env) { s.env.set(env) }

// ReloadConfig 重新加载 AI 配置。存储里的配置没变化时直接返回，避免每次调用都做 AES-GCM 解密。
func (s *AIService) ReloadConfig() {
	defer logger.Perf("app.AI.ReloadConfig")()
	s.configMu.Lock()
	defer s.configMu.Unlock()
	var stored ai.Config
	if err := s.store.Get("settings", "ai_config", &stored); err != nil {
		return
	}
	fp := configFingerprint(&stored)
	if fp == s.configFingerprint {
		return
	}
	cfg, err := openStoredConfig(s.store, stored)
	if err != nil {
		logger.Error("[AIService] 解密 AI 配置失败: %v", err)
		return
	}
	s.client.UpdateConfig(&cfg)
	s.configFingerprint = fp
}

// configFingerprint 存储形态（密钥仍是密文）下配置的指纹。
func configFingerprint(cfg *ai.Config) string {
	data, _ := json.Marshal(cfg) // map 键按字典序输出，所以相同配置的指纹稳定
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// friendlyAIError 把网关限流转成用户能看懂的提示；其余错误原样返回。
func friendlyAIError(err error) error {
	if ai.IsRateLimit(err) {
		return fmt.Errorf("AI 服务当前繁忙（限流），请稍后重试")
	}
	return err
}

// GenerateTableDoc 为一张表生成 Markdown 文档。
func (s *AIService) GenerateTableDoc(connID, dbName, tableName string) (string, error) {
	defer logger.Perf("app.AI.GenerateTableDoc", "conn", connID, "db", dbName, "table", tableName)()
	s.ReloadConfig()
	schema, err := s.schema.GetSchema(context.Background(), connID, dbName)
	if err != nil {
		return "", err
	}
	doc, err := s.client.GenerateTableDoc(context.Background(), schema, tableName)
	if err != nil {
		return "", friendlyAIError(err)
	}
	return doc, nil
}

// registerCancel 登记一次请求的取消函数，返回释放函数（请求结束时调用）。
// 同一 requestID 重复登记时先取消旧的。
func (s *AIService) registerCancel(requestID string, cancel context.CancelFunc) (release func()) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return func() {}
	}
	s.cancelMu.Lock()
	if old := s.cancels[requestID]; old != nil {
		old()
	}
	s.cancels[requestID] = cancel
	s.cancelMu.Unlock()
	return func() {
		s.cancelMu.Lock()
		delete(s.cancels, requestID)
		s.cancelMu.Unlock()
	}
}

// CancelChatStream 取消指定 requestID 的进行中的对话请求。
func (s *AIService) CancelChatStream(requestID string) {
	s.cancelMu.Lock()
	cancel := s.cancels[strings.TrimSpace(requestID)]
	s.cancelMu.Unlock()
	if cancel != nil {
		logger.Info("[AIService] 取消对话: requestID=%s", requestID)
		cancel()
	}
}
