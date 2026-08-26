import React, { useEffect, Component, type ReactNode } from "react";
import { TooltipProvider } from "@/components/ui/tooltip";
import { AppLayout } from "@/components/layout/AppLayout";
import { TitleTooltipBridge } from "@/components/ui/TitleTooltipBridge";
import { trackAppLaunch } from "@/lib/analytics";

interface ErrorBoundaryProps {
  children: ReactNode;
}

interface ErrorBoundaryState {
  hasError: boolean;
  error: Error | null;
}

class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    console.error("[ErrorBoundary] Caught error:", error, errorInfo);
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="min-h-screen w-full flex flex-col items-center justify-center bg-[var(--surface,#18181b)] text-[var(--fg,#f4f4f5)] p-6 font-sans">
          <div className="max-w-md w-full p-6 rounded-[var(--radius-panel)] border border-red-500/30 bg-red-500/10 text-left space-y-4">
            <h2 className="text-lg font-semibold text-red-400">渲染发生错误 (Rendering Error)</h2>
            <p className="text-sm text-red-300 font-mono break-all leading-relaxed">
              {this.state.error?.message || "未知组件运行错误"}
            </p>
            <pre className="text-xs text-zinc-400 overflow-auto max-h-40 p-2 bg-black/40 rounded font-mono">
              {this.state.error?.stack}
            </pre>
            <button
              onClick={() => window.location.reload()}
              className="px-4 py-2 text-xs font-medium rounded bg-red-600 hover:bg-red-500 text-white transition-colors"
            >
              刷新页面 (Reload Page)
            </button>
          </div>
        </div>
      );
    }
    return this.props.children;
  }
}

export default function App() {
  useEffect(() => {
    void trackAppLaunch();
  }, []);

  return (
    <ErrorBoundary>
      <TooltipProvider>
        <AppLayout />
        <TitleTooltipBridge />
      </TooltipProvider>
    </ErrorBoundary>
  );
}
