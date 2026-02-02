package util

// 检测字符串是否包含中文字符
func IsChinese(text string) bool {
	for _, r := range text {
		// 中文字符Unicode范围
		if (r >= 0x4E00 && r <= 0x9FFF) || // CJK统一汉字
			(r >= 0x3400 && r <= 0x4DBF) || // CJK扩展A
			(r >= 0x20000 && r <= 0x2A6DF) || // CJK扩展B
			(r >= 0x2A700 && r <= 0x2B73F) || // CJK扩展C
			(r >= 0x2B740 && r <= 0x2B81F) || // CJK扩展D
			(r >= 0x2B820 && r <= 0x2CEAF) || // CJK扩展E
			(r >= 0x2CEB0 && r <= 0x2EBEF) || // CJK扩展F
			(r >= 0x3000 && r <= 0x303F) || // CJK符号和标点
			(r >= 0xFF00 && r <= 0xFFEF) { // 全角ASCII、全角标点
			return true
		}
	}
	return false
}

// 检测语言类型
func DetectLanguage(text string) string {
	if text == "" {
		return "en" // 默认英文
	}

	// 优先检测中文
	if IsChinese(text) {
		return "zh"
	}

	// 然后检测日文
	if IsJapanese(text) {
		return "ja"
	}

	// 默认英文
	return "en"
}

// 根据语言处理查询字符串
func ProcessQueryByLanguage(text, language string) string {
	switch language {
	case "ja":
		return ToRomaji(text) // 日语转罗马字
	case "zh":
		return text // 中文保持不变
	default:
		return text // 英文等其他语言保持不变
	}
}