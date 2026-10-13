import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

// Built into ui/dist, embedded in the Go binary (ui/embed.go) and served at the root of the site.
// npm run dev: the API calls go to a monitor running on localhost:8080.
export default defineConfig({
  base: '/',
  plugins: [svelte()],
  build: { outDir: 'dist', emptyOutDir: true, assetsDir: 'assets' },
  server: { proxy: { '/api': 'http://localhost:8080', '/me': 'http://localhost:8080' } }
});
