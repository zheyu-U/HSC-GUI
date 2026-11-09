// Vite 构建会清空 dist。补一个占位文件，保证：
// 1) 仓库里始终存在 frontend/dist 目录（git 只跟踪文件，不跟踪空目录）
// 2) 全新克隆后 main.go 的 //go:embed all:frontend/dist 能编译通过
import {mkdirSync, writeFileSync} from 'node:fs'

mkdirSync('dist', {recursive: true})
writeFileSync('dist/gitkeep', '')
