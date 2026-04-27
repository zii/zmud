# zmud

为MUD玩家打造的AI助手！AI驱动的MUD翻译终端 + 自动化脚本引擎

1. **翻译层** - 把英文MUD服务器返回的文本实时翻译成中文（Baidu/Google/DeepSeek/Kilo）
2. **AI助手** - 可以直接 `/ask "怎么走出这个迷宫"` 让AI帮你决策
3. **脚本系统** - 用 `#loop`、`#if`、`#jmp` 写步进脚本自动打怪练级
4. **触发器** - 匹配关键字自动执行命令

<img src="screenshot.png" width="600">

## 下载

**v0.3** (2026-04-26)

| 平台 | 大小 | 下载地址 |
|------|------|----------|
| macOS Intel | 11MB | [zmud-darwin-amd64](https://github.com/zii/zmud/releases/download/v0.3/zmud-darwin-amd64) |
| Linux x86_64 | 14MB | [zmud-linux-amd64](https://github.com/zii/zmud/releases/download/v0.3/zmud-linux-amd64) |
| Windows x86_64 | 14MB | [zmud-windows-amd64.exe](https://github.com/zii/zmud/releases/download/v0.3/zmud-windows-x86_64.exe) |

## 主要特性

- Go语言实现的 Telnet 客户端，支持多平台
- Baidu、DeepSeek、Kilo 翻译引擎
- AI助手：提问/自动建议/直接给出操作指令
- 翻译自动缓存，批量预热加速
- 内置热门MUD服务器列表，支持自定义
- 命令历史（上下键）、Tab自动补全
- F1/F2 切换原文/译文/混合显示模式
- 支持 GB/BIG5/UTF-8 编码中文MUD
- **线性步进脚本引擎**（#loop/#if/#jmp）
- **触发器系统**：文本匹配自动执行

## 步进脚本语法

脚本引擎按顺序执行指令，配合变量捕获和条件判断，适合自动化练级、刷怪等重复操作。

### 基础写法

- 用 **`;`** 分隔指令，顺序执行
- 用 **`:`** 等待服务器返回关键字（支持通配符捕获）
- 指令前后空格自动忽略

```
# 基础流程
look:正厅;e;kill 怪物;#wa 2s;#loop
```

### 内置指令速查

| 指令 | 说明 | 示例 |
|------|------|------|
| `#wa <时间>` | 等待（可被中断） | `#wa 2s`、`#wa 500ms` |
| `#loop` | 跳回第一条指令 | 配合 `#wa` 实现循环 |
| `#jmp N` | 跳转到第 N 条指令 | `#jmp 1`、`#jmp +2`、`#jmp -1` |
| `#gap N` | 设置命令间停顿 | `#gap 0.5`（0.5秒） |
| `#if <条件> <动作>` | 条件判断 | 见下文详解 |
| `#N <命令>` | 重复执行 N 次 | `#3 e`（走3次） |
| `%N <命令>` | 概率执行（0-100） | `%30 drink`（30%概率） |

### 条件判断：`#if`

**语法**: `#if <表达式> <动作列表>`

条件为真时执行动作。动作用逗号分隔，支持多个。

**表达式** 支持：
- 数字比较：`>`、`<`、`>=`、`<=`、`=`、`!=`
- 字符串比较：`"东" = "东"`、`$1 = 西`
- 变量引用：`$nl`、`$hp`、`$1`（位置捕获）

**动作类型**（按顺序执行，遇到终止性动作立即停止）：
- **数字**：跳转到指定指令编号（1-based）
- **`break`**：终止整个脚本
- **普通命令**：执行该命令后继续下一个
- **`#wa`**：等待指定时间（可放在动作列表中）

**示例**:
```
# 气血<30时先吃药，再吃饭，然后终止
#if $hp<30 drink,eat,break

# 内力充足则打坐，否则睡觉
enforce:内力{nl}/*;#if $nl>100 dazuo,sleep

# 匹配"东"后等待1.5秒，然后说"到了"
#if $1="东" #wa 1.5s,say 到了

# 循环练功：内力<100跳回打坐，否则睡觉
dazuo;#if $nl<100 1;sleep:醒来;#loop
```

### 变量引用与捕获

#### 捕获变量
在 `命令:关键字` 中，用 `*` 或 `{name}` 捕获服务器返回的内容：

```
# * 捕获第1段
hp:气血*/*;dazuo $1        # → 气血 100/130 → dazuo 100

# {name} 命名捕获
hp:气血{hp}/{maxhp};dazuo $hp  # → $hp=100

# 多个捕获
look:你觉得*正在*;#say $1 $2
```

#### 四则运算
变量后直接跟 `+`/`-`/`*`/`/` 数字：

```
dazuo $hp-20      # 打坐到 (当前气血-20)
eat $full*0.8     # 吃到 80% 饱足度
```

### 时间格式

`#wa` 和 `#gap` 使用：
- `500ms` - 500毫秒
- `1s` - 1秒
- `2.5s` - 2.5秒
- 纯数字（如 `500`）默认为毫秒

### 常用脚本示例

```text
# 例1：自动打坐练级（内力<100继续，否则睡觉）
enforce:内力{nl}/*;dazuo;#if $nl<100 1;sleep:醒来;#loop

# 例2：循环刷怪
e;kill 小怪;#wa 3s;get all from corpse;#jmp 2

# 例3：条件判断练功
hp:气血{hp}/*;#if $hp<200 drink potion,say need heal

# 例4：等待特定回复后继续
look:正厅;e:东厢房;sleep:醒来;w

# 例5：别名与跳转结合
/alias chihe hp:气血{hp}/*;drink $A1
chihe 100;#loop
```

## 别名

通过 `/alias` 定义命令缩写，支持位置参数 `$A1`~`$A9`。

```
/alias chihe hp:气血{hp}/*;drink $A1
chihe 80    # 展开为：气血...;drink 80

# 别名可在脚本中直接使用，索引不因展开而改变
chihe 100;#jmp 1   # #jmp 指向原始的 chihe
```

别名也支持在 `%N`、`#N`、`cmd:keyword` 中使用。

## 触发器

服务器返回文本匹配模式时自动执行命令。

```
/trigger "状态: 缠住" exert chanwu
/trigger 掉落了* get all from corpse
```

支持 `*` 和 `{name}` 捕获变量，在命令中用 `$1`、`$name` 引用。

## 系统命令

| 命令 | 说明 |
|------|------|
| `/e [baidu\|deepseek\|kilo]` | 切换翻译引擎 |
| `/e update` | 更新引擎API参数 |
| `/ask <问题>` | 询问AI任何问题 |
| `/hint` | 获取基于上下文的AI建议 |
| `/quit` | 退出游戏 |
| `/alias` | 查看别名列表 |
| `/alias key DELETE` | 删除别名 |
| `/alias key value` | 设置别名 |
| `/trigger` | 查看触发器列表 |
| `/trigger pattern DELETE` | 删除触发器 |
| `/trigger pattern command` | 添加触发器 |

## 安全

- `#loop` 要求前面 `#wa` 总延迟 ≥1秒，防止死循环卡死
- 任何键盘输入会立即中断当前运行的脚本
