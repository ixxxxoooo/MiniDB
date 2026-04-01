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
  History,
  Star,
} from "lucide-react";
import { cn, copyToClipboard } from "@/lib/utils";
import { useThemeStore } from "@/stores/theme";

import { useUIStore } from "@/stores/ui";
import { useTranslation } from "@/i18n";
import { useSQLHistoryStore } from "@/stores/sqlHistory";
import { SQLHistoryPanel } from "./SQLHistoryPanel";

interface SQLEditorProps {
  initialSQL?: string;
  onExecute: (sql: string) => void;
  onExecuteAll?: (sql: string) => void;
  onAIAssist?: (sql: string) => void;
  onSave?: (sql: string) => void;
  onSQLChange?: (sql: string) => void;
  loading?: boolean;
  dialect?: "mysql" | "postgres" | "sqlite" | "tidb" | "starrocks";
  connectionId?: string;
  database?: string;
}

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

function unescapeSQL(sql: string): string {
  return sql
    .replace(/\\n/g, "\n")
    .replace(/\\t/g, "\t")
    .replace(/\\r/g, "\r")
    .replace(/\\\\/g, "\\")
    .replace(/\\'/g, "'");
}

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
  connectionId = "",
  database = "",
}: SQLEditorProps) {
  const [sql, setSQL] = useState(initialSQL);
  const [copied, setCopied] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [favoritesOpen, setFavoritesOpen] = useState(false);
  const editorRef = useRef<any>(null);
  const monacoRef = useRef<Monaco | null>(null);
  const { resolved: theme } = useThemeStore();
  const { layoutMode } = useUIStore();
  const { t } = useTranslation();
  const { addHistory } = useSQLHistoryStore();
  const editorFontSize = layoutMode === "compact" ? 12 : 13;

  useEffect(() => {
    if (initialSQL && initialSQL !== sql) {
      setSQL(initialSQL);
    }
  }, [initialSQL]);

  const handleEditorMount: OnMount = (editor, monaco) => {
    editorRef.current = editor;
    monacoRef.current = monaco;

    editor.addAction({
      id: "execute-sql",
      label: t("editor.execute"),
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
          const offset = model.getOffsetAt(editor.getPosition()!);
          const fullSQL = model.getValue();
          const stmt = getStatementAtOffset(fullSQL, offset);
          onExecute(stmt);
        }
      },
    });

    editor.addAction({
      id: "execute-all-sql",
      label: t("editor.executeAll"),
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

    editor.addAction({
      id: "save-sql",
      label: t("editor.save"),
      keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS],
      run: () => {
        if (onSave) {
          onSave(editor.getModel()?.getValue() || "");
        }
      },
    });

    editor.addAction({
      id: "format-sql",
      label: t("editor.format"),
      keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyMod.Shift | monaco.KeyCode.KeyF],
      run: () => handleFormat(),
    });

    editor.focus();
  };

  const handleFormat = useCallback(() => {
    try {
      // TiDB 和 StarRocks 兼容 MySQL 语法，sql-formatter 使用 mysql 方言
      const formatterDialect = dialect === "postgres" ? "postgresql"
        : (dialect === "tidb" || dialect === "starrocks") ? "mysql"
        : dialect;
      const formatted = sqlFormat(sql, {
        language: formatterDialect,
        tabWidth: 2,
        keywordCase: "preserve",
        linesBetweenQueries: 2,
      });
      setSQL(formatted);
      editorRef.current?.setValue(formatted);
    } catch (e) {
      console.warn("SQL format failed:", e);
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
      addHistory({ sql, database, connectionId });
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
      addHistory({ sql: selectedText.trim(), database, connectionId });
      onExecute(selectedText.trim());
    } else {
      const offset = model.getOffsetAt(editor.getPosition()!);
      const fullSQL = model.getValue();
      const stmt = getStatementAtOffset(fullSQL, offset);
      addHistory({ sql: stmt, database, connectionId });
      onExecute(stmt);
    }
  };

  const handleExecuteAllClick = () => {
    addHistory({ sql, database, connectionId });
    if (onExecuteAll) {
      onExecuteAll(sql);
    } else {
      onExecute(sql);
    }
  };

  // 从历史/收藏中选择SQL，粘贴到编辑器
  const handleInsertSQL = useCallback((selectedSQL: string) => {
    setSQL(selectedSQL);
    editorRef.current?.setValue(selectedSQL);
    editorRef.current?.focus();
  }, []);

  return (
    <div className="flex flex-col h-full relative">
      {/* 顶部工具栏 */}
      <div
        className={cn(
          "flex items-center gap-1.5 px-2 py-1 border-b flex-shrink-0 select-none text-xs",
          "bg-[var(--surface-secondary)] border-[var(--border-color)]"
        )}
      >
        {/* 执行按钮 */}
        <button
          className="flex items-center justify-center gap-1 h-[22px] px-2 rounded-[var(--radius-btn)] bg-[var(--accent)] text-[var(--accent-fg)] hover:opacity-90 transition-opacity disabled:opacity-50 font-medium text-[length:var(--size-font-xs)]"
          onClick={handleExecute}
          disabled={loading || !sql.trim()}
          title={`${t("editor.execute")} (⌘↵)`}
        >
          {loading ? (
            <Loader2 className="h-3 w-3 animate-spin" />
          ) : (
            <Play className="h-3 w-3 fill-current" />
          )}
          <span>{t("editor.execute")}</span>
        </button>

        <div className="w-px h-3.5 bg-[var(--border-color)]" />

        <button
          className="flex items-center justify-center h-[22px] w-[22px] rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] text-[var(--fg-secondary)] hover:text-[var(--fg)] transition-colors"
          onClick={handleFormat}
          title={`${t("editor.format")} (⌘⇧F)`}
        >
          <AlignLeft className="h-3.5 w-3.5" />
        </button>

        <button
          className="flex items-center justify-center h-[22px] w-[22px] rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] text-[var(--fg-secondary)] hover:text-[var(--fg)] transition-colors"
          onClick={handleCompress}
          title={t("editor.compress")}
        >
          <Minimize2 className="h-3.5 w-3.5" />
        </button>

        <button
          className="flex items-center justify-center h-[22px] w-[22px] rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] text-[var(--fg-secondary)] hover:text-[var(--fg)] transition-colors"
          onClick={handleUnescape}
          title={t("editor.unescape")}
        >
          <WrapText className="h-3.5 w-3.5" />
        </button>

        <div className="w-px h-3.5 bg-[var(--border-color)]" />

        {/* 历史按钮 */}
        <button
          className="flex items-center justify-center h-[22px] w-[22px] rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] text-[var(--fg-secondary)] hover:text-[var(--fg)] transition-colors"
          onClick={() => setHistoryOpen(true)}
          title={t("editor.history")}
        >
          <History className="h-3.5 w-3.5" />
        </button>

        {/* 收藏按钮 */}
        <button
          className="flex items-center justify-center h-[22px] w-[22px] rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] text-[var(--fg-secondary)] hover:text-[var(--fg)] transition-colors"
          onClick={() => setFavoritesOpen(true)}
          title={t("editor.favorites")}
        >
          <Star className="h-3.5 w-3.5" />
        </button>

        {onAIAssist && (
          <>
            <div className="w-px h-3.5 bg-[var(--border-color)]" />
            <button
              className="flex items-center justify-center gap-1 h-[22px] px-1.5 rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] text-[var(--fg-secondary)] hover:text-[var(--accent)] transition-colors"
              onClick={() => onAIAssist(sql)}
            >
              <Sparkles className="h-3.5 w-3.5" />
            </button>
          </>
        )}

        <div className="flex-1" />

        {onSave && (
          <button
            className="flex items-center justify-center h-[22px] w-[22px] rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] text-[var(--fg-secondary)] hover:text-[var(--fg)] transition-colors"
            onClick={() => onSave(sql)}
            title={`${t("editor.save")} (⌘S)`}
          >
            <Save className="h-3.5 w-3.5" />
          </button>
        )}

        <button
          className="flex items-center justify-center h-[22px] w-[22px] rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] text-[var(--fg-secondary)] hover:text-[var(--fg)] transition-colors"
          onClick={handleCopy}
          title={t("common.copy")}
        >
          {copied ? (
            <Check className="h-3.5 w-3.5 text-[var(--success)]" />
          ) : (
            <Copy className="h-3.5 w-3.5" />
          )}
        </button>
      </div>

      {/* SQL 历史面板 */}
      <SQLHistoryPanel
        open={historyOpen}
        onClose={() => setHistoryOpen(false)}
        onSelect={handleInsertSQL}
        mode="history"
      />

      {/* SQL 收藏面板 */}
      <SQLHistoryPanel
        open={favoritesOpen}
        onClose={() => setFavoritesOpen(false)}
        onSelect={handleInsertSQL}
        mode="favorites"
      />

      {/* Monaco 编辑区 */}
      <div className="flex-1 min-h-0 bg-[var(--surface)]">
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
            fontSize: editorFontSize,
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
