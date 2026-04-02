import React, { useRef } from "react";
import { Copy, Check } from "lucide-react";
import { Button } from "@/components/ui/button";
import { copyToClipboard } from "@/lib/utils";
import Editor from "@monaco-editor/react";
import { useThemeStore } from "@/stores/theme";
import { useUIStore } from "@/stores/ui";
import { format } from "sql-formatter";
import { useTranslation } from "@/i18n";

interface DDLViewerProps {
  ddl: string;
  tableName: string;
}

export function DDLViewer({ ddl, tableName }: DDLViewerProps) {
  const [copied, setCopied] = React.useState(false);
  const { resolved } = useThemeStore();
  const { layoutMode } = useUIStore();
  const { t } = useTranslation();
  const editorFontSize = layoutMode === "compact" ? 12 : 13;

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
      <div className="flex items-center justify-between px-[var(--size-padding)] py-[var(--size-gap-sm)] border-b border-[var(--border-color)]">
        <span className="text-[length:var(--size-font-sm)] font-medium">{tableName}</span>
        <Button variant="ghost" size="sm" onClick={handleCopy}>
          {copied ? (<><Check className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)] mr-1" />{t("common.success")}</>) : (<><Copy className="h-[var(--size-btn-icon-sm)] w-[var(--size-btn-icon-sm)] mr-1" />{t("common.copy")}</>)}
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
            fontSize: editorFontSize,
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
