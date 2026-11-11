//go:build dev

package hsc

// devMode 由 Wails 开发模式的 dev 构建标签决定：wails dev 的 state 留在项目根目录，
// 与本地命令行调试一致；打包后的 exe 一律用 exe 所在目录。
const devMode = true
