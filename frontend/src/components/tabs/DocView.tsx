import React, { useEffect, useState } from "react";
import { MarkdownEditor } from "@/components/editor/MarkdownEditor";
import { type Tab } from "@/stores/tabs";
import { useConnectionStore } from "@/stores/connection";
import * as DocService from "../../../wailsjs/go/services/DocService";

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
  );
}
