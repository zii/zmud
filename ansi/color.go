package ansi

// 基础控制码
const (
	Reset     = "\x1b[0m" // 重置所有属性
	Bold      = "\x1b[1m" // 加粗/高亮
	Dim       = "\x1b[2m" // 昏暗模式
	Underline = "\x1b[4m" // 下划线
	Blink     = "\x1b[5m" // 闪烁
	Reverse   = "\x1b[7m" // 反色
	Hide      = "\x1b[8m" // 隐藏
)

// 前景色（标准色）
const (
	Black     = "\x1b[30m"
	Red       = "\x1b[31m"
	Green     = "\x1b[32m"
	Yellow    = "\x1b[33m"
	Blue      = "\x1b[34m"
	Magenta   = "\x1b[35m"
	Cyan      = "\x1b[36m"
	White     = "\x1b[37m"
	Straw     = "\x1b[38;2;214;214;161m" // 淡雅麦秆色
	CadetBlue = "\x1b[38;2;157;186;183m" // 复古青石色
	Thistle   = "\x1b[38;2;188;169;201m" // 朦胧浅紫色
)

// 前景色（高亮/加粗色 - MUD 最常用）
const (
	HiBlack   = "\x1b[1;30m"
	HiRed     = "\x1b[1;31m"
	HiGreen   = "\x1b[1;32m"
	HiYellow  = "\x1b[1;33m"
	HiBlue    = "\x1b[1;34m"
	HiMagenta = "\x1b[1;35m"
	HiCyan    = "\x1b[1;36m"
	HiWhite   = "\x1b[1;37m" // 比如：甘道夫的名字
)

// 背景色
const (
	BgBlack     = "\x1b[40m"
	BgRed       = "\x1b[41m"
	BgGreen     = "\x1b[42m"
	BgYellow    = "\x1b[43m"
	BgBlue      = "\x1b[44m"
	BgMagenta   = "\x1b[45m"
	BgCyan      = "\x1b[46m"
	BgWhite     = "\x1b[47m"
	BgTranslate = "\x1b[48;2;107;107;107m" // 译文背景色
)
