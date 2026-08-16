import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

// 构建产物写入 Go 内嵌目录，开发态将 /api 代理到后端。
export default defineConfig({
  plugins: [
    vue({
      template: {
        compilerOptions: {
          isCustomElement: (tag) => tag.startsWith("media-"),
        },
      },
    }),
  ],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: process.env.LITEPAN_API_PROXY || "http://127.0.0.1:38088",
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: "../internal/api/web",
    emptyOutDir: true,
    // 解码器按需分包，阈值略高于当前最大独立产物。
    chunkSizeWarningLimit: 3200,
    rollupOptions: {
      output: {
        // 禁用按需拆分：defineAsyncComponent 页面与主入口共享模块级状态
        // （useAdminLoadingBar 的 ref/Set）时，rollup 自动拆包会产生循环
        // chunk 依赖（TDZ: Cannot access 'R'/'L' before initialization）。
        // 全部打进主 bundle，牺牲首屏体积换取无循环依赖的确定性。
        inlineDynamicImports: true,
      },
    },
  },
});
