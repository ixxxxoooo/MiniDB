import React from "react";
import { useTranslation } from "@/i18n";
import { Keyboard } from "lucide-react";

export function ShortcutsSettings() {
  const { t } = useTranslation();

  const shortcutGroups = [
    {
      title: t("generalSettings.shortcutsGlobal"),
      items: [
        { keys: "⌘P", description: t("toolbar.quickSearch") },
        { keys: "⌘N", description: t("toolbar.newConnection") },
        { keys: "⌘T", description: t("tabs.newQuery") },
        { keys: "⌘K", description: t("toolbar.switchDatabase") },
        { keys: "⌘,", description: t("settings.title") },
        { keys: "⌘W", description: t("tabs.close") },
        { keys: "⌘[ / ⌘]", description: "切换标签页 / Switch tabs" },
        { keys: "⌘⇧[ / ⌘⇧]", description: "切换工作区 / Switch workspaces" },
      ],
    },
    {
      title: t("generalSettings.shortcutsTableView"),
      items: [
        { keys: "⌘F", description: "显示或隐藏筛选栏 / Toggle filters" },
        { keys: "⌘S", description: `${t("common.commit")}（数据或结构） / Commit changes` },
        { keys: "⌘R", description: `${t("common.refresh")}当前视图 / Refresh current view` },
        { keys: "⌘⌥[ / ⌘⌥]", description: "切换 Data / Structure / DDL / Doc 子视图 / Switch sub views" },
        { keys: "↑ / ↓", description: "在数据行之间移动选中项 / Move row selection" },
        { keys: "Space", description: `${t("contextMenu.previewRow")} / Preview selected row` },
        { keys: "⌫", description: `${t("contextMenu.deleteRow")}或删除选中列 / Delete selected row or column` },
      ],
    },
    {
      title: t("generalSettings.shortcutsSqlEditor"),
      items: [
        { keys: "⌘↵", description: t("editor.execute") },
        { keys: "⌘⇧↵", description: "执行全部 SQL / Execute all SQL" },
        { keys: "⌘⇧F", description: t("editor.format") },
        { keys: "⌘D", description: "收藏或取消收藏当前 SQL / Toggle SQL favorite" },
        { keys: "⌘I", description: t("editor.aiAssist") },
        { keys: "⌘S", description: t("editor.save") },
      ],
    },
    {
      title: t("generalSettings.shortcutsAiPanel"),
      items: [
        { keys: "Enter", description: "发送消息 / Send message" },
        { keys: "⇧Enter", description: "输入换行 / Insert newline" },
        { keys: "⌘Enter", description: "发送消息 / Send message" },
        { keys: "↑ / ↓", description: "选择 @ 联想项 / Navigate mention suggestions" },
        { keys: "Tab / Enter", description: "确认 @ 联想项 / Confirm mention suggestion" },
        { keys: "Escape", description: "关闭 @ 联想面板 / Close mention suggestions" },
      ],
    },
    {
      title: t("generalSettings.shortcutsDocEditor"),
      items: [
        { keys: "⌘S", description: t("editor.save") },
        { keys: "⌘B", description: t("markdown.bold") },
        { keys: "⌘I", description: t("markdown.italic") },
        { keys: "⌘Z", description: t("markdown.undo") },
        { keys: "⌘⇧Z", description: t("markdown.redo") },
        { keys: "⌘Enter", description: "确认行内编辑 / Confirm inline edit" },
      ],
    },
    {
      title: t("generalSettings.shortcutsDialog"),
      items: [
        { keys: "Escape", description: "关闭设置、连接、日志等弹窗 / Close dialogs" },
        { keys: "↑ / ↓", description: "连接列表中移动高亮项 / Navigate connection list" },
        { keys: "Enter", description: "确认选中项或提交 / Confirm selected item" },
      ],
    },
  ];

  return (
    <div className="space-y-[var(--size-gap)]">
      <div className="flex items-start gap-2">
        <div className="h-7 w-7 rounded-[var(--radius-btn)] border border-[var(--border-color)] bg-[var(--surface-secondary)] flex items-center justify-center flex-shrink-0">
          <Keyboard className="h-3.5 w-3.5 text-[var(--fg-secondary)]" />
        </div>
        <div>
          <h3 className="text-[length:var(--size-font-xs)] font-semibold mb-0.5">{t("generalSettings.shortcutsTitle")}</h3>
          <p className="text-[length:var(--size-font-2xs)] text-[var(--fg-secondary)]">
            {t("generalSettings.shortcutsDescription")}
          </p>
        </div>
      </div>

      <div className="space-y-[var(--size-gap-sm)]">
        {shortcutGroups.map((group) => (
          <div
            key={group.title}
            className="rounded-[var(--radius-panel)] border border-[var(--border-color)] bg-[var(--surface-secondary)]/50 overflow-hidden"
          >
            <div className="px-[var(--size-padding-sm)] py-[var(--size-gap-sm)] border-b border-[var(--border-color)] text-[length:var(--size-font-xs)] font-medium text-[var(--fg)]">
              {group.title}
            </div>
            <div className="divide-y divide-[var(--border-color)]/70">
              {group.items.map((item) => (
                <div
                  key={`${group.title}-${item.keys}-${item.description}`}
                  className="flex items-center gap-[var(--size-gap)] px-[var(--size-padding-sm)] py-[var(--size-gap-sm)]"
                >
                  <div className="min-w-[132px] flex-shrink-0 flex flex-wrap gap-[var(--size-gap-sm)]">
                    {item.keys.split(" / ").map((part) => (
                      <kbd
                        key={part}
                        className="inline-flex items-center h-[var(--size-btn-sm)] px-2 rounded-[var(--radius-btn)] border border-[var(--border-color)] bg-[var(--surface)] text-[length:var(--size-font-2xs)] text-[var(--fg-secondary)]"
                      >
                        {part}
                      </kbd>
                    ))}
                  </div>
                  <div className="text-[length:var(--size-font-2xs)] text-[var(--fg-secondary)] leading-5">
                    {item.description}
                  </div>
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
