import { defineConfig }           from 'vite';
import { svelte, vitePreprocess } from '@sveltejs/vite-plugin-svelte';
import fs from 'fs/promises';

const updateAirFile = () => ({
  name: "update-air-file",
  apply: "build",
  writeBundle: async () => {
    await fs.writeFile("../.air.update", new Date().toLocaleString("SWE"));
  },
});

// see https://vitejs.dev/config
export default defineConfig({
    server: {
        port: 3000,
    },
    envPrefix: 'PB',
    base: './',
    build: {
        chunkSizeWarningLimit: 1000,
        reportCompressedSize: false,
    },
    plugins: [
        svelte({
            preprocess: [vitePreprocess()],
            onwarn: (warning, handler) => {
                if (warning.code.startsWith('a11y-')) {
                    return; // silence a11y warnings
                }
                handler(warning);
            },
        }),
        updateAirFile()
    ],
    resolve: {
        alias: {
            '@': __dirname + '/src',
        }
    },
})
