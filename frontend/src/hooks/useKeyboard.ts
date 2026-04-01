import { useEffect } from "react";

type KeyHandler = (e: KeyboardEvent) => void;

interface ShortcutMap {
  [key: string]: KeyHandler;
}

export function useKeyboard(shortcuts: ShortcutMap) {
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      const mod = e.metaKey || e.ctrlKey;

      for (const [combo, fn] of Object.entries(shortcuts)) {
        const parts = combo.toLowerCase().split("+");
        const needMod = parts.includes("mod");
        const needShift = parts.includes("shift");
        const needAlt = parts.includes("alt");
        const needCtrl = parts.includes("ctrl");
        const key = parts.filter((p) => !["mod", "shift", "alt", "ctrl"].includes(p))[0];

        if (needMod && !mod) continue;
        if (needShift && !e.shiftKey) continue;
        if (needAlt && !e.altKey) continue;
        if (needCtrl && !e.ctrlKey) continue;
        // 避免误触：如果快捷键不需要 shift 但按下了 shift，则跳过（精确匹配修饰键）
        if (!needShift && e.shiftKey) continue;
        if (!needAlt && e.altKey) continue;
        if (e.key.toLowerCase() !== key && e.code.toLowerCase() !== key) continue;

        e.preventDefault();
        fn(e);
        return;
      }
    };

    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [shortcuts]);
}
