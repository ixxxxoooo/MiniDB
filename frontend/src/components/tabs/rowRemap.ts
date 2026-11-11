/** 删除若干行后，原下标在新数组中的位置；被删除的行返回 -1。 */
export function makeRowRemap(removed: Set<number>): (idx: number) => number {
  const sorted = Array.from(removed).sort((a, b) => a - b);
  return (idx) => {
    if (removed.has(idx)) return -1;
    let shift = 0;
    for (const r of sorted) {
      if (r < idx) shift += 1;
      else break;
    }
    return idx - shift;
  };
}

export function remapIndexSet(set: Set<number>, remap: (idx: number) => number): Set<number> {
  const next = new Set<number>();
  for (const idx of set) {
    const mapped = remap(idx);
    if (mapped >= 0) next.add(mapped);
  }
  return next;
}

/** editedCells 的 key 形如 "行号:列名"（列名本身可含冒号）。 */
export function remapEditedCells(cells: Record<string, unknown>, remap: (idx: number) => number): Record<string, unknown> {
  const next: Record<string, unknown> = {};
  for (const [key, val] of Object.entries(cells)) {
    const sep = key.indexOf(":");
    const mapped = remap(parseInt(key.slice(0, sep), 10));
    if (mapped >= 0) next[`${mapped}:${key.slice(sep + 1)}`] = val;
  }
  return next;
}
