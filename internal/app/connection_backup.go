package app

import (
	"encoding/json"
	"fmt"
	"minidb/internal/model"
	"os"
	"path/filepath"
	"strings"

	"minidb/internal/database"
	"minidb/internal/logger"
	"time"
)

// 备份文件版本：1 只有连接；2 增加了文件夹（旧版本文件照常导入，连接进「未分组」）。
const connectionsBackupVersion = 2

// ConnectionsBackup 连接配置备份文件结构
type ConnectionsBackup struct {
	Version     int                         `json:"version"`
	ExportedAt  string                      `json:"exportedAt"`
	Connections []database.ConnectionConfig `json:"connections"`
	Folders     []model.ConnectionFolder    `json:"folders,omitempty"`
}

// ImportConnectionsResult 导入连接配置结果
type ImportConnectionsResult struct {
	Imported int `json:"imported"`
	Skipped  int `json:"skipped"`
}

// ExportConnections 导出所有连接配置为备份文件，整个文件用口令加密（AES-256-GCM + PBKDF2）。
// 口令不能为空，也不能太短（见 backupMinPassphraseLen）；确实要明文导出时用 ExportConnectionsPlaintext。
func (s *ConnectionService) ExportConnections(passphrase string) (string, error) {
	defer logger.Perf("app.Connections.ExportConnections")()
	if err := validateBackupPassphrase(passphrase); err != nil {
		return "", err
	}
	return s.exportConnections(passphrase)
}

// ExportConnectionsPlaintext 导出不加密的备份：文件里有全部连接的明文密码和密钥，
// 调用方必须先让用户明确确认。
func (s *ConnectionService) ExportConnectionsPlaintext() (string, error) {
	defer logger.Perf("app.Connections.ExportConnectionsPlaintext")()
	return s.exportConnections("")
}

func (s *ConnectionService) exportConnections(passphrase string) (string, error) {
	conns, err := s.loadConnectionsWithSecrets()
	if err != nil {
		return "", err
	}
	filePath, err := s.getConnectionsExportPath()
	if err != nil {
		return "", err
	}
	if filePath == "" {
		logger.Info("[ConnectionService] 用户取消了导出路径选择")
		return "", nil
	}

	folders, err := s.ListFolders()
	if err != nil {
		return "", err
	}
	backup := ConnectionsBackup{
		Version:     connectionsBackupVersion,
		ExportedAt:  time.Now().UTC().Format(time.RFC3339),
		Connections: conns,
		Folders:     folders,
	}
	data, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化连接配置失败: %w", err)
	}
	if passphrase != "" {
		if data, err = encryptBackup(data, passphrase); err != nil {
			return "", fmt.Errorf("加密备份失败: %w", err)
		}
	}
	if err := writePrivateFile(filePath, data); err != nil {
		return "", fmt.Errorf("写入备份文件失败: %w", err)
	}
	logger.Info("[ConnectionService] 导出连接配置成功: path=%s count=%d encrypted=%v", filePath, len(conns), passphrase != "")
	return filePath, nil
}

// ImportConnections 从备份文件导入连接配置（同 ID 已存在则跳过）。
// 备份已加密时需要提供 passphrase。
func (s *ConnectionService) ImportConnections(passphrase string) (ImportConnectionsResult, error) {
	defer logger.Perf("app.Connections.ImportConnections")()
	result := ImportConnectionsResult{}
	filePath, err := s.getConnectionsImportPath()
	if err != nil {
		return result, err
	}
	if filePath == "" {
		logger.Info("[ConnectionService] 用户取消了导入文件选择")
		return result, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return result, fmt.Errorf("读取备份文件失败: %w", err)
	}

	if isEncryptedBackup(data) {
		if data, err = decryptBackup(data, passphrase); err != nil {
			return result, err
		}
	}

	var backup ConnectionsBackup
	if err := json.Unmarshal(data, &backup); err != nil {
		return result, fmt.Errorf("解析备份文件失败: %w", err)
	}
	if len(backup.Connections) == 0 {
		return result, model.Errf(model.CodeBackupNoConns, "备份文件中没有连接配置")
	}

	known, err := s.importFolders(backup.Folders)
	if err != nil {
		return result, err
	}
	for _, cfg := range backup.Connections {
		if cfg.FolderID != "" && !known[cfg.FolderID] {
			cfg.FolderID = "" // 备份里的文件夹丢了或是旧版本备份：放进「未分组」
		}
		if cfg.ID == "" {
			result.Skipped++
			continue
		}
		if s.connectionExists(cfg.ID) {
			result.Skipped++
			continue
		}
		if err := s.SaveConnection(cfg); err != nil {
			return result, fmt.Errorf("导入连接 %s 失败: %w", cfg.Name, err)
		}
		result.Imported++
	}

	logger.Info("[ConnectionService] 导入连接配置完成: imported=%d skipped=%d path=%s", result.Imported, result.Skipped, filePath)
	return result, nil
}

func (s *ConnectionService) connectionExists(id string) bool {
	var cfg database.ConnectionConfig
	return s.store.Get("connections", id, &cfg) == nil
}

func (s *ConnectionService) getConnectionsExportPath() (string, error) {
	if s.env.get() != nil && s.env.get().Host != nil {
		return s.env.get().Host.SaveFile(SaveOptions{
			Title:    "导出连接配置",
			Filename: fmt.Sprintf("minidb_connections_%s.json", time.Now().Format("20060102_150405")),
		})
	}
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, "Desktop",
		fmt.Sprintf("minidb_connections_%s.json", time.Now().Format("20060102_150405"))), nil
}

func (s *ConnectionService) getConnectionsImportPath() (string, error) {
	if s.env.get() != nil && s.env.get().Host != nil {
		return s.env.get().Host.OpenFile(OpenOptions{
			Title:   "导入连接配置",
			Filters: []FileFilter{{Name: "JSON 文件", Exts: []string{"json"}}},
		})
	}
	return "", model.Errf(model.CodeAppNotReady, "应用未初始化，无法打开文件选择对话框")
}

// importFolders 把备份里的文件夹补进本机：同 ID 已存在的保留本机的名字和顺序，不存在的新建。
// 返回导入后本机上存在的文件夹 ID 集合，导入连接时据此判断 FolderID 是否有效。
func (s *ConnectionService) importFolders(in []model.ConnectionFolder) (map[string]bool, error) {
	have, err := s.ListFolders()
	if err != nil {
		return nil, err
	}
	known := make(map[string]bool, len(have)+len(in))
	next := 0
	for _, f := range have {
		known[f.ID] = true
		next = max(next, f.Order)
	}
	for _, f := range in {
		if f.ID == "" || strings.TrimSpace(f.Name) == "" || known[f.ID] {
			continue
		}
		next++
		f.Name, f.Order = strings.TrimSpace(f.Name), next
		if err := s.store.Put(foldersBucket, f.ID, f); err != nil {
			return nil, fmt.Errorf("导入文件夹失败: %w", err)
		}
		known[f.ID] = true
	}
	return known, nil
}
