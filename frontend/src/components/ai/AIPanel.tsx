import React, { useState } from "react";
import {
  Sparkles,
  X,
  MessageSquare,
  Code,
  BarChart3,
  FileText,
  AlertTriangle,
  Send,
  Loader2,
  Copy,
  Check,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { copyToClipboard } from "@/lib/utils";

type AIMode = "nl2sql" | "explain" | "insight" | "docgen" | "diagnose";

interface AIPanelProps {
  open: boolean;
  onClose: () => void;
  currentDatabase?: string;
  currentTable?: string;
  selectedSQL?: string;
  errorMessage?: string;
  onExecuteSQL?: (sql: string) => void;
}

const MODES: { id: AIMode; label: string; icon: React.ElementType; description: string }[] = [
  { id: "nl2sql", label: "自然语言查询", icon: MessageSquare, description: "用自然语言描述，AI 生成 SQL" },
  { id: "explain", label: "SQL 解释", icon: Code, description: "解释 SQL 执行逻辑和优化建议" },
  { id: "insight", label: "数据洞察", icon: BarChart3, description: "对查询结果进行智能分析" },
  { id: "docgen", label: "文档生成", icon: FileText, description: "自动生成表结构文档" },
  { id: "diagnose", label: "错误诊断", icon: AlertTriangle, description: "分析 SQL 错误并给出修复建议" },
];

export function AIPanel({
  open,
  onClose,
  currentDatabase,
  currentTable,
  selectedSQL,
  errorMessage,
  onExecuteSQL,
}: AIPanelProps) {
  const [mode, setMode] = useState<AIMode>("nl2sql");
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<string>("");
  const [generatedSQL, setGeneratedSQL] = useState<string>("");
  const [copied, setCopied] = useState(false);

  if (!open) return null;

  const handleSubmit = async () => {
    if (!input.trim() && mode === "nl2sql") return;
    setLoading(true);
    setResult("");
    setGeneratedSQL("");

    // 模拟 AI 调用（后续将替换为真实的 Wails 后端调用）
    setTimeout(() => {
      if (mode === "nl2sql") {
        const sql = `SELECT * FROM users WHERE created_at >= DATE_SUB(NOW(), INTERVAL 7 DAY) ORDER BY created_at DESC;`;
        setGeneratedSQL(sql);
        setResult(
          `根据您的描述"${input}"，我生成了以下 SQL 查询。\n该查询将从 users 表中筛选最近 7 天创建的记录，并按创建时间倒序排列。`
        );
      } else if (mode === "explain") {
        setResult(
          `## SQL 分析\n\n**查询类型**: SELECT 查询\n\n**执行步骤**:\n1. 全表扫描 users 表\n2. 应用 WHERE 条件过滤\n3. 按 created_at 排序\n\n**优化建议**:\n- 建议在 created_at 列上创建索引\n- 考虑添加 LIMIT 限制返回行数`
        );
      } else if (mode === "insight") {
        setResult(
          `## 数据洞察\n\n**数据摘要**: 共 1,234 条记录\n\n**异常检测**:\n- user_type 列中有 5% 的空值\n\n**趋势分析**:\n- 注册量在最近 7 天呈上升趋势 (+12%)`
        );
      } else if (mode === "docgen") {
        setResult(
          `# ${currentTable || "users"} 表文档\n\n## 概述\n用户信息主表，存储系统所有注册用户的基本信息。\n\n## 字段说明\n| 字段 | 类型 | 说明 |\n|------|------|------|\n| id | BIGINT | 主键 |\n| username | VARCHAR(50) | 用户名 |\n| email | VARCHAR(100) | 邮箱 |\n| created_at | DATETIME | 创建时间 |`
        );
      } else if (mode === "diagnose") {
        setResult(
          `## 错误诊断\n\n**错误类型**: 语法错误\n\n**原因分析**: SQL 语句中存在拼写错误\n\n**修复建议**: 将 "SELCET" 修改为 "SELECT"`
        );
        setGeneratedSQL(`SELECT * FROM users WHERE id = 1;`);
      }
      setLoading(false);
    }, 1500);
  };

  const handleCopySQL = async () => {
    await copyToClipboard(generatedSQL);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div
      className={cn(
        "flex flex-col border-l h-full animate-slide-in-right",
        "bg-[var(--surface)] border-[var(--border-color)]"
      )}
      style={{ width: 380 }}
    >
      {/* 头部 */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-[var(--border-color)]">
        <div className="flex items-center gap-1.5">
          <Sparkles className="h-4 w-4 text-[var(--accent)]" />
          <span className="text-sm font-medium">AI 助手</span>
        </div>
        <Button variant="ghost" size="icon" className="h-6 w-6" onClick={onClose}>
          <X className="h-3 w-3" />
        </Button>
      </div>

      {/* 模式选择 */}
      <div className="flex gap-1 px-3 py-2 overflow-x-auto border-b border-[var(--border-subtle)]">
        {MODES.map((m) => {
          const Icon = m.icon;
          return (
            <button
              key={m.id}
              className={cn(
                "flex items-center gap-1 px-2 py-1 rounded-md text-xs whitespace-nowrap transition-colors",
                mode === m.id
                  ? "bg-[var(--accent)]/10 text-[var(--accent)] font-medium"
                  : "text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)]"
              )}
              onClick={() => {
                setMode(m.id);
                setResult("");
                setGeneratedSQL("");
              }}
              title={m.description}
            >
              <Icon className="h-3 w-3" />
              {m.label}
            </button>
          );
        })}
      </div>

      {/* 上下文信息 */}
      {(currentDatabase || currentTable) && (
        <div className="px-3 py-1.5 text-2xs text-[var(--fg-muted)] border-b border-[var(--border-subtle)]">
          上下文: {currentDatabase}
          {currentTable && ` / ${currentTable}`}
        </div>
      )}

      {/* 结果区域 */}
      <div className="flex-1 overflow-y-auto p-3">
        {result && (
          <div className="text-sm text-[var(--fg)] whitespace-pre-wrap leading-relaxed">
            {result}
          </div>
        )}

        {generatedSQL && (
          <div className="mt-3 rounded-lg border border-[var(--border-color)] overflow-hidden">
            <div className="flex items-center justify-between px-3 py-1.5 bg-[var(--surface-secondary)]">
              <span className="text-xs font-medium text-[var(--fg-secondary)]">
                生成的 SQL
              </span>
              <div className="flex gap-1">
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-5 w-5"
                  onClick={handleCopySQL}
                >
                  {copied ? (
                    <Check className="h-3 w-3 text-[var(--success)]" />
                  ) : (
                    <Copy className="h-3 w-3" />
                  )}
                </Button>
              </div>
            </div>
            <pre className="p-3 text-xs font-mono text-[var(--fg)] bg-[var(--surface)] overflow-x-auto">
              {generatedSQL}
            </pre>
            {onExecuteSQL && (
              <div className="px-3 py-2 border-t border-[var(--border-subtle)] bg-[var(--surface-secondary)]">
                <Button
                  size="sm"
                  className="w-full text-xs"
                  onClick={() => onExecuteSQL(generatedSQL)}
                >
                  执行此 SQL
                </Button>
              </div>
            )}
          </div>
        )}

        {loading && (
          <div className="flex items-center gap-2 py-4 text-sm text-[var(--fg-secondary)]">
            <Loader2 className="h-4 w-4 animate-spin" />
            AI 正在思考中...
          </div>
        )}
      </div>

      {/* 输入区域 */}
      <div className="px-3 py-2 border-t border-[var(--border-color)]">
        <div className="flex gap-2">
          <Input
            className="flex-1 text-xs"
            placeholder={
              mode === "nl2sql"
                ? "描述你想查询的内容，如\"查询最近7天注册的用户\""
                : mode === "explain"
                ? "粘贴要解释的 SQL..."
                : mode === "diagnose"
                ? "粘贴错误信息..."
                : "输入你的问题..."
            }
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault();
                handleSubmit();
              }
            }}
          />
          <Button
            size="icon"
            className="h-9 w-9 flex-shrink-0"
            onClick={handleSubmit}
            disabled={loading}
          >
            <Send className="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>
    </div>
  );
}
