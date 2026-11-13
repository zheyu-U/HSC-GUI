import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'

// 本地预览夹具 frontend/dev-preview/ 不入库，见 .gitignore；仅 Vite 开发服务器使用。
export default defineConfig({
  plugins: [vue()],
  server: {
    watch: {
      // 编辑器原子写会先在同目录建 *.tmpdir 临时目录，Vite 默认会去 watch 里面的
      // 锁文件并报 EBUSY 把开发服务器打崩；忽略这类临时路径。
      ignored: ['**/*.tmpdir', '**/*.tmpdir/**', '**/.*.tmpdir', '**/.*.tmpdir/**'],
    },
  },
})
