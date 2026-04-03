import { useEffect } from "react";

type KeyHandler = (e: KeyboardEvent) => void;

interface ShortcutMap {
  [key: string]: KeyHandler;
}

export function useKeyboard(shortcuts: ShortcutMap) {
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      for (const [combo, fn] of Object.entries(shortcuts)) {
        const parts = combo.toLowerCase().split("+");
        const needMod = parts.includes("mod");
        const needShift = parts.includes("shift");
        const needAlt = parts.includes("alt");
        const needCtrl = parts.includes("ctrl");
        const key = parts.filter((p) => !["mod", "shift", "alt", "ctrl"].includes(p))[0];

        const isMac = navigator.platform.toUpperCase().includes("MAC");
        const primaryModPressed = isMac ? e.metaKey : e.ctrlKey;
        const extraCtrlPressed = isMac ? e.ctrlKey : false;

        if (needMod && !primaryModPressed) continue;
        if (needShift && !e.shiftKey) continue;
        if (needAlt && !e.altKey) continue;
        if (needCtrl && !e.ctrlKey) continue;
        // 避免误触：未声明的修饰键按下时跳过，要求精确匹配
        if (!needShift && e.shiftKey) continue;
        if (!needAlt && e.altKey) continue;
        if (!needCtrl && extraCtrlPressed) continue;
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
