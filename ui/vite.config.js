import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import { tanstackRouter } from '@tanstack/router-plugin/vite';
export default defineConfig({
    plugins: [tanstackRouter({ quoteStyle: 'single' }), react()],
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
        environment: 'jsdom',
        include: ['src/**/*.test.{ts,tsx}'],
        setupFiles: ['src/test-setup.ts'],
    },
});
