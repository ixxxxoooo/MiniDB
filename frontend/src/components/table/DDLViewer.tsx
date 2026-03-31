import React from "react";
import { Copy, Check } from "lucide-react";
import { Button } from "@/components/ui/button";
import { copyToClipboard } from "@/lib/utils";

interface DDLViewerProps {
  ddl: string;
  tableName: string;
}

export function DDLViewer({ ddl, tableName }: DDLViewerProps) {
  const [copied, setCopied] = React.useState(false);

  const handleCopy = async () => {
    await copyToClipboard(ddl);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="flex flex-col h-full">
      {/* 头部 */}
      <div className="flex items-center justify-between px-4 py-2 border-b border-[var(--border-color)]">
        <span className="text-sm font-medium">DDL — {tableName}</span>
        <Button variant="ghost" size="sm" onClick={handleCopy}>
          {copied ? (
            <>
              <Check className="h-3 w-3 mr-1" />
              已复制
            </>
          ) : (
            <>
              <Copy className="h-3 w-3 mr-1" />
              复制
            </>
          )}
        </Button>
      </div>

      {/* DDL 内容 */}
      <div className="flex-1 overflow-auto p-4">
        <pre className="text-sm font-mono leading-relaxed text-[var(--fg)] whitespace-pre-wrap">
          {ddl || "加载中..."}
        </pre>
      </div>
    </div>
  );
}
