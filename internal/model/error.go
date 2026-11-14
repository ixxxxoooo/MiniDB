package model

import (
	"errors"
	"fmt"
	"strings"
)

// Error 是带错误码的错误。Error() 给日志和不认识错误码的调用方用，是中文原文；
// 界面按 Code 和 Args 到语言包里取当前语言的文字，取不到再退回 Error()。
type Error struct {
	Code  string // 错误码，对应语言包里 "err" + 首字母大写的 Code
	Args  []any  // 填进语言包文案的参数，依次是 {a0}、{a1}……
	Cause error  // 底层错误（驱动、系统调用），可为空
	msg   string
}

// Error 返回中文原文。
func (e *Error) Error() string { return e.msg }

// Unwrap 返回底层错误，使 errors.Is / errors.As 能穿过它。
func (e *Error) Unwrap() error { return e.Cause }

// Errf 创建一个带错误码的错误；format 是中文原文，args 同时用于原文和语言包文案。
func Errf(code, format string, args ...any) *Error {
	return &Error{Code: code, Args: args, msg: fmt.Sprintf(format, args...)}
}

// Wrapf 与 fmt.Errorf 的 %w 用法一致：format 里写 %w，最后一个参数是被包装的底层错误。
// 语言包文案里底层错误的文字作为最后一个参数。
func Wrapf(code, format string, args ...any) *Error {
	e := &Error{Code: code, msg: fmt.Sprintf(strings.ReplaceAll(format, "%w", "%v"), args...)}
	e.Args = append([]any(nil), args...)
	if n := len(args); n > 0 {
		if cause, ok := args[n-1].(error); ok {
			e.Cause = cause
			e.Args[n-1] = cause.Error()
		}
	}
	return e
}

// CodeOf 返回错误链里第一个带码错误的错误码；没有则返回空串。
func CodeOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

// 错误码。新增时同时在 locales 的两个语言文件里加 "err"+Code 的词条。
const (
	CodeConnNotFound              = "connNotFound"
	CodeFolderNotFound            = "folderNotFound"
	CodeFolderNameEmpty           = "folderNameEmpty"
	CodeFolderNameDup             = "folderNameDup"
	CodeExplainUnsupported        = "explainUnsupported"
	CodeExplainRejected           = "explainRejected"
	CodeExplainAnalyzeUnsupported = "explainAnalyzeUnsupported"
	CodeConnIDRequired            = "connIDRequired"
	CodeGetConnFailed             = "getConnFailed"
	CodeDBOpenFailed              = "dbOpenFailed"
	CodeDBConnectFailed           = "dbConnectFailed"
	CodeDBConnectNamed            = "dbConnectNamedFailed"
	CodeConnTestFailed            = "connTestFailed"
	CodeSwitchDBFailed            = "switchDBFailed"
	CodeDisconnected              = "disconnected"
	CodeUnsupportedType           = "unsupportedType"
	CodeUnsupportedUsers          = "unsupportedUsers"
	CodeUnsupportedRoutines       = "unsupportedRoutines"
	CodeUnsupportedProc           = "unsupportedProcess"
	CodeUnsupportedKill           = "unsupportedKill"
	CodeAppNotReady               = "appNotReady"
	CodeNoDBSelected              = "noDBSelected"
	CodeReadOnlyConn              = "readOnlyConn"
	CodeTableNameEmpty            = "tableNameEmpty"
	CodeHostRequired              = "hostRequired"
	CodePortInvalid               = "portInvalid"
	CodeSSHHostUser               = "sshHostUserRequired"
	CodeSSHPortInvalid            = "sshPortInvalid"
	CodeSSHKeyRequired            = "sshKeyRequired"
	CodeSQLiteNoSSH               = "sqliteNoSSH"
	CodeFilePathEmpty             = "filePathEmpty"
	CodePKRequired                = "pkRequired"
	CodeSQLExecFailed             = "sqlExecFailed"
	CodeBackupNeedPass            = "backupPassphraseRequired"
	CodeBackupWrongPass           = "backupPassphraseWrong"
	CodeBackupCorrupt             = "backupCorrupt"
	CodeBackupNoConns             = "backupNoConnections"
	CodeBackupWeakPass            = "backupPassphraseWeak"
	CodeAIBaseURLInvalid          = "aiBaseUrlInvalid"
	CodeAIBaseURLInsecure         = "aiBaseUrlInsecure"
)
