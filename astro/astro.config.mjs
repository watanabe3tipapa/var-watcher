import { defineConfig } from 'astro/config'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  site: 'https://watanabe3tipapa.github.io/var-watcher',
  base: '/var-watcher/',
  vite: {
    plugins: [tailwindcss()],
  },
})
