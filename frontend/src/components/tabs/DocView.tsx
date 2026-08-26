import React, { useEffect, useState, lazy, Suspense } from "react";
import { type Tab } from "@/stores/tabs";
import { useConnectionStore } from "@/stores/connection";
import * as DocService from "@/lib/wails/services/DocService";
import { LazyLoadingPlaceholder } from "@/components/ui/LazyLoadingPlaceholder";

// MarkdownEditor 依赖 tiptap（体积大），按需懒加载
const MarkdownEditor = lazy(() =>
  import("@/components/editor/MarkdownEditor").then((m) => ({ default: m.MarkdownEditor }))
);

export function DocView({ tab, isActive = true }: { tab: Tab; isActive?: boolean }) {
  const [content, setContent] = useState("");
  const isConnectionReady = useConnectionStore(
    (s) => s.connectionStates[tab.connectionId || ""]?.status === "connected"
  );

  useEffect(() => {
    if (!isActive || !isConnectionReady) return;
    if (tab.connectionId && tab.database && tab.table) {
      DocService.GetTableDoc(tab.connectionId, tab.database, tab.table)
        .then((doc) => setContent(doc || ""))
        .catch(() => {});
    }
  }, [isActive, isConnectionReady, tab.connectionId, tab.database, tab.table]);

  return (
    <Suspense fallback={<LazyLoadingPlaceholder />}>
      <MarkdownEditor
        content={content}
        tableName={tab.table || ""}
        onSave={async (md) => {
          if (tab.connectionId && tab.database && tab.table) {
            await DocService.SaveTableDoc(tab.connectionId, tab.database, tab.table, md);
            setContent(md);
          }
        }}
      />
    </Suspense>
  );
}