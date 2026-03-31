import React from "react";
import { TooltipProvider } from "@/components/ui/tooltip";
import { AppLayout } from "@/components/layout/AppLayout";

export default function App() {
  return (
    <TooltipProvider delayDuration={300}>
      <AppLayout />
    </TooltipProvider>
  );
}
