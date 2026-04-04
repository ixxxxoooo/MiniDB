import { useEffect } from "react";

type KeyHandler = (e: KeyboardEvent) => void;

interface ShortcutMap {
  [key: string]: KeyHandler;
}

export function useKeyboard(shortcuts: ShortcutMap) {
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.isComposing) return;
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
        const eventKey = e.key.toLowerCase();
        const eventCode = e.code.toLowerCase();
        const normalizedKey = key || "";
        const bracketCodeAlias =
          normalizedKey === "["
            ? "bracketleft"
            : normalizedKey === "]"
              ? "bracketright"
              : "";
        if (eventKey !== normalizedKey && eventCode !== normalizedKey && (!bracketCodeAlias || eventCode !== bracketCodeAlias)) {
          continue;
        }

        e.preventDefault();
        fn(e);
        return;
      }
    };

    // 捕获阶段优先处理，避免 Monaco 等编辑器先吞掉组合键导致全局快捷键失效
    window.addEventListener("keydown", handler, { capture: true });
    return () => window.removeEventListener("keydown", handler, { capture: true });
  }, [shortcuts]);
}
