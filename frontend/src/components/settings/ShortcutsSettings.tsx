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
  const rows = shortcutGroups.flatMap((group) =>
    group.items.map((item) => ({
      group: group.title,
      keys: item.keys,
      description: item.description,
    }))
  );

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

      <div className="rounded-[var(--radius-panel)] border border-[var(--border-color)] bg-[var(--surface-secondary)]/35 overflow-hidden">
        <table className="w-full table-fixed border-collapse">
          <thead className="bg-[var(--surface-secondary)]/70">
            <tr className="text-[length:var(--size-font-2xs)] text-[var(--fg-secondary)]">
              <th className="w-[24%] px-2 py-1.5 text-left font-medium border-b border-[var(--border-color)]">分类</th>
              <th className="w-[26%] px-2 py-1.5 text-left font-medium border-b border-[var(--border-color)]">快捷键</th>
              <th className="px-2 py-1.5 text-left font-medium border-b border-[var(--border-color)]">说明</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr
                key={`${row.group}-${row.keys}-${row.description}`}
                className="align-top even:bg-[var(--surface)]/35"
              >
                <td className="px-2 py-1.5 text-[length:var(--size-font-2xs)] text-[var(--fg)]">
                  <span className="inline-block leading-5">{row.group}</span>
                </td>
                <td className="px-2 py-1.5">
                  <div className="flex flex-wrap gap-1">
                    {row.keys.split(" / ").map((part) => (
                      <kbd
                        key={part}
                        className="inline-flex items-center h-5 px-1.5 rounded-[6px] border border-[var(--border-color)] bg-[var(--surface)] text-[11px] leading-none text-[var(--fg-secondary)]"
                      >
                        {part}
                      </kbd>
                    ))}
                  </div>
                </td>
                <td className="px-2 py-1.5 text-[length:var(--size-font-2xs)] text-[var(--fg-secondary)] leading-5">
                  {row.description}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
