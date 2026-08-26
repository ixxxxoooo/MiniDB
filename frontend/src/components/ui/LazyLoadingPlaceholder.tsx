// 懒加载组件的通用占位：避免重复写 Suspense fallback
export function LazyLoadingPlaceholder({ height = "100%" }: { height?: string | number }) {
  return (
    <div
      className="flex items-center justify-center text-[var(--fg-muted)] text-[length:var(--size-font-xs)] select-none"
      style={{ height }}
    >
      Loading…&nbsp;<span className="animate-pulse">▮</span>
    </div>
  );
}