/**
 * monaco 本地化配置：将 @monaco-editor/react 的 loader 指向本地打包的 monaco，
 * 避免桌面应用在运行时从 jsdelivr CDN 拉取编辑器资源（离线/内网环境下无法使用）。
 *
 * 该模块由 SQLEditor / DDLViewer 懒加载 chunk 引入，monaco 本体也只随编辑器按需加载。
 */
import { loader } from "@monaco-editor/react";
import * as monaco from "monaco-editor";
import EditorWorker from "monaco-editor/esm/vs/editor/editor.worker?worker";

declare global {
  interface Window {
    MonacoEnvironment?: {
      getWorker: (workerId: string, label: string) => Worker;
    };
  }
}

// 本地 worker：SQL/纯文本不需要语言专属 worker，所有服务共用编辑器 worker 即可
window.MonacoEnvironment = {
  getWorker() {
    return new EditorWorker();
  },
};

loader.config({ monaco });

export { monaco };