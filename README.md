# zmud

一款专为 MUD 游戏打造的翻译终端, 帮你跨越语言障碍, 玩转英文Mud.

## 功能特性

- 用 Go 语言实现的 Telnet 客户端
- 支持 Baidu、DeepSeek、Kilo 翻译引擎
- 翻译结果自动缓存，避免重复请求
- 批量翻译预热(warmup)，大幅提升翻译速度
- 自带常用游戏服务器列表，支持自定义添加删除服务器
- 上下键切换历史命令
- Tab 键补全
- F1/F2 快速切换原文/译文/混合模式

## 系统命令

- /e [baidu|deepseek|kilo] - 切换翻译引擎
- /e update - 更新引擎参数
- /ask <问题> - 询问 AI
- /hint - 获取 AI 建议
- /quit - 退出游戏