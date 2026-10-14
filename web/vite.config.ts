import { writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath, URL } from 'node:url'

import vue from '@vitejs/plugin-vue'
import type { Plugin } from 'vite'
import { defineConfig } from 'vitest/config'

/**
 * dist/ 的内容被 .gitignore 忽略，但 web/embed.go 的 `//go:embed all:dist`
 * 要求这个目录存在，否则新克隆的仓库 `go build` 会直接失败。
 * 空目录 git 存不下，所以放一个 .gitkeep；而 build.emptyOutDir 会连它一起删掉，
 * 于是构建结束时把它补回来。
 */
function keepDistPlaceholder(): Plugin {
  let outDir = 'dist'
  return {
    name: 'keep-dist-placeholder',
    configResolved(config) {
      outDir = config.build.outDir
    },
    closeBundle() {
      writeFileSync(join(outDir, '.gitkeep'), '')
    },
  }
}

export default defineConfig({
  plugins: [vue(), keepDistPlaceholder()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  build: {
    // Go 侧 //go:embed all:dist 指的就是这里，改路径要同步改 web/embed.go
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    // 开发时前端独立跑，接口反代到本机后端（生产是同源，由 Go 一并提供）
    proxy: {
      '/api': 'http://127.0.0.1:8737',
    },
  },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.ts'],
    setupFiles: ['src/test-setup.ts'],
  },
})
