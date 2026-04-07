import React from "react";
import { TooltipProvider } from "@/components/ui/tooltip";
import { AppLayout } from "@/components/layout/AppLayout";
import { TitleTooltipBridge } from "@/components/ui/TitleTooltipBridge";

export default function App() {
  return (
    <TooltipProvider delayDuration={300}>
      <AppLayout />
      <TitleTooltipBridge />
    </TooltipProvider>
  );
}
