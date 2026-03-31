import React, { useState, useRef, useCallback, useEffect } from "react";
import Editor, { OnMount, type Monaco } from "@monaco-editor/react";
import { format as sqlFormat } from "sql-formatter";
import {
  Play,
  Sparkles,
  Copy,
  Check,
  Loader2,
  AlignLeft,
  Minimize2,
  Save,
  WrapText,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn, copyToClipboard } from "@/lib/utils";
import { useThemeStore } from "@/stores/theme";

interface SQLEditorProps {
  initialSQL?: string;
  onExecute: (sql: string) => void;
  onExecuteAll?: (sql: string) => void;
  onAIAssist?: (sql: string) => void;
  onSave?: (sql: string) => void;
  /** SQL 内容变化时回调，用于同步到外部 store */
  onSQLChange?: (sql: string) => void;
  loading?: boolean;
  /** 数据库类型，用于 SQL 格式化方言选择 */
  dialect?: "mysql" | "postgres" | "sqlite";
}

/**
 * 按分号拆分 SQL 语句，忽略字符串内的分号。
 * 返回非空的语句数组。
 */
function splitStatements(sql: string): string[] {
  const results: string[] = [];
  let current = "";
  let inString = false;
  let stringChar = "";

  for (let i = 0; i < sql.length; i++) {
    const ch = sql[i];
    if (inString) {
      current += ch;
      if (ch === stringChar && sql[i - 1] !== "\\") {
        inString = false;
      }
    } else if (ch === "'" || ch === '"') {
      current += ch;
      inString = true;
      stringChar = ch;
    } else if (ch === ";") {
      const trimmed = current.trim();
      if (trimmed) results.push(trimmed);
      current = "";
    } else {
      current += ch;
    }
  }
  const trimmed = current.trim();
  if (trimmed) results.push(trimmed);
  return results;
}

/**
 * 获取光标所在位置对应的 SQL 语句（按分号分割的逻辑块）
 */
function getStatementAtOffset(sql: string, offset: number): string {
  const statements = splitStatements(sql);
  let pos = 0;
  for (const stmt of statements) {
    const start = sql.indexOf(stmt, pos);
    const end = start + stmt.length;
    if (offset >= start && offset <= end + 1) {
      return stmt;
    }
    pos = end + 1;
  }
  return statements[statements.length - 1] || sql;
}

/**
 * 反转义 SQL 字符串（将 \\n, \\t, \\\\ 等转回实际字符）
 */
