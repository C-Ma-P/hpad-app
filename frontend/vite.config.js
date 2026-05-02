import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import wails from "@wailsio/runtime/plugins/vite";
export default defineConfig({
    plugins: [react(), wails("./bindings")],
    build: {
        outDir: "../internal/ui/dist",
    },
});
