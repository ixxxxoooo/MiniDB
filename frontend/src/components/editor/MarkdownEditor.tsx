import React, { useState, useEffect, useCallback } from "react";
import { useEditor, EditorContent } from "@tiptap/react";
import StarterKit from "@tiptap/starter-kit";
import Placeholder from "@tiptap/extension-placeholder";
import Highlight from "@tiptap/extension-highlight";
import Link from "@tiptap/extension-link";
import { Markdown } from "tiptap-markdown";
import {
  Save,
  Sparkles,
  Loader2,
  Bold,
  Italic,
  Strikethrough,
  Code,
  Heading1,
  Heading2,
  Heading3,
  List,
  ListOrdered,
  Quote,
  Minus,
  Undo,
  Redo,
  Link as LinkIcon,
  Highlighter,
  CodeSquare,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { useTranslation } from "@/i18n";

interface MarkdownEditorProps {
  content: string;
  tableName: string;
  onSave: (content: string) => void;
  onAIGenerate?: () => Promise<string>;
}

/**
 * 所见即所得 Markdown 编辑器
 * 基于 tiptap + tiptap-markdown 实现
 */
export function MarkdownEditor({
  content,
  tableName,
  onSave,
  onAIGenerate,
}: MarkdownEditorProps) {
  const [generating, setGenerating] = useState(false);
  const [dirty, setDirty] = useState(false);
  const { t } = useTranslation();

  const editor = useEditor({
    extensions: [
      StarterKit.configure({
        heading: { levels: [1, 2, 3] },
      }),
      Placeholder.configure({
        placeholder: "在这里编写表文档（支持 Markdown 格式）...",
      }),
      Highlight,
      Link.configure({
        openOnClick: false,
        HTMLAttributes: { class: "text-[var(--accent)] underline cursor-pointer" },
      }),
      Markdown.configure({
        html: false,
        transformPastedText: true,
        transformCopiedText: true,
      }),
    ],
    content: content || "",
    editorProps: {
      attributes: {
        class: "max-w-none focus:outline-none min-h-[200px]",
      },
    },
    onUpdate: () => {
      setDirty(true);
    },
  });

  // 从编辑器获取 Markdown 文本
  const getMarkdown = useCallback((): string => {
    if (!editor) return "";
    const storage = editor.storage as Record<string, any>;
    return storage.markdown?.getMarkdown?.() ?? "";
  }, [editor]);

  // 外部 content 变化时同步到编辑器
  useEffect(() => {
    if (editor && content !== undefined) {
      const currentMd = getMarkdown();
      if (currentMd !== content) {
        editor.commands.setContent(content || "");
        setDirty(false);
      }
    }
  }, [content, editor, getMarkdown]);

  const handleSave = useCallback(() => {
    if (!editor) return;
    onSave(getMarkdown());
    setDirty(false);
  }, [editor, onSave, getMarkdown]);

  // 快捷键 ⌘S 保存
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "s") {
        e.preventDefault();
        handleSave();
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [handleSave]);

  const handleAIGenerate = useCallback(async () => {
    if (!onAIGenerate || !editor) return;
    setGenerating(true);
    try {
      const generated = await onAIGenerate();
      editor.commands.setContent(generated || "");
      setDirty(true);
    } finally {
      setGenerating(false);
    }
  }, [onAIGenerate, editor]);

  const addLink = useCallback(() => {
    if (!editor) return;
    const url = window.prompt("输入链接地址：");
    if (url) {
      editor.chain().focus().setLink({ href: url }).run();
    }
  }, [editor]);

  if (!editor) return null;

  return (
    <div className="flex flex-col h-full flex-1">
      {/* 工具栏 */}
      <div
        className={cn(
          "flex items-center gap-[var(--size-gap-sm)] px-[var(--size-padding-sm)] py-[var(--size-gap-sm)] border-b flex-shrink-0 flex-wrap",
          "bg-[var(--surface-secondary)] border-[var(--border-color)]"
        )}
      >
        <span className="text-[length:var(--size-font-2xs)] font-medium text-[var(--fg-secondary)] mr-1.5 flex-shrink-0">
          {tableName}
        </span>

        <div className="w-px h-3 bg-[var(--border-color)] mx-0.5" />

        {/* 格式化按钮组 */}
        <ToolbarBtn
          icon={Bold}
          active={editor.isActive("bold")}
          onClick={() => editor.chain().focus().toggleBold().run()}
          title="粗体 (⌘B)"
        />
        <ToolbarBtn
          icon={Italic}
          active={editor.isActive("italic")}
          onClick={() => editor.chain().focus().toggleItalic().run()}
          title="斜体 (⌘I)"
        />
        <ToolbarBtn
          icon={Strikethrough}
          active={editor.isActive("strike")}
          onClick={() => editor.chain().focus().toggleStrike().run()}
          title="删除线"
        />
        <ToolbarBtn
          icon={Code}
          active={editor.isActive("code")}
          onClick={() => editor.chain().focus().toggleCode().run()}
          title="行内代码"
        />
        <ToolbarBtn
          icon={Highlighter}
          active={editor.isActive("highlight")}
          onClick={() => editor.chain().focus().toggleHighlight().run()}
          title="高亮"
        />

        <div className="w-px h-3 bg-[var(--border-color)] mx-0.5" />

        <ToolbarBtn
          icon={Heading1}
          active={editor.isActive("heading", { level: 1 })}
          onClick={() => editor.chain().focus().toggleHeading({ level: 1 }).run()}
          title="标题 1"
        />
        <ToolbarBtn
          icon={Heading2}
          active={editor.isActive("heading", { level: 2 })}
          onClick={() => editor.chain().focus().toggleHeading({ level: 2 }).run()}
          title="标题 2"
        />
        <ToolbarBtn
          icon={Heading3}
          active={editor.isActive("heading", { level: 3 })}
          onClick={() => editor.chain().focus().toggleHeading({ level: 3 }).run()}
          title="标题 3"
        />

        <div className="w-px h-3 bg-[var(--border-color)] mx-0.5" />

        <ToolbarBtn
          icon={List}
          active={editor.isActive("bulletList")}
          onClick={() => editor.chain().focus().toggleBulletList().run()}
          title="无序列表"
        />
        <ToolbarBtn
          icon={ListOrdered}
          active={editor.isActive("orderedList")}
          onClick={() => editor.chain().focus().toggleOrderedList().run()}
          title="有序列表"
        />
        <ToolbarBtn
          icon={Quote}
          active={editor.isActive("blockquote")}
          onClick={() => editor.chain().focus().toggleBlockquote().run()}
          title="引用"
        />
        <ToolbarBtn
          icon={CodeSquare}
          active={editor.isActive("codeBlock")}
          onClick={() => editor.chain().focus().toggleCodeBlock().run()}
          title="代码块"
        />
        <ToolbarBtn
          icon={Minus}
          onClick={() => editor.chain().focus().setHorizontalRule().run()}
          title="分隔线"
        />
        <ToolbarBtn
          icon={LinkIcon}
          active={editor.isActive("link")}
          onClick={addLink}
          title="插入链接"
        />

        <div className="w-px h-3 bg-[var(--border-color)] mx-0.5" />

        <ToolbarBtn
          icon={Undo}
          onClick={() => editor.chain().focus().undo().run()}
          disabled={!editor.can().undo()}
          title="撤销 (⌘Z)"
        />
        <ToolbarBtn
          icon={Redo}
          onClick={() => editor.chain().focus().redo().run()}
          disabled={!editor.can().redo()}
          title="重做 (⌘⇧Z)"
        />

        <div className="flex-1" />

        {onAIGenerate && (
          <Button
            variant="ghost"
            size="sm"
            className="h-[var(--size-btn-sm)] text-[length:var(--size-font-2xs)] px-2"
            onClick={handleAIGenerate}
            disabled={generating}
          >
            {generating ? (
              <Loader2 className="h-2.5 w-2.5 mr-1 animate-spin" />
            ) : (
              <Sparkles className="h-2.5 w-2.5 mr-1" />
            )}
            AI
          </Button>
        )}

        {dirty && (
          <Button
            size="sm"
            className="h-[var(--size-btn-sm)] text-[length:var(--size-font-2xs)] px-2"
            onClick={handleSave}
          >
            <Save className="h-2.5 w-2.5 mr-1" />
            {t("common.save")}
          </Button>
        )}
      </div>

      {/* 编辑器内容区 */}
      <div className="flex-1 overflow-y-auto bg-[var(--surface)]">
        <EditorContent editor={editor} className="h-full wysiwyg-editor" />
      </div>
    </div>
  );
}

/** 工具栏小按钮 */
function ToolbarBtn({
  icon: Icon,
  active,
  onClick,
  disabled,
  title,
}: {
  icon: React.ElementType;
  active?: boolean;
  onClick: () => void;
  disabled?: boolean;
  title?: string;
}) {
  return (
    <button
      className={cn(
        "h-[var(--size-btn-sm)] w-[var(--size-btn-sm)] flex items-center justify-center rounded-[var(--radius-btn)] transition-colors",
        active
          ? "bg-[var(--accent)]/15 text-[var(--accent)]"
          : "text-[var(--fg-secondary)] hover:bg-[var(--sidebar-hover)] hover:text-[var(--fg)]",
        disabled && "opacity-30 cursor-not-allowed"
      )}
      onClick={onClick}
      disabled={disabled}
      title={title}
    >
      <Icon className="h-3 w-3" />
    </button>
  );
}
