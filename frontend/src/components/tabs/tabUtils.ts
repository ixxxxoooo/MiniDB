export function extractJSONFromText(text: string): any | null {
  const direct = text.trim();
  try {
    return JSON.parse(direct);
  } catch {}
  const codeMatch = direct.match(/```json\s*([\s\S]*?)```/i);
  if (codeMatch) {
    try {
      return JSON.parse(codeMatch[1].trim());
    } catch {}
  }
  const objectMatch = direct.match(/\{[\s\S]*\}/);
  if (objectMatch) {
    try {
      return JSON.parse(objectMatch[0]);
    } catch {}
  }
  return null;
}

export function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  const tagName = target.tagName;
  if (tagName === "INPUT" || tagName === "TEXTAREA" || tagName === "SELECT" || tagName === "BUTTON") {
    return true;
  }
  if (target.isContentEditable) {
    return true;
  }
  return Boolean(target.closest("input, textarea, select, button, [contenteditable='true'], .monaco-editor, .monaco-inputbox, [role='textbox']"));
}

export function isGridTarget(target: EventTarget | null): boolean {
  return target instanceof HTMLElement && Boolean(target.closest("[role='grid']"));
}
