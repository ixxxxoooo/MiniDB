import React, { useEffect, useState } from "react";
import { MarkdownEditor } from "@/components/editor/MarkdownEditor";
import { type Tab } from "@/stores/tabs";
import * as DocService from "../../../wailsjs/go/services/DocService";

export function DocView({ tab }: { tab: Tab }) {
  const [content, setContent] = useState("");
  useEffect(() => {
    if (tab.connectionId && tab.database && tab.table) {
      DocService.GetTableDoc(tab.connectionId, tab.database, tab.table)
        .then((doc) => setContent(doc || ""))
        .catch(() => {});
    }
  }, [tab.connectionId, tab.database, tab.table]);

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
