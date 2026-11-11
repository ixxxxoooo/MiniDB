import { create } from "zustand";

export interface ConfirmOptions {
  title: string;
  message?: string;
  confirmText?: string;
  cancelText?: string;
  /** 危险操作：确认按钮用红色 */
  danger?: boolean;
  /** 要求用户输入这段文字后才能确认（如生产库删表要求输入表名） */
  requireText?: string;
}

interface ConfirmState {
  current: (ConfirmOptions & { resolve: (ok: boolean) => void }) | null;
  request: (options: ConfirmOptions) => Promise<boolean>;
  resolve: (ok: boolean) => void;
}

export const useConfirmStore = create<ConfirmState>((set, get) => ({
  current: null,
  request: (options) =>
    new Promise<boolean>((resolve) => {
      // 同一时刻只保留一个确认框；新请求到来时，旧的按取消处理
      get().current?.resolve(false);
      set({ current: { ...options, resolve } });
    }),
  resolve: (ok) => {
    const current = get().current;
    if (!current) return;
    set({ current: null });
    current.resolve(ok);
  },
}));

/** 弹出应用内确认框，返回用户是否确认。替代原生 confirm()。 */
export function requestConfirm(options: ConfirmOptions): Promise<boolean> {
  return useConfirmStore.getState().request(options);
}
