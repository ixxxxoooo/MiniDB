import React, { useState, useRef, useCallback, useEffect, useMemo } from "react";
import Editor, { DiffEditor, OnMount, type Monaco } from "@monaco-editor/react";
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
  FolderOpen,
  Eye,
  Wand2,
  X,
} from "lucide-react";
import { cn, copyToClipboard } from "@/lib/utils";
import { useThemeStore } from "@/stores/theme";
import * as AIService from "../../../wailsjs/go/services/AIService";
import * as DatabaseService from "../../../wailsjs/go/services/DatabaseService";

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
  serverVersion?: string;
}

interface AITargetRange {
  startLineNumber: number;
  startColumn: number;
  endLineNumber: number;
  endColumn: number;
}

interface AIAssistResult {
  mode: "check" | "generate";
  source: "selection" | "full";
  originalText: string;
  suggestedSQL: string;
  explanation: string;
  hasError?: boolean;
  targetRange: AITargetRange;
}

function applyMonacoSQLTheme(monaco: Monaco, mode: "light" | "dark") {
  monaco.editor.defineTheme("tpai-sql-light", {
    base: "vs",
    inherit: true,
    rules: [
      { token: "keyword", foreground: "4A8FD1" },
      { token: "keyword.sql", foreground: "4A8FD1" },
      { token: "comment", foreground: "0A9843" },
      { token: "string", foreground: "C53A31" },
      { token: "number", foreground: "2A35F0" },
      { token: "predefined.sql", foreground: "D38718" },
    ],
    colors: {
      "editor.background": "#FFFFFF",
      "editor.foreground": "#1D1D1F",
      "editorLineNumber.foreground": "#9A9AA0",
      "editorLineNumber.activeForeground": "#616167",
      "editorCursor.foreground": "#FF2D2D",
      "editor.lineHighlightBackground": "#F6F6F8",
      "editor.selectionBackground": "#DCEBFF",
      "editor.inactiveSelectionBackground": "#EAF2FF",
    },
  });
  monaco.editor.defineTheme("tpai-sql-dark", {
    base: "vs-dark",
    inherit: true,
    rules: [
      { token: "keyword", foreground: "79B8F2" },
      { token: "keyword.sql", foreground: "79B8F2" },
      { token: "comment", foreground: "56D188" },
      { token: "string", foreground: "F08C7A" },
      { token: "number", foreground: "A6A9FF" },
      { token: "predefined.sql", foreground: "E4B96D" },
    ],
    colors: {
      "editor.background": "#252626",
      "editor.foreground": "#F5F5F7",
      "editorLineNumber.foreground": "#7A7A80",
      "editorLineNumber.activeForeground": "#C9C9CF",
      "editorCursor.foreground": "#FF6565",
      "editor.lineHighlightBackground": "#303131",
      "editor.selectionBackground": "#1C3D66",
      "editor.inactiveSelectionBackground": "#2B3644",
    },
  });
  monaco.editor.setTheme(mode === "dark" ? "tpai-sql-dark" : "tpai-sql-light");
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

function toFormatterDialect(dialect: SQLEditorProps["dialect"]) {
  if (dialect === "postgres") return "postgresql";
  if (dialect === "tidb" || dialect === "starrocks") return "mysql";
  return dialect || "mysql";
}

function formatSQLSafe(sql: string, dialect: SQLEditorProps["dialect"]) {
  try {
    return sqlFormat(sql, {
      language: toFormatterDialect(dialect),
      tabWidth: 2,
      keywordCase: "preserve",
      linesBetweenQueries: 1,
    });
  } catch {
    return sql;
  }
}

function looksLikeSQL(text: string): boolean {
  const trimmed = text.trim();
  if (!trimmed) return false;
  const upper = trimmed.toUpperCase();
  const starters = [
    "SELECT", "WITH", "INSERT", "UPDATE", "DELETE", "REPLACE", "UPSERT",
    "CREATE", "ALTER", "DROP", "TRUNCATE", "SHOW", "DESCRIBE", "DESC",
    "EXPLAIN", "CALL", "SET", "USE", "BEGIN", "COMMIT", "ROLLBACK",
  ];
  if (starters.some((kw) => upper.startsWith(kw))) return true;
  return /\b(SELECT|FROM|WHERE|JOIN|GROUP BY|ORDER BY|LIMIT)\b/i.test(trimmed);
}

function dialectToLabel(dialect: SQLEditorProps["dialect"], locale: "zh-CN" | "en-US") {
  switch (dialect) {
    case "postgres":
      return "PostgreSQL";
    case "sqlite":
      return "SQLite";
    case "tidb":
      return locale === "en-US" ? "TiDB (MySQL compatible)" : "TiDB (MySQL 兼容)";
    case "starrocks":
      return "StarRocks";
    default:
      return "MySQL";
  }
}

function extractJSONFromText(text: string): any | null {
  const direct = text.trim();
  try {
    return JSON.parse(direct);
  } catch {}
  const codeMatch = direct.match(/```json\s*([\s\S]*?)```/i);
  if (codeMatch) {
    try {
      return JSON.parse(codeMatch[1].trim());
    } catch {}
  }
  const objectMatch = direct.match(/\{[\s\S]*\}/);
  if (objectMatch) {
    try {
      return JSON.parse(objectMatch[0]);
    } catch {}
  }
  return null;
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
  serverVersion = "",
}: SQLEditorProps) {
  const [sql, setSQL] = useState(initialSQL);
  const [copied, setCopied] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [favoritesOpen, setFavoritesOpen] = useState(false);
  const [aiBusy, setAIBusy] = useState(false);
  const [aiError, setAIError] = useState("");
  const [aiResult, setAIResult] = useState<AIAssistResult | null>(null);
  const [aiPreviewOpen, setAIPreviewOpen] = useState(false);
  const editorRef = useRef<any>(null);
  const monacoRef = useRef<Monaco | null>(null);
  const completionProviderDisposableRef = useRef<{ dispose: () => void } | null>(null);
  const tableNamesRef = useRef<string[]>([]);
  const tableNamesCacheRef = useRef<Record<string, string[]>>({});
  const { resolved: theme } = useThemeStore();
  const { layoutMode } = useUIStore();
  const { t, locale } = useTranslation();
  const { history, addHistory, addFavorite, removeFavoriteBySQL } = useSQLHistoryStore();
  const editorFontSize = layoutMode === "compact" ? 11 : 12;
  const editorLineHeight = Math.round(editorFontSize * 1.2);
  const isCurrentSQLFavorited = useMemo(
    () => history.some((h) => h.favorite && h.sql.trim() === sql.trim()),
    [history, sql]
  );

  useEffect(() => {
    if (initialSQL && initialSQL !== sql) {
      setSQL(initialSQL);
    }
  }, [initialSQL]);

  useEffect(() => {
    let cancelled = false;
    // 加载当前库表名并缓存，供编辑器表名补全与模糊匹配
    const loadTables = async () => {
      if (!connectionId || !database) {
        tableNamesRef.current = [];
        return;
      }
      const cacheKey = `${connectionId}::${database}`;
      const cached = tableNamesCacheRef.current[cacheKey];
      if (cached && cached.length > 0) {
        tableNamesRef.current = cached;
        return;
      }
      try {
        const tables = await DatabaseService.GetTables(connectionId, database);
        if (cancelled) return;
        const names = (tables || []).map((item: any) => String(item?.name || "")).filter(Boolean);
        tableNamesCacheRef.current[cacheKey] = names;
        tableNamesRef.current = names;
      } catch {
        if (!cancelled) tableNamesRef.current = [];
      }
    };
    loadTables();
    return () => {
      cancelled = true;
    };
  }, [connectionId, database]);

  const handleEditorMount: OnMount = (editor, monaco) => {
    editorRef.current = editor;
    monacoRef.current = monaco;
    applyMonacoSQLTheme(monaco, theme === "dark" ? "dark" : "light");

    // 注册 SQL 表名补全：支持前缀与模糊匹配（FROM/JOIN/UPDATE 等场景）
    completionProviderDisposableRef.current?.dispose();
    completionProviderDisposableRef.current = monaco.languages.registerCompletionItemProvider("sql", {
      triggerCharacters: [" ", ".", "`", '"'],
      provideCompletionItems: (model: any, position: any) => {
        const linePrefix = model.getLineContent(position.lineNumber).slice(0, position.column - 1);
        const matched = linePrefix.match(/([a-zA-Z0-9_]+)$/);
        const input = (matched?.[1] || "").toLowerCase();
        const startColumn = matched ? position.column - matched[1].length : position.column;
        const keywordContext = /\b(from|join|update|into|table|desc|describe|show\s+create\s+table)\s+[`"\[]?[a-zA-Z0-9_]*$/i.test(linePrefix);
        if (!input && !keywordContext) {
          return { suggestions: [] };
        }

        const scored = tableNamesRef.current
          .map((name) => {
            const lower = name.toLowerCase();
            let score = 0;
            if (!input) score = 30;
            if (lower === input) score += 120;
            if (input && lower.startsWith(input)) score += 90;
            if (input && lower.includes(input)) score += 50;
            if (input) {
              let i = 0;
              for (const ch of lower) {
                if (i < input.length && ch === input[i]) i++;
              }
              if (i > 1) score += i * 6;
            }
            return { name, score };
          })
          .filter((item) => item.score > 0)
          .sort((a, b) => b.score - a.score || a.name.localeCompare(b.name))
          .slice(0, 50);

        return {
          suggestions: scored.map((item) => ({
            label: item.name,
            kind: monaco.languages.CompletionItemKind.Field,
            insertText: item.name,
            detail: database ? `表名 · ${database}` : "表名",
            range: {
              startLineNumber: position.lineNumber,
              endLineNumber: position.lineNumber,
              startColumn,
              endColumn: position.column,
            },
          })),
        };
      },
    });

    editor.addAction({
      id: "execute-sql",
      label: t("editor.execute"),
      keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.Enter],
      run: () => {
        handleExecute();
      },
    });

    editor.addAction({
      id: "execute-all-sql",
      label: t("editor.executeAll"),
      keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyMod.Shift | monaco.KeyCode.Enter],
      run: () => {
        handleExecuteAllClick();
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

    editor.addAction({
      id: "toggle-favorite-sql",
      label: t("editor.favorite"),
      keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyD],
      run: () => handleToggleFavoriteCurrent(),
    });

    editor.addAction({
      id: "ai-assist-sql",
      label: t("editor.aiAssist"),
      keybindings: [monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyI],
      run: () => runAIAssist(),
    });

    editor.focus();
  };

  useEffect(() => () => {
    completionProviderDisposableRef.current?.dispose();
    completionProviderDisposableRef.current = null;
  }, []);

  useEffect(() => {
    const monaco = monacoRef.current;
    if (!monaco) return;
    applyMonacoSQLTheme(monaco, theme === "dark" ? "dark" : "light");
  }, [theme]);

  const handleFormat = useCallback(() => {
    const formatted = formatSQLSafe(sql, dialect);
    setSQL(formatted);
    editorRef.current?.setValue(formatted);
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

  const handleToggleFavoriteCurrent = useCallback(() => {
    if (!sql.trim()) return;
    if (isCurrentSQLFavorited) {
      removeFavoriteBySQL(sql);
      return;
    }
    addFavorite({ sql, database, connectionId });
  }, [sql, isCurrentSQLFavorited, removeFavoriteBySQL, addFavorite, database, connectionId]);

  const getTargetText = useCallback((): { text: string; source: "selection" | "full"; range: AITargetRange } | null => {
    const editor = editorRef.current;
    const model = editor?.getModel();
    if (!editor || !model) return null;
    const selection = editor.getSelection();
    if (selection && !selection.isEmpty()) {
      return {
        text: model.getValueInRange(selection),
        source: "selection",
        range: {
          startLineNumber: selection.startLineNumber,
          startColumn: selection.startColumn,
          endLineNumber: selection.endLineNumber,
          endColumn: selection.endColumn,
        },
      };
    }
    const fullRange = model.getFullModelRange();
    return {
      text: model.getValue(),
      source: "full",
      range: {
        startLineNumber: fullRange.startLineNumber,
        startColumn: fullRange.startColumn,
        endLineNumber: fullRange.endLineNumber,
        endColumn: fullRange.endColumn,
      },
    };
  }, []);

  const runAIAssist = useCallback(async () => {
    const target = getTargetText();
    if (!target || !target.text.trim()) {
      setAIError(t("editor.aiNeedInput"));
      return;
    }
    setAIBusy(true);
    setAIError("");
    setAIResult(null);
    onAIAssist?.(target.text);

    const dbDialect = dialectToLabel(dialect, locale);
    const dbVersion = (serverVersion || "").trim();
    const dbVersionHint = dbVersion
      ? (locale === "en-US" ? `\nCurrent database version: ${dbVersion}` : `\n当前数据库版本: ${dbVersion}`)
      : (locale === "en-US" ? "\nCurrent database version: unknown" : "\n当前数据库版本: 未知");
    const targetText = target.text.trim();
    try {
      if (!looksLikeSQL(targetText)) {
        // 文本语义识别：自然语言转 SQL
        const nlResult = await AIService.NaturalLanguageToSQL(connectionId, database, `${targetText}${dbVersionHint}`);
        const generated = String(nlResult?.sql || "").trim();
        if (!generated) {
          throw new Error(t("editor.aiGenerateEmpty"));
        }
        setAIResult({
          mode: "generate",
          source: target.source,
          originalText: targetText,
          suggestedSQL: generated,
          explanation: String(nlResult?.explanation || t("editor.aiGenerateDone")),
          targetRange: target.range,
        });
        setAIPreviewOpen(true);
        return;
      }

      // SQL 语法检查与修复：携带数据库方言上下文
      const prompt = locale === "en-US"
        ? `You are a senior SQL syntax checker and fixer.
Current SQL dialect: ${dbDialect}${dbVersionHint}
Tasks:
1) Check whether the SQL below has syntax issues or dialect incompatibilities.
2) If there are issues, output fixed SQL; if no issues, return the original SQL.
3) Provide a brief explanation.
4) Return STRICT JSON only, with no extra text.

JSON format:
{
  "hasError": true/false,
  "sql": "fixed or original SQL",
  "explanation": "issue and fix rationale"
}

SQL:
\`\`\`sql
${targetText}
\`\`\``
        : `你是一个资深 SQL 语法检查器与修复助手。
当前数据库方言: ${dbDialect}${dbVersionHint}
任务:
1) 检查下面 SQL 是否存在语法问题、方言不兼容问题。
2) 如果有问题，输出修复后的 SQL；如果无问题，原样返回。
3) 输出简短说明。
4) 严格返回 JSON，不要包含额外文本。

返回 JSON 格式:
{
  "hasError": true/false,
  "sql": "修复后或原始 SQL",
  "explanation": "问题说明和修复理由"
}

SQL:
\`\`\`sql
${targetText}
\`\`\``;
      const resp = await AIService.ChatAI(connectionId, database, [{ role: "user", content: prompt }] as any);
      const content = String(resp?.content || "");
      const parsed = extractJSONFromText(content);
      if (!parsed || !parsed.sql) {
        throw new Error(t("editor.aiCheckFormatError"));
      }

      const fixedSQL = String(parsed.sql || "").trim();
      setAIResult({
        mode: "check",
        source: target.source,
        originalText: targetText,
        suggestedSQL: fixedSQL || targetText,
        explanation: String(parsed.explanation || t("editor.aiCheckDone")),
        hasError: Boolean(parsed.hasError),
        targetRange: target.range,
      });
      setAIPreviewOpen(true);
    } catch (e: any) {
      setAIError(e?.message || t("editor.aiFailed"));
    } finally {
      setAIBusy(false);
    }
  }, [getTargetText, onAIAssist, dialect, connectionId, database, t, locale, serverVersion]);

  const applyAIResult = useCallback(() => {
    if (!aiResult) return;
    const editor = editorRef.current;
    const monaco = monacoRef.current;
    if (!editor || !monaco) return;
    const targetRange = new monaco.Range(
      aiResult.targetRange.startLineNumber,
      aiResult.targetRange.startColumn,
      aiResult.targetRange.endLineNumber,
      aiResult.targetRange.endColumn
    );
    editor.executeEdits("ai-assist-apply", [
      {
        range: targetRange,
        text: aiResult.suggestedSQL,
        forceMoveMarkers: true,
      },
    ]);
    const newSQL = editor.getModel()?.getValue() || aiResult.suggestedSQL;
    setSQL(newSQL);
    onSQLChange?.(newSQL);
    setAIPreviewOpen(false);
    editor.focus();
  }, [aiResult, onSQLChange]);

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

  const aiPreviewData = useMemo(() => {
    if (!aiResult) return null;
    const formattedOriginal = formatSQLSafe(aiResult.originalText, dialect);
    const formattedSuggested = formatSQLSafe(aiResult.suggestedSQL, dialect);
    return {
      ...aiResult,
      formattedOriginal,
      formattedSuggested,
      changed: formattedOriginal.trim() !== formattedSuggested.trim(),
    };
  }, [aiResult, dialect]);

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
          className="flex items-center justify-center gap-1 h-[22px] px-2 rounded-[var(--radius-btn)] bg-[var(--accent)] text-[var(--accent-fg)] hover:opacity-90 transition-opacity disabled:opacity-50 font-medium text-[length:var(--size-font-xs)] whitespace-nowrap flex-shrink-0"
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

        {/* 收藏按钮（当前 SQL） */}
        <button
          className="flex items-center justify-center h-[22px] w-[22px] rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] text-[var(--fg-secondary)] hover:text-[var(--fg)] transition-colors"
          onClick={handleToggleFavoriteCurrent}
          title={`${isCurrentSQLFavorited ? t("editor.unfavorite") : t("editor.favorite")} (⌘D)`}
        >
          <Star className={cn("h-3.5 w-3.5", isCurrentSQLFavorited && "fill-yellow-500 text-yellow-500")} />
        </button>

        {/* 收藏夹按钮（面板） */}
        <button
          className="flex items-center justify-center h-[22px] w-[22px] rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] text-[var(--fg-secondary)] hover:text-[var(--fg)] transition-colors"
          onClick={() => setFavoritesOpen(true)}
          title={t("editor.favorites")}
        >
          <FolderOpen className="h-3.5 w-3.5" />
        </button>

        {onAIAssist && (
          <div className="w-px h-3.5 bg-[var(--border-color)]" />
        )}
        <button
          className="flex items-center justify-center gap-1 h-[22px] px-1.5 rounded-[var(--radius-btn)] hover:bg-[var(--sidebar-hover)] text-[var(--fg-secondary)] hover:text-[var(--accent)] transition-colors"
          onClick={runAIAssist}
          disabled={aiBusy}
          title={`${t("editor.aiAssist")} (⌘I)`}
        >
          {aiBusy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Sparkles className="h-3.5 w-3.5" />}
        </button>

        <div className="min-w-0 max-w-[420px] px-1">
          {aiError ? (
            <div className="truncate text-[10px] text-[var(--danger)]" title={aiError}>
              {aiError}
            </div>
          ) : aiResult ? (
            <div className="flex items-center gap-1 min-w-0">
              <span className="truncate text-[10px] text-[var(--fg-secondary)]" title={aiResult.explanation}>
                {aiResult.mode === "check"
                  ? `${aiResult.hasError ? t("editor.aiCheckFixed") : t("editor.aiCheckPassed")} · ${aiResult.source === "selection" ? t("editor.aiSelection") : t("editor.aiFullText")}`
                  : `${t("editor.aiGenerateDone")} · ${aiResult.source === "selection" ? t("editor.aiFromSelection") : t("editor.aiFromFullText")}`}
              </span>
              <button
                className="h-[20px] px-1.5 rounded-[var(--radius-btn)] text-[10px] text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)] transition-colors flex-shrink-0"
                onClick={() => setAIPreviewOpen(true)}
              >
                {t("editor.aiPreview")}
              </button>
              <button
                className="h-[20px] px-1.5 rounded-[var(--radius-btn)] text-[10px] text-[var(--accent)] hover:bg-[var(--sidebar-hover)] transition-colors flex-shrink-0"
                onClick={applyAIResult}
              >
                {t("common.apply")}
              </button>
              <button
                className="h-[20px] px-1.5 rounded-[var(--radius-btn)] text-[10px] text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)] transition-colors flex-shrink-0"
                onClick={() => copyToClipboard(aiResult.suggestedSQL)}
              >
                {t("common.copy")}
              </button>
            </div>
          ) : (
            <div className="truncate text-[10px] text-[var(--fg-muted)]">
              {t("editor.aiHint")}
            </div>
          )}
        </div>
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

      {/* AI 预览弹窗（检查场景显示差异，生成场景显示候选 SQL） */}
      {aiPreviewOpen && aiPreviewData && (
        <>
          <div
            className="fixed inset-0 z-[65] bg-black/20 backdrop-blur-sm"
            onClick={() => setAIPreviewOpen(false)}
          />
          <div className="fixed z-[66] left-1/2 top-[8%] -translate-x-1/2 w-[min(92vw,1100px)] h-[min(84vh,760px)] rounded-[var(--radius-panel)] border border-[var(--border-color)] bg-[var(--surface)] shadow-xl overflow-hidden flex flex-col">
            <div className="h-[var(--size-toolbar)] px-[var(--size-padding)] border-b border-[var(--border-color)] flex items-center gap-[var(--size-gap-sm)] flex-shrink-0">
              {aiPreviewData.mode === "check" ? (
                <Wand2 className="h-4 w-4 text-[var(--accent)]" />
              ) : (
                <Eye className="h-4 w-4 text-[var(--accent)]" />
              )}
              <div className="text-[length:var(--size-font-sm)] text-[var(--fg)] font-medium">
                {aiPreviewData.mode === "check" ? t("editor.aiPreviewFixTitle") : t("editor.aiPreviewGenTitle")}
              </div>
              <div className="text-[length:var(--size-font-xs)] text-[var(--fg-muted)] truncate min-w-0">
                {aiPreviewData.mode === "check"
                  ? t("editor.aiPreviewFixDesc", {
                    status: aiPreviewData.hasError ? t("editor.aiCheckFixed") : t("editor.aiCheckPassed"),
                    dialect: dialectToLabel(dialect, locale),
                  })
                  : t("editor.aiPreviewGenDesc", { dialect: dialectToLabel(dialect, locale) })}
              </div>
              {aiPreviewData.mode === "check" && (
                <span className="text-[10px] text-[var(--fg-muted)]">
                  {aiPreviewData.changed ? t("editor.aiChanged") : t("editor.aiUnchanged")}
                </span>
              )}
              <div className="flex-1" />
              <button
                className="h-[var(--size-btn-sm)] px-2 rounded-[var(--radius-btn)] text-[length:var(--size-font-xs)] text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)] inline-flex items-center gap-1"
                onClick={() => copyToClipboard(aiPreviewData.formattedSuggested)}
              >
                <Copy className="h-3.5 w-3.5" />
                {t("editor.aiCopyResult")}
              </button>
              <button
                className="h-[var(--size-btn-sm)] px-2 rounded-[var(--radius-btn)] text-[length:var(--size-font-xs)] text-[var(--accent)] hover:bg-[var(--sidebar-hover)] inline-flex items-center gap-1"
                onClick={applyAIResult}
              >
                <Check className="h-3.5 w-3.5" />
                {t("editor.aiApplyToEditor")}
              </button>
              <button
                className="h-[var(--size-btn-sm)] px-2 rounded-[var(--radius-btn)] text-[length:var(--size-font-xs)] text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)] inline-flex items-center gap-1"
                onClick={() => setAIPreviewOpen(false)}
              >
                <X className="h-3.5 w-3.5" />
                {t("common.close")}
              </button>
            </div>

            <div className="px-[var(--size-padding)] py-[var(--size-padding-sm)] border-b border-[var(--border-color)] bg-[var(--surface-secondary)] flex-shrink-0">
              <div className="text-[length:var(--size-font-xs)] text-[var(--fg-muted)] mb-1">{t("editor.aiExplanation")}</div>
              <div className="text-[length:var(--size-font-sm)] text-[var(--fg)] leading-5 max-h-[88px] overflow-y-auto whitespace-pre-wrap break-words pr-1">
                {aiPreviewData.explanation}
              </div>
            </div>

            <div className="flex-1 min-h-0">
              {aiPreviewData.mode === "check" ? (
                <DiffEditor
                  original={aiPreviewData.formattedOriginal}
                  modified={aiPreviewData.formattedSuggested}
                  language="sql"
                  theme={theme === "dark" ? "vs-dark" : "vs"}
                  options={{
                    readOnly: true,
                    renderSideBySide: true,
                    automaticLayout: true,
                    minimap: { enabled: false },
                    wordWrap: "on",
                    fontSize: editorFontSize,
                    originalEditable: false,
                    scrollbar: { vertical: "auto", horizontal: "auto", alwaysConsumeMouseWheel: false },
                  }}
                />
              ) : (
                <div className="h-full grid grid-cols-2">
                  <div className="min-w-0 border-r border-[var(--border-color)]">
                    <div className="h-[28px] px-2 flex items-center text-[length:var(--size-font-xs)] text-[var(--fg-muted)] border-b border-[var(--border-color)]">
                      {t("editor.aiInputText")}
                    </div>
                    <Editor
                      defaultLanguage="markdown"
                      value={aiPreviewData.originalText}
                      theme={theme === "dark" ? "vs-dark" : "vs"}
                      options={{
                        readOnly: true,
                        automaticLayout: true,
                        minimap: { enabled: false },
                        lineNumbers: "off",
                        wordWrap: "on",
                        fontSize: editorFontSize,
                        scrollBeyondLastLine: false,
                        scrollbar: { vertical: "auto", horizontal: "auto", alwaysConsumeMouseWheel: false },
                      }}
                    />
                  </div>
                  <div className="min-w-0">
                    <div className="h-[28px] px-2 flex items-center text-[length:var(--size-font-xs)] text-[var(--fg-muted)] border-b border-[var(--border-color)]">
                      {t("editor.aiGeneratedSQL")}
                    </div>
                    <Editor
                      defaultLanguage="sql"
                      value={aiPreviewData.formattedSuggested}
                      theme={theme === "dark" ? "vs-dark" : "vs"}
                      options={{
                        readOnly: true,
                        automaticLayout: true,
                        minimap: { enabled: false },
                        lineNumbers: "on",
                        wordWrap: "on",
                        fontSize: editorFontSize,
                        scrollBeyondLastLine: false,
                        scrollbar: { vertical: "auto", horizontal: "auto", alwaysConsumeMouseWheel: false },
                      }}
                    />
                  </div>
                </div>
              )}
            </div>
          </div>
        </>
      )}

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
