//go:build !dev

package hsc

// devMode 为 false：普通构建（release / debug / 测试）都按打包后的行为处理，
// state 目录跟着 exe 走。
const devMode = false
