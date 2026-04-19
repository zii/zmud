# zmud

一款专为 MUD 游戏打造的智能翻译终端, 帮中国玩家跨越语言障碍, 体验古老的英文Mud.  
结合AI助手, 大大缓解文字游戏难以上手的痛点.

<img src="screenshot.png" width="600">

## 功能特性

- 用 Go 语言实现的 Telnet 客户端
- 支持 Baidu、DeepSeek、Kilo 翻译引擎
- 向AI助手咨询任何问题, 或让AI自动给出最佳操作命令
- 翻译结果自动缓存，避免重复请求
- 批量翻译预热(warmup)，大幅提升翻译速度
- 自带热门游戏服务器列表，支持自定义添加删除游戏
- 上下键切换历史命令
- Tab 键自动补命令
- F1/F2 实时重绘当前屏幕, 秒切原文/译文/混合模式
- 支持gb/big5/utf-8编码, 支持中文mud
- 线性指令步进脚本引擎
- 触发器：服务器返回文本匹配时自动执行命令

## 步进脚本语法

步进脚本引擎支持线性指令执行，适合自动化练级、刷怪等重复操作。

### 基础语法

- **分号 `;`**：分隔指令，顺序执行
- **冒号 `:`**：发送命令后阻塞等待，直到服务器返回关键字或超时

### 内置指令

| 指令 | 说明 | 示例 |
|------|------|------|
| `#wa 时间` | 等待一段时间 | `#wa 2s` 等待2秒 |
| `#loop` | 跳回第一条指令重新执行 | 配合 `#wa` 实现循环 |
| `#jmp N` | 跳转到第 N 条指令 | `#jmp 1` 跳回第一条 |
| `#N` | 重复一条指令 N 次 | `#3 e` 连续执行3次 e |

### 时间格式

- `500ms` - 500毫秒
- `1s` - 1秒
- `2.5s` - 2.5秒
- 纯数字默认毫秒

### 玩法举例

#### 例1：自动打坐练级
```
cast dough;e;#wa 1s;kill meng;jie;e;#wa 1s;kill meng;jie;e;#wa 1s;#loop
```

#### 例2：举石锁脚本
```
get suo;ju;#wa 2s;#jmp 2
```

#### 例3：等待特定关键字
```
look:正厅;e:东厢房;sleep:醒来;w
```

### 安全机制

- `#loop` 强制要求前面有足够的 `#wa` 延迟（总计 >= 1秒）
- 任何输入都会中断当前运行中的脚本

## 系统命令

- /e [baidu|deepseek|kilo] - 切换翻译引擎
- /e update - 更新引擎参数
- /ask <问题> - 询问 AI 任何问题
- /hint - 根据最新上下文获取 AI 建议
- /quit - 退出游戏
- /alias - 打印列表，暂无别名时提示
- /alias key - 打印该别名
- /alias key DELETE - 删除别名
- /alias key value - 设置别名
- /trigger - 打印列表，暂无触发器时提示
- /trigger pattern - 打印该触发器
- /trigger pattern DELETE - 删除触发器
- /trigger pattern command - 设置触发器
- /trigger "带空格的 pattern" command - 支持带空格的 pattern

### 触发器举例

```
/trigger "Input 1 for GBK, 2 for UTF8, 3 for BIG5" 1
/trigger 将*放入了 look;#wa 2s;get all
```

- 第一个：服务器显示编码选择时自动输入 1
- 第二个：捡东西时自动先 look 再 get all