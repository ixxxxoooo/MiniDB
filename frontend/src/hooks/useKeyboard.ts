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
        const key = parts.filter((p) => !["mod", "shift"].includes(p))[0];

        if (needMod && !mod) continue;
        if (needShift && !e.shiftKey) continue;
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
