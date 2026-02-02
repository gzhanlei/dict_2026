package util

import (
	"strings"
)

// 检测字符串是否包含日文字符
func IsJapanese(text string) bool {
	// 简单检测：检查是否包含假名字符
	for _, r := range text {
		if (r >= 0x3040 && r <= 0x309F) || // 平假名
			(r >= 0x30A0 && r <= 0x30FF) || // 片假名
			(r >= 0x4E00 && r <= 0x9FAF) { // 汉字
			return true
		}
	}
	return false
}

// 假名转罗马字映射表
var hiraganaToRomaji = map[string]string{
	// 基本假名
	"あ": "a", "い": "i", "う": "u", "え": "e", "お": "o",
	"か": "ka", "き": "ki", "く": "ku", "け": "ke", "こ": "ko",
	"が": "ga", "ぎ": "gi", "ぐ": "gu", "げ": "ge", "ご": "go",
	"さ": "sa", "し": "shi", "す": "su", "せ": "se", "そ": "so",
	"ざ": "za", "じ": "ji", "ず": "zu", "ぜ": "ze", "ぞ": "zo",
	"た": "ta", "ち": "chi", "つ": "tsu", "て": "te", "と": "to",
	"だ": "da", "ぢ": "di", "づ": "du", "で": "de", "ど": "do",
	"な": "na", "に": "ni", "ぬ": "nu", "ね": "ne", "の": "no",
	"は": "ha", "ひ": "hi", "ふ": "fu", "へ": "he", "ほ": "ho",
	"ば": "ba", "び": "bi", "ぶ": "bu", "べ": "be", "ぼ": "bo",
	"ぱ": "pa", "ぴ": "pi", "ぷ": "pu", "ぺ": "pe", "ぽ": "po",
	"ま": "ma", "み": "mi", "む": "mu", "め": "me", "も": "mo",
	"や": "ya", "ゆ": "yu", "よ": "yo",
	"ら": "ra", "り": "ri", "る": "ru", "れ": "re", "ろ": "ro",
	"わ": "wa", "ゐ": "wi", "ゑ": "we", "を": "wo", "ん": "n",
	"ー": "-",
}

// 拗音映射表
var youonMap = map[string]string{
	"きゃ": "kya", "きゅ": "kyu", "きょ": "kyo",
	"しゃ": "sha", "しゅ": "shu", "しょ": "sho",
	"ちゃ": "cha", "ちゅ": "chu", "ちょ": "cho",
	"にゃ": "nya", "にゅ": "nyu", "にょ": "nyo",
	"ひゃ": "hya", "ひゅ": "hyu", "ひょ": "hyo",
	"みゃ": "mya", "みゅ": "myu", "みょ": "myo",
	"りゃ": "rya", "りゅ": "ryu", "りょ": "ryo",
	"ぎゃ": "gya", "ぎゅ": "gyu", "ぎょ": "gyo",
	"じゃ": "ja", "じゅ": "ju", "じょ": "jo",
	"びゃ": "bya", "びゅ": "byu", "びょ": "byo",
	"ぴゃ": "pya", "ぴゅ": "pyu", "ぴょ": "pyo",
}

// 片假名到平假名的映射
var katakanaToHiragana = map[string]string{
	"ア": "あ", "イ": "い", "ウ": "う", "エ": "え", "オ": "お",
	"カ": "か", "キ": "き", "ク": "く", "ケ": "け", "コ": "こ",
	"ガ": "が", "ギ": "ぎ", "グ": "ぐ", "ゲ": "げ", "ゴ": "ご",
	"サ": "さ", "シ": "し", "ス": "す", "セ": "せ", "ソ": "そ",
	"ザ": "ざ", "ジ": "じ", "ズ": "ず", "ゼ": "ぜ", "ゾ": "ぞ",
	"タ": "た", "チ": "ち", "ツ": "つ", "テ": "て", "ト": "と",
	"ダ": "だ", "ヂ": "ぢ", "ヅ": "づ", "デ": "で", "ド": "ど",
	"ナ": "な", "ニ": "に", "ヌ": "ぬ", "ネ": "ね", "ノ": "の",
	"ハ": "は", "ヒ": "ひ", "フ": "ふ", "ヘ": "へ", "ホ": "ほ",
	"バ": "ば", "ビ": "び", "ブ": "ぶ", "ベ": "べ", "ボ": "ぼ",
	"パ": "ぱ", "ピ": "ぴ", "プ": "ぷ", "ペ": "ぺ", "ポ": "ぽ",
	"マ": "ま", "ミ": "み", "ム": "む", "メ": "め", "モ": "も",
	"ヤ": "や", "ユ": "ゆ", "ヨ": "よ",
	"ラ": "ら", "リ": "り", "ル": "る", "レ": "れ", "ロ": "ろ",
	"ワ": "わ", "ヰ": "ゐ", "ヱ": "ゑ", "ヲ": "を", "ン": "ン",
	"ー": "ー",
	"ャ": "ゃ", "ュ": "ゅ", "ョ": "ョ", "ッ": "っ",
}

// 将片假名转换为平假名
func katakanaToHiraganaFunc(text string) string {
	result := ""
	for _, r := range text {
		if h, ok := katakanaToHiragana[string(r)]; ok {
			result += h
		} else {
			result += string(r)
		}
	}
	return result
}

// 处理促音（小っ）
func processSokuon(text string) string {
	runes := []rune(text)
	result := ""
	for i, r := range runes {
		if r == 'っ' {
			// 如果促音后面跟着一个辅音，双写该辅音
			if i+1 < len(runes) {
				nextRune := runes[i+1]
				nextRomaji := ""
				if h, ok := hiraganaToRomaji[string(nextRune)]; ok {
					nextRomaji = h
					// 取第一个字符（辅音部分）
					if len(nextRomaji) > 0 {
						result += string(nextRomaji[0])
					}
				} else {
					// 检查是否是拗音的开始
					if i+2 < len(runes) {
						youonStr := string(runes[i+1:i+3])
						if y, ok := youonMap[youonStr]; ok {
							if len(y) > 0 {
								result += string(y[0])
							}
						}
					}
				}
			}
		} else {
			result += string(r)
		}
	}
	return result
}

// 处理长音
func processChouon(text string) string {
	// 简单处理：う段的长音用u，お段的长音用o
	replacements := map[string]string{
		"おう": "ou", "こう": "kou", "そう": "sou", "とう": "tou",
		"のう": "nou", "ほう": "hou", "もう": "mou", "よう": "you", "ろう": "rou",
		"おお": "oo",
	}
	
	for old, new := range replacements {
		text = strings.ReplaceAll(text, old, new)
	}
	
	return text
}

// 将日语假名转换为罗马字
func ToRomaji(text string) string {
	if !IsJapanese(text) {
		return text
	}

	// 先将片假名转为平假名
	text = katakanaToHiraganaFunc(text)

	// 处理拗音
	for youon, romaji := range youonMap {
		text = strings.ReplaceAll(text, youon, romaji)
	}

	// 处理促音
	text = processSokuon(text)

	// 处理基本假名
	for hiragana, romaji := range hiraganaToRomaji {
		text = strings.ReplaceAll(text, hiragana, romaji)
	}

	// 处理长音
	text = processChouon(text)

	return text
}