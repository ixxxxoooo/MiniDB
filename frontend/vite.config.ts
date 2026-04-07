import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "path";

const DEV_PORT = 43817;

export default defineConfig({
  plugins: [react()],
  server: {
    host: "127.0.0.1",
    port: DEV_PORT,
    strictPort: true,
    cors: true,
    allowedHosts: ["wails.localhost", "localhost", "127.0.0.1"],
    hmr: {
      protocol: "ws",
      host: "localhost",
      port: DEV_PORT,
      clientPort: DEV_PORT,
    },
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
});
