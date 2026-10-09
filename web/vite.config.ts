import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The built files go where the Go server embeds them (internal/web/dist).
// During `npm run dev`, the Go server on :8080 answers the API and sign-in.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: "../internal/web/dist",
    emptyOutDir: true,
  },
  server: {
    proxy: {
      "/api": "http://127.0.0.1:8080",
      "/signin": "http://127.0.0.1:8080",
      "/signout": "http://127.0.0.1:8080",
    },
  },
});
