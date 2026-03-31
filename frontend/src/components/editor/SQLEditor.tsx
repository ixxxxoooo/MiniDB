import React, { useState, useRef } from "react";
import { Play, Sparkles, Copy, Check, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { copyToClipboard } from "@/lib/utils";

interface SQLEditorProps {
  initialSQL?: string;
  onExecute: (sql: string) => void;
  onAIAssist?: (sql: string) => void;
  loading?: boolean;
}

export function SQLEditor({
  initialSQL = "",
  onExecute,
  onAIAssist,
  loading,
}: SQLEditorProps) {
  const [sql, setSQL] = useState(initialSQL);
  const [copied, setCopied] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const handleExecute = () => {
    const selected = window.getSelection()?.toString();
    onExecute(selected || sql);
  };

  const handleCopy = async () => {
    await copyToClipboard(sql);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    // Cmd+Enter 执行
    if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
      e.preventDefault();
      handleExecute();
    }
    // Tab 缩进
    if (e.key === "Tab") {
      e.preventDefault();
      const textarea = textareaRef.current;
      if (textarea) {
        const start = textarea.selectionStart;
        const end = textarea.selectionEnd;
        const value = textarea.value;
        setSQL(value.substring(0, start) + "  " + value.substring(end));
        setTimeout(() => {
          textarea.selectionStart = textarea.selectionEnd = start + 2;
        }, 0);
      }
    }
  };

  return (
    <div className="flex flex-col h-full">
      {/* 工具栏 */}
      <div
        className={cn(
          "flex items-center gap-1 px-3 py-1.5 border-b",
          "bg-[var(--surface-secondary)] border-[var(--border-color)]"
        )}
      >
        <Button
          size="sm"
          className="h-7 text-xs"
          onClick={handleExecute}
          disabled={loading || !sql.trim()}
        >
          {loading ? (
            <Loader2 className="h-3 w-3 mr-1 animate-spin" />
          ) : (
            <Play className="h-3 w-3 mr-1" />
          )}
          执行 (⌘↵)
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

        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7"
          onClick={handleCopy}
        >
          {copied ? (
            <Check className="h-3 w-3 text-[var(--success)]" />
          ) : (
            <Copy className="h-3 w-3" />
          )}
        </Button>
      </div>

      {/* 编辑区 */}
      <div className="flex-1 relative">
        <textarea
          ref={textareaRef}
          className={cn(
            "w-full h-full resize-none p-4 font-mono text-sm leading-relaxed",
            "bg-[var(--surface)] text-[var(--fg)] placeholder:text-[var(--fg-muted)]",
            "focus:outline-none"
          )}
          placeholder="输入 SQL 语句..."
          value={sql}
          onChange={(e) => setSQL(e.target.value)}
          onKeyDown={handleKeyDown}
          spellCheck={false}
        />
      </div>
    </div>
  );
}
