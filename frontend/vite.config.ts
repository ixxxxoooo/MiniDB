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
  build: {
    target: "es2020",
    rollupOptions: {
      output: {
        manualChunks(id) {
          // 把体积大且独立的大依赖拆成独立 chunk，配合 React.lazy 按需加载
          if (id.includes("node_modules/monaco-editor/")) {
            return "monaco";
          }
          if (id.includes("node_modules/@tiptap") || id.includes("node_modules/tiptap-markdown")) {
            return "tiptap";
          }
          if (id.includes("node_modules/mermaid")) {
            return "mermaid";
          }
          // 其余 node_modules 归入单一 vendor chunk（含 react 全家桶），便于缓存
          if (id.includes("node_modules")) {
            return "vendor";
          }
        },
      },
    },
  },
});