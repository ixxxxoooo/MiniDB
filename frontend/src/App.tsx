import React, { useEffect } from "react";
import { TooltipProvider } from "@/components/ui/tooltip";
import { AppLayout } from "@/components/layout/AppLayout";
import { TitleTooltipBridge } from "@/components/ui/TitleTooltipBridge";
import { trackAppLaunch } from "@/lib/analytics";

export default function App() {
  useEffect(() => {
    void trackAppLaunch();
  }, []);

  return (
    <TooltipProvider>
      <AppLayout />
      <TitleTooltipBridge />
    </TooltipProvider>
  );
}
