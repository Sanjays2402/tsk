import { defineConfig } from "vite";

// We ship the built SPA into the Go binary via go:embed at
// internal/serve/web_dist/. Vite writes there directly so a `go build`
// after a `npm run build` always picks up the latest assets.
export default defineConfig({
  root: ".",
  base: "./",
  build: {
    outDir: "../internal/serve/web_dist",
    emptyOutDir: true,
    target: "es2022",
    cssCodeSplit: false,
    sourcemap: false,
    rollupOptions: {
      output: {
        // Stable, content-hashed filenames so the Go embed picks up cleanly
        // and the browser cache can be aggressive.
        entryFileNames: "assets/app-[hash].js",
        chunkFileNames: "assets/chunk-[hash].js",
        assetFileNames: "assets/[name]-[hash][extname]",
      },
    },
  },
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      // dev-server convenience: forward /api/* to a locally running `tsk serve`
      "/api": "http://127.0.0.1:7878",
    },
  },
});
