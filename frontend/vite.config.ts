import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "path";

export default defineConfig({
  plugins: [react()],
  server: {
    host: "127.0.0.1",
    port: 34219,
    strictPort: true,
    cors: true,
    allowedHosts: ["wails.localhost", "localhost", "127.0.0.1"],
    hmr: {
      protocol: "ws",
      host: "localhost",
      port: 34219,
      clientPort: 34219,
    },
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
});
