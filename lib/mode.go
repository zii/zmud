package lib

// 语言显示模式类型
type Mode int

// 显示模式常量
const (
	LSRC Mode = 1 // 原文 (source)
	LTRN Mode = 2 // 译文 (translate)
	LMIX Mode = 3 // 双语 (mix, 默认)
)

// Mode 的字符串表示
func (l Mode) String() string {
	switch l {
	case LSRC:
		return "原文"
	case LTRN:
		return "译文"
	case LMIX:
		return "双语"
	default:
		return "未知"
	}
}
