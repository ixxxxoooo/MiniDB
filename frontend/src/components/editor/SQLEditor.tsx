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
import { useUIStore } from "@/stores/ui";
import { useTranslation } from "@/i18n";

interface SQLEditorProps {
  initialSQL?: string;
  onExecute: (sql: string) => void;
  onExecuteAll?: (sql: string) => void;
  onAIAssist?: (sql: string) => void;
  onSave?: (sql: string) => void;
  onSQLChange?: (sql: string) => void;
  loading?: boolean;
  dialect?: "mysql" | "postgres" | "sqlite";
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
}: SQLEditorProps) {
  const [sql, setSQL] = useState(initialSQL);
  const [copied, setCopied] = useState(false);
  const editorRef = useRef<any>(null);
  const monacoRef = useRef<Monaco | null>(null);
  const { resolved: theme } = useThemeStore();
  const { layoutMode } = useUIStore();
  const { t } = useTranslation();
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
      const formatted = sqlFormat(sql, {
        language: dialect === "postgres" ? "postgresql" : dialect,
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
          "flex items-center gap-[var(--size-gap-sm)] px-[var(--size-padding)] py-[var(--size-gap-sm)] border-b flex-shrink-0",
          "bg-[var(--surface-secondary)] border-[var(--border-color)]"
        )}
      >
        <Button
          size="sm"
          className="h-[var(--size-btn-sm)] text-[length:var(--size-font-xs)]"
          onClick={handleExecute}
          disabled={loading || !sql.trim()}
          title={`${t("editor.execute")} (⌘↵)`}
        >
          {loading ? (
            <Loader2 className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)] mr-1 animate-spin" />
          ) : (
            <Play className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)] mr-1" />
          )}
          {t("editor.execute")}
        </Button>

        <div className="w-px h-4 bg-[var(--border-color)] mx-0.5" />

        <Button
          variant="ghost"
          size="icon"
          className="h-[var(--size-btn-sm)] w-[var(--size-btn-sm)]"
          onClick={handleFormat}
          title={`${t("editor.format")} (⌘⇧F)`}
        >
          <AlignLeft className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)]" />
        </Button>

        <Button
          variant="ghost"
          size="icon"
          className="h-[var(--size-btn-sm)] w-[var(--size-btn-sm)]"
          onClick={handleCompress}
          title={t("editor.compress")}
        >
          <Minimize2 className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)]" />
        </Button>

        <Button
          variant="ghost"
          size="icon"
          className="h-[var(--size-btn-sm)] w-[var(--size-btn-sm)]"
          onClick={handleUnescape}
          title={t("editor.unescape")}
        >
          <WrapText className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)]" />
        </Button>

        {onAIAssist && (
          <Button
            variant="ghost"
            size="sm"
            className="h-[var(--size-btn-sm)] text-[length:var(--size-font-xs)]"
            onClick={() => onAIAssist(sql)}
          >
            <Sparkles className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)] mr-1" />
            {t("editor.aiAssist")}
          </Button>
        )}

        <div className="flex-1" />

        {onSave && (
          <Button
            variant="ghost"
            size="icon"
            className="h-[var(--size-btn-sm)] w-[var(--size-btn-sm)]"
            onClick={() => onSave(sql)}
            title={`${t("editor.save")} (⌘S)`}
          >
            <Save className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)]" />
          </Button>
        )}

        <Button
          variant="ghost"
          size="icon"
          className="h-[var(--size-btn-sm)] w-[var(--size-btn-sm)]"
          onClick={handleCopy}
          title={t("common.copy")}
        >
          {copied ? (
            <Check className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)] text-[var(--success)]" />
          ) : (
            <Copy className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)]" />
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
