import { defineConfig } from 'vite';

export default defineConfig({
  base: '/hyperdeck-adapter/',
  build: {
    target: 'es2022',
    sourcemap: false,
  },
});
