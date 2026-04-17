package lib

// 语言显示模式类型
type Lang int

// 语言显示模式常量
const (
	LSRC Lang = 1 // 原文 (source)
	LTRN Lang = 2 // 译文 (translate)
	LMIX Lang = 3 // 双语 (mix, 默认)
)

// Lang 的字符串表示
func (l Lang) String() string {
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
