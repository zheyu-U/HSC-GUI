# hsc 课程助手 v1.0.0-beta

[hust-smartcourse-cli](https://github.com/xcdoge/hust-smartcourse-cli) 的 Windows 图形界面首个公开测试版：
扫码/账密登录、刷课、文档、作业测验与自动答题，单文件 exe，与命令行共用 `state/`。

> beta：核心流程已在真实账号上跑通，目前只做过 Windows 验证。

## 下载

`hsc-gui.exe`（13.3 MB）——Windows 10/11 + WebView2，免安装。
SHA-256：`99b82cb806decf5e2e61d7bdcddaa7eafd0eb9817a51a4007b11ea04a0fca30c6`

放进一个空文件夹再运行，`state/` 会生成在 exe 旁边；升级只替换 exe。

## 这一版

- 视频：倍速 1–25、上报间隔 1–30 秒，批量播未完成，没通过的任务点标红「失败」
- 文档：扫描 + 一键完成未完成文档，结果按服务器确认的数量报告
- 作业测验：自动扫描/抓题、AI 自动答题（DeepSeek）、二次确认交卷、批阅结果与题库
- debug 版多调试面板，并把调用明细写进 `state/logs`（不记密码、cookie、Key）

## 实测（2026-11-13，真账号 + 空 state）

12 门课 / 53 章 / 49 视频 / 49 文档 / 50 份作业测验；文档 49/49 完成，
自动答题 228/230 题，交卷 50 份成功 0 失败，批阅结果全部「已批阅」。

## 注意

- 高倍速（尤其 25 倍）平台可能连续判不过，建议 2–5 倍
- 自动答题需自备 DeepSeek API Key：<https://platform.deepseek.com/api_keys>
- 限速课按自设倍速播可能不判定通过，需补看或降速重试
- 仅供个人学习使用，账号安全由使用者自负

完整变更：<https://github.com/zheyu-U/HSC-GUI/compare/f5c9cd5...v1.0.0-beta> ·
使用说明：[README](https://github.com/zheyu-U/HSC-GUI/blob/main/README.md)
