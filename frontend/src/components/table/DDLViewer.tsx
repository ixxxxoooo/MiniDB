import React, { useRef } from "react";
import { Copy, Check } from "lucide-react";
import { Button } from "@/components/ui/button";
import { copyToClipboard } from "@/lib/utils";
import Editor from "@monaco-editor/react";
import { useThemeStore } from "@/stores/theme";
import { format } from "sql-formatter";

interface DDLViewerProps {
  ddl: string;
  tableName: string;
}

export function DDLViewer({ ddl, tableName }: DDLViewerProps) {
  const [copied, setCopied] = React.useState(false);
  const { resolved } = useThemeStore();

  const handleCopy = async () => {
    await copyToClipboard(formattedDDL);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  let formattedDDL = ddl;
  try {
    formattedDDL = format(ddl, { language: "mysql", tabWidth: 2 });
  } catch {
    formattedDDL = ddl;
  }

  return (
    <div className="flex flex-col h-full flex-1">
      <div className="flex items-center justify-between px-4 py-2 border-b border-[var(--border-color)]">
        <span className="text-sm font-medium">{tableName}</span>
        <Button variant="ghost" size="sm" onClick={handleCopy}>
          {copied ? (<><Check className="h-3 w-3 mr-1" />已复制</>) : (<><Copy className="h-3 w-3 mr-1" />复制</>)}
        </Button>
      </div>
      <div className="flex-1 overflow-hidden">
        <Editor
          height="100%"
          language="sql"
          theme={resolved === "dark" ? "vs-dark" : "vs"}
          value={formattedDDL || "-- 加载中..."}
          options={{
            readOnly: true,
            minimap: { enabled: false },
            lineNumbers: "on",
            fontSize: 13,
            fontFamily: "'SF Mono', 'Fira Code', Menlo, monospace",
            scrollBeyondLastLine: false,
            wordWrap: "on",
            renderLineHighlight: "none",
            overviewRulerBorder: false,
            padding: { top: 12 },
          }}
        />
      </div>
    </div>
  );
}
