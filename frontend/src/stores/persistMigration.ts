export function migratePersistedKey(currentKey: string, legacyKey: string) {
  if (typeof window === "undefined" || currentKey === legacyKey) return;
  try {
    const storage = window.localStorage;
    if (storage.getItem(currentKey) != null) return;
    const legacyValue = storage.getItem(legacyKey);
    if (legacyValue == null) return;
    storage.setItem(currentKey, legacyValue);
  } catch {
    // localStorage may be unavailable in restricted browser contexts.
  }
}