function unescapeSQL(sql: string): string {
  return sql
    .replace(/\\n/g, "\n")
    .replace(/\\t/g, "\t")
    .replace(/\\r/g, "\r")
    .replace(/\\\\/g, "\\")
    .replace(/\\'/g, "'");
}

/**
 * 压缩 SQL（去除多余空白，合并为单行）
 */
function compressSQL(sql: string): string {
  return sql.replace(/\s+/g, " ").replace(/\s*;\s*/g, ";\n").trim();
}

export function SQLEditor({
  initialSQL = "",
  onExecute,
  onExecuteAll,
  onAIAssist,
  onSave,
  onSQLChange,
  loading,
  dialect = "mysql",
}: SQLEditorProps) {
  const [sql, setSQL] = useState(initialSQL);
  const [copied, setCopied] = useState(false);
  const editorRef = useRef<any>(null);
  const monacoRef = useRef<Monaco | null>(null);
  const { resolved: theme } = useThemeStore();

  // 当 initialSQL 变化时同步
  useEffect(() => {
    if (initialSQL && initialSQL !== sql) {
      setSQL(initialSQL);
    }
  }, [initialSQL]);

  const handleEditorMount: OnMount = (editor, monaco) => {
    editorRef.current = editor;
    monacoRef.current = monaco;

    // Cmd+Enter: 执行当前语句或选中部分
    editor.addAction({
      id: "execute-sql",
      label: "执行 SQL",
      keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.Enter],
      run: () => {
        const selection = editor.getSelection();
        const model = editor.getModel();
        if (!model) return;

        const selectedText = selection && !selection.isEmpty()
          ? model.getValueInRange(selection)
          : "";

        if (selectedText.trim()) {
          onExecute(selectedText.trim());
        } else {
          // 获取光标所在的语句
          const offset = model.getOffsetAt(editor.getPosition()!);
          const fullSQL = model.getValue();
          const stmt = getStatementAtOffset(fullSQL, offset);
          onExecute(stmt);
        }
      },
    });

    // Cmd+Shift+Enter: 执行所有语句
    editor.addAction({
      id: "execute-all-sql",
      label: "执行所有 SQL",
      keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyMod.Shift | monaco.KeyCode.Enter],
      run: () => {
        const value = editor.getModel()?.getValue() || "";
        if (onExecuteAll) {
          onExecuteAll(value);
        } else {
          onExecute(value);
        }
      },
    });

    // Cmd+S: 保存
    editor.addAction({
      id: "save-sql",
      label: "保存 SQL",
      keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS],
      run: () => {
        if (onSave) {
          onSave(editor.getModel()?.getValue() || "");
        }
      },
    });

    // Cmd+Shift+F: 格式化
    editor.addAction({
      id: "format-sql",
      label: "格式化 SQL",
      keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyMod.Shift | monaco.KeyCode.KeyF],
      run: () => handleFormat(),
    });

    editor.focus();
  };

  const handleFormat = useCallback(() => {
    try {
      const formatted = sqlFormat(sql, {
        language: dialect === "postgres" ? "postgresql" : dialect,
        tabWidth: 2,
        keywordCase: "preserve",
        linesBetweenQueries: 2,
      });
      setSQL(formatted);
      editorRef.current?.setValue(formatted);
    } catch (e) {
      console.warn("SQL 格式化失败:", e);
    }
  }, [sql, dialect]);

  const handleCompress = useCallback(() => {
    const compressed = compressSQL(sql);
    setSQL(compressed);
    editorRef.current?.setValue(compressed);
  }, [sql]);

  const handleUnescape = useCallback(() => {
    const unescaped = unescapeSQL(sql);
    setSQL(unescaped);
    editorRef.current?.setValue(unescaped);
  }, [sql]);

  const handleCopy = async () => {
    await copyToClipboard(sql);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleExecute = () => {
    const editor = editorRef.current;
    if (!editor) {
      onExecute(sql);
      return;
    }
    const selection = editor.getSelection();
    const model = editor.getModel();
    if (!model) return;

    const selectedText = selection && !selection.isEmpty()
      ? model.getValueInRange(selection)
      : "";

    if (selectedText.trim()) {
      onExecute(selectedText.trim());
    } else {
      const offset = model.getOffsetAt(editor.getPosition()!);
      const fullSQL = model.getValue();
      const stmt = getStatementAtOffset(fullSQL, offset);
      onExecute(stmt);
    }
  };

  const handleExecuteAllClick = () => {
    if (onExecuteAll) {
      onExecuteAll(sql);
    } else {
      onExecute(sql);
    }
  };

  return (
    <div className="flex flex-col h-full">
      {/* 工具栏 */}
      <div
        className={cn(
          "flex items-center gap-1 px-3 py-1.5 border-b flex-shrink-0",
          "bg-[var(--surface-secondary)] border-[var(--border-color)]"
        )}
      >
        <Button
          size="sm"
          className="h-7 text-xs"
          onClick={handleExecute}
          disabled={loading || !sql.trim()}
          title="执行 (⌘↵ 当前语句 / ⌘⇧↵ 全部)"
        >
          {loading ? (
            <Loader2 className="h-3 w-3 mr-1 animate-spin" />
          ) : (
            <Play className="h-3 w-3 mr-1" />
          )}
          执行
        </Button>

        <div className="w-px h-4 bg-[var(--border-color)] mx-1" />

        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7"
          onClick={handleFormat}
          title="格式化 (⌘⇧F)"
        >
          <AlignLeft className="h-3 w-3" />
        </Button>

        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7"
          onClick={handleCompress}
          title="压缩"
        >
          <Minimize2 className="h-3 w-3" />
        </Button>

        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7"
          onClick={handleUnescape}
          title="反转义"
        >
          <WrapText className="h-3 w-3" />
        </Button>

        {onAIAssist && (
          <Button
            variant="ghost"
            size="sm"
            className="h-7 text-xs"
            onClick={() => onAIAssist(sql)}
          >
            <Sparkles className="h-3 w-3 mr-1" />
            AI 辅助
          </Button>
        )}

        <div className="flex-1" />

        {onSave && (
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7"
            onClick={() => onSave(sql)}
            title="保存 (⌘S)"
          >
            <Save className="h-3 w-3" />
          </Button>
        )}

        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7"
          onClick={handleCopy}
          title="复制"
        >
          {copied ? (
            <Check className="h-3 w-3 text-[var(--success)]" />
          ) : (
            <Copy className="h-3 w-3" />
          )}
        </Button>
      </div>

      {/* Monaco 编辑区 */}
      <div className="flex-1 min-h-0">
        <Editor
          defaultLanguage="sql"
          value={sql}
          onChange={(value) => {
            const v = value || "";
            setSQL(v);
            onSQLChange?.(v);
          }}
          onMount={handleEditorMount}
          theme={theme === "dark" ? "vs-dark" : "vs"}
          options={{
            fontSize: 13,
            fontFamily: "'SF Mono', Menlo, Monaco, 'Courier New', monospace",
            lineNumbers: "on",
            minimap: { enabled: false },
            scrollBeyondLastLine: false,
            automaticLayout: true,
            tabSize: 2,
            wordWrap: "on",
            padding: { top: 8, bottom: 8 },
            suggestOnTriggerCharacters: true,
            quickSuggestions: true,
            renderLineHighlight: "line",
            smoothScrolling: true,
            cursorBlinking: "smooth",
            cursorSmoothCaretAnimation: "on",
            folding: true,
            lineDecorationsWidth: 8,
            overviewRulerBorder: false,
          }}
        />
      </div>
    </div>
  );
}
