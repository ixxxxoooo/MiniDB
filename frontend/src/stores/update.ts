import { create } from "zustand";
import { EventsOn } from "@/lib/wails/runtime";
import * as SettingsService from "@/lib/wails/services/SettingsService";

export type UpdateState = "idle" | "checking" | "available" | "downloading" | "ready" | "installing" | "error";

export interface UpdatePayload {
  state: UpdateState;
  version?: string;
  releaseDate?: string;
  releaseNotes?: string;
  mandatory?: boolean;
  downloaded?: number;
  total?: number;
  percentage?: number;
  error?: string;
  messageCode?: string;
  prompt?: boolean;
  promptKind?: string;
  releaseUrl?: string;
  canInstall?: boolean;
  manual?: boolean;
}

export interface UpdateSettingsValue {
  autoCheck: boolean;
  autoDownload: boolean;
  skippedVersion: string;
}

interface UpdateStore {
  state: UpdateState;
  info: UpdatePayload;
  settings: UpdateSettingsValue;
  /** 需要弹出的卡片：available / ready / message / error */
  card: { kind: "available" | "ready" | "message" | "error"; key: number } | null;
  dialogOpen: boolean;
  /** 最近一条面向用户的消息码（手动检查的结果、错误等） */
  messageCode: string;

  applyPayload: (payload: UpdatePayload) => void;
  dismissCard: () => void;
  openDialog: () => void;
  closeDialog: () => void;
  loadSettings: () => Promise<void>;
  saveSettings: (next: Partial<UpdateSettingsValue>) => Promise<void>;
  checkNow: () => Promise<void>;
  download: () => Promise<void>;
  skipVersion: () => Promise<void>;
}

const DEFAULT_SETTINGS: UpdateSettingsValue = { autoCheck: true, autoDownload: true, skippedVersion: "" };
let cardSeq = 0;

export const useUpdateStore = create<UpdateStore>((set, get) => ({
  state: "idle",
  info: { state: "idle" },
  settings: DEFAULT_SETTINGS,
  card: null,
  dialogOpen: false,
  messageCode: "",

  applyPayload: (payload) => {
    const state = (payload.state || "idle") as UpdateState;
    // 进度事件只带进度字段，需要合并而不是替换，否则版本号等信息会丢失
    const info = { ...get().info, ...payload, state };
    const next: Partial<UpdateStore> = { state, info };

    if (state !== "error") {
      if (!payload.messageCode) next.messageCode = "";
    }
    if (payload.messageCode) next.messageCode = payload.messageCode;

    if (payload.prompt) {
      const kind = (payload.promptKind || "") as "available" | "ready" | "message" | "error" | "";
      if (kind === "available" || kind === "ready" || kind === "message" || kind === "error") {
        next.card = { kind, key: ++cardSeq };
      }
    }
    set(next as UpdateStore);
  },

  dismissCard: () => set({ card: null }),
  openDialog: () => set({ dialogOpen: true, card: null }),
  closeDialog: () => set({ dialogOpen: false }),

  loadSettings: async () => {
    try {
      const s = await SettingsService.GetUpdateSettings();
      set({ settings: { autoCheck: !!s.autoCheck, autoDownload: !!s.autoDownload, skippedVersion: s.skippedVersion || "" } });
    } catch {
      /* 保持默认值 */
    }
  },

  saveSettings: async (patch) => {
    const next = { ...get().settings, ...patch };
    set({ settings: next });
    try {
      await SettingsService.SaveUpdateSettings(next as any);
    } catch (e) {
      console.error("保存更新设置失败:", e);
    }
  },

  checkNow: async () => {
    set({ state: "checking", messageCode: "" });
    try {
      await SettingsService.CheckForUpdates();
    } catch (e: any) {
      set({ state: "error", messageCode: "network", info: { ...get().info, state: "error", error: e?.message || String(e) } });
    }
  },

  download: async () => {
    try {
      await SettingsService.DownloadUpdate();
    } catch (e: any) {
      set({ state: "error", messageCode: "download_failed", info: { ...get().info, state: "error", error: e?.message || String(e) } });
    }
  },

  skipVersion: async () => {
    const version = get().info.version;
    if (!version) return;
    try {
      await SettingsService.SkipUpdateVersion(version);
      set({ settings: { ...get().settings, skippedVersion: version }, dialogOpen: false, card: null });
    } catch (e) {
      console.error("跳过版本失败:", e);
    }
  },
}));

let initialized = false;

/** 应用启动时调用一次：同步当前状态并订阅后端 update:* 事件。 */
export function initUpdateListeners(): () => void {
  if (initialized) return () => {};
  initialized = true;
  const store = useUpdateStore.getState();
  void store.loadSettings();
  SettingsService.GetUpdateStatus()
    .then((s) => useUpdateStore.getState().applyPayload({ ...(s as UpdatePayload), prompt: false }))
    .catch(() => undefined);

  const offs = [
    EventsOn<UpdatePayload>("update:state", (p) => useUpdateStore.getState().applyPayload(p)),
    EventsOn<UpdatePayload>("update:progress", (p) =>
      useUpdateStore.getState().applyPayload({ ...p, state: "downloading", prompt: false }),
    ),
    // ready / error 事件携带的是同一信息，状态已由 update:state 驱动；这里只补充 canInstall 与错误码
    EventsOn<UpdatePayload>("update:ready", (p) =>
      useUpdateStore.setState((s) => ({ info: { ...s.info, canInstall: p.canInstall, mandatory: p.mandatory } })),
    ),
    EventsOn<UpdatePayload>("update:error", (p) => useUpdateStore.getState().applyPayload(p.state === "error" ? p : { ...p, state: useUpdateStore.getState().state })),
  ];
  return () => {
    offs.forEach((off) => off());
    initialized = false;
  };
}
