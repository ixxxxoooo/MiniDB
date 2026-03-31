import React, { useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { Edit3, Eye, Save, Sparkles, Loader2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

interface MarkdownEditorProps {
  content: string;
  tableName: string;
  onSave: (content: string) => void;
  onAIGenerate?: () => Promise<string>;
}

export function MarkdownEditor({
  content,
  tableName,
  onSave,
  onAIGenerate,
}: MarkdownEditorProps) {
  const [text, setText] = useState(content);
  const [isEditing, setIsEditing] = useState(!content);
  const [generating, setGenerating] = useState(false);
  const dirty = text !== content;

  const handleAIGenerate = async () => {
    if (!onAIGenerate) return;
    setGenerating(true);
    try {
      const generated = await onAIGenerate();
      setText(generated);
      setIsEditing(true);
    } finally {
      setGenerating(false);
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
        <span className="text-xs font-medium text-[var(--fg-secondary)]">
          {tableName} 文档
        </span>

        <div className="flex-1" />

        {onAIGenerate && (
          <Button
            variant="ghost"
            size="sm"
            className="h-7 text-xs"
            onClick={handleAIGenerate}
            disabled={generating}
          >
            {generating ? (
              <Loader2 className="h-3 w-3 mr-1 animate-spin" />
            ) : (
              <Sparkles className="h-3 w-3 mr-1" />
            )}
            AI 生成
          </Button>
        )}

        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7"
          onClick={() => setIsEditing(!isEditing)}
        >
          {isEditing ? <Eye className="h-3.5 w-3.5" /> : <Edit3 className="h-3.5 w-3.5" />}
        </Button>

        {dirty && (
          <Button
            size="sm"
            className="h-7 text-xs"
            onClick={() => onSave(text)}
          >
            <Save className="h-3 w-3 mr-1" />
            保存
          </Button>
        )}
      </div>

      {/* 内容区 */}
      <div className="flex-1 overflow-y-auto">
        {isEditing ? (
          <textarea
            className={cn(
              "w-full h-full resize-none p-4 font-mono text-sm leading-relaxed",
              "bg-[var(--surface)] text-[var(--fg)] placeholder:text-[var(--fg-muted)]",
              "focus:outline-none"
            )}
            placeholder="在这里编写表文档（支持 Markdown 格式）..."
            value={text}
            onChange={(e) => setText(e.target.value)}
            spellCheck={false}
          />
        ) : (
          <div className="p-4 prose prose-sm dark:prose-invert max-w-none">
            {text ? (
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{text}</ReactMarkdown>
            ) : (
              <div className="text-center py-8 text-[var(--fg-muted)]">
                <p>暂无文档</p>
                <p className="text-xs mt-1">点击编辑或使用 AI 自动生成</p>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
