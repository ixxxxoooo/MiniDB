import { useTabsStore, type Tab } from "@/stores/tabs";
import { requestConfirm } from "@/stores/confirm";
import { t } from "@/i18n";

/** 关闭这些标签页之前，若其中有未保存修改则让用户确认；返回是否可以继续。 */
export async function confirmDiscardForTabs(tabs: Tab[]): Promise<boolean> {
  const dirty = tabs.filter((tab) => tab.dirty);
  if (dirty.length === 0) return true;
  return requestConfirm({
    title: t("dialog.unsavedTitle"),
    message: t("dialog.unsavedMessage", { names: dirty.map((tab) => tab.title).join("、") }),
    confirmText: t("dialog.discard"),
    danger: true,
  });
}

/** 是否存在任何未保存的标签页（用于退出应用、安装更新前的确认）。 */
export function hasDirtyTabs(): boolean {
  return useTabsStore.getState().tabs.some((tab) => tab.dirty);
}

export async function confirmDiscardAllDirty(): Promise<boolean> {
  return confirmDiscardForTabs(useTabsStore.getState().tabs);
}

export async function removeTabGuarded(id: string): Promise<void> {
  const { tabs, removeTab } = useTabsStore.getState();
  const tab = tabs.find((item) => item.id === id);
  if (!tab) return;
  if (await confirmDiscardForTabs([tab])) removeTab(id);
}

export async function removeTabsGuarded(ids: string[]): Promise<void> {
  const { tabs, removeTab } = useTabsStore.getState();
  const targets = tabs.filter((tab) => ids.includes(tab.id));
  if (await confirmDiscardForTabs(targets)) targets.forEach((tab) => removeTab(tab.id));
}

export async function closeOtherTabsGuarded(id: string): Promise<void> {
  const { tabs, closeOtherTabs } = useTabsStore.getState();
  const keep = tabs.find((tab) => tab.id === id);
  if (!keep) return;
  const ws = `${keep.connectionId}:${keep.database}`;
  const doomed = tabs.filter((tab) => tab.id !== id && tab.closable && `${tab.connectionId}:${tab.database}` === ws);
  if (await confirmDiscardForTabs(doomed)) closeOtherTabs(id);
}

export async function closeAllTabsGuarded(workspaceId?: string): Promise<void> {
  const { tabs, closeAllTabs } = useTabsStore.getState();
  const doomed = tabs.filter((tab) => tab.closable && (!workspaceId || `${tab.connectionId}:${tab.database}` === workspaceId));
  if (await confirmDiscardForTabs(doomed)) closeAllTabs(workspaceId);
}

/** 断开连接前：该连接下有未保存修改则确认（断开会关闭其标签页）。 */
export async function confirmDiscardForConnection(connectionId: string): Promise<boolean> {
  return confirmDiscardForTabs(useTabsStore.getState().tabs.filter((tab) => tab.connectionId === connectionId));
}
