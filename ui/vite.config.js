import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import { tanstackRouter } from '@tanstack/router-plugin/vite';
import { barrel } from 'vite-plugin-barrel';
export default defineConfig({
    plugins: [
        tanstackRouter({ quoteStyle: 'single' }),
        react(),
        barrel({ packages: ['@mui/material', '@mui/icons-material'] }),
    ],
    build: { outDir: 'dist' },
    server: {
        port: 7610,
        proxy: {
            '/api': {
                target: 'http://localhost:7600',
                changeOrigin: true,
            },
        },
    },
    test: {
        environment: 'happy-dom',
        include: ['src/**/*.test.{ts,tsx}'],
        setupFiles: ['src/test-setup.ts'],
        pool: 'vmThreads',
    },
});
