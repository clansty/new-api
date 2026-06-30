package claude

import (
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/dto"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// "Today's date is" 中的撇号可能被替换成的同形字符；用有序切片保证清洗详情顺序确定
var apostropheHomoglyphs = []struct {
	r     rune
	label string
}{
	{'\u2019', "U+2019"},
	{'\u02BC', "U+02BC"},
	{'\u02B9', "U+02B9"},
}

const cleanedApostropheLabel = "U+0027"

// cleanSystemText 规范化单段系统提示词文本，返回清洗后的文本与清洗详情。
// now 由调用方注入以便测试；日期清洗以 now 当天 ±1 天为候选。
func cleanSystemText(text string, now time.Time) (string, []dto.SystemPromptCleanDetail) {
	var details []dto.SystemPromptCleanDetail

	// 仅针对 "Today's date is" 这一已知短语替换撇号，避免误伤正文里合法的弯引号
	for _, variant := range apostropheHomoglyphs {
		needle := "Today" + string(variant.r) + "s date is"
		if strings.Contains(text, needle) {
			text = strings.ReplaceAll(text, needle, "Today's date is")
			details = append(details, dto.SystemPromptCleanDetail{
				Type: dto.SystemPromptCleanApostrophe,
				From: variant.label,
				To:   cleanedApostropheLabel,
			})
		}
	}

	// 按服务器日期 ±1 天构造斜杠格式候选串，命中则改回短横线格式
	for offset := -1; offset <= 1; offset++ {
		day := now.AddDate(0, 0, offset)
		slash := day.Format("2006/01/02")
		dash := day.Format("2006-01-02")
		if strings.Contains(text, slash) {
			text = strings.ReplaceAll(text, slash, dash)
			details = append(details, dto.SystemPromptCleanDetail{
				Type: dto.SystemPromptCleanDateFormat,
				From: slash,
				To:   dash,
			})
		}
	}

	return text, details
}

// CleanClaudeSystemPrompt 原地清洗请求体的 system 字段（字符串或内容块两种形态），返回清洗详情。
// 用于转换路径：清洗后的结构体会被序列化发往上游。
func CleanClaudeSystemPrompt(req *dto.ClaudeRequest) []dto.SystemPromptCleanDetail {
	if req == nil || req.System == nil {
		return nil
	}
	now := time.Now()
	if req.IsStringSystem() {
		cleaned, details := cleanSystemText(req.GetStringSystem(), now)
		if len(details) > 0 {
			req.SetStringSystem(cleaned)
		}
		return details
	}
	contents := req.ParseSystem()
	var allDetails []dto.SystemPromptCleanDetail
	modified := false
	for i := range contents {
		if contents[i].Type != dto.ContentTypeText || contents[i].Text == nil {
			continue
		}
		cleaned, details := cleanSystemText(*contents[i].Text, now)
		if len(details) > 0 {
			contents[i].SetText(cleaned)
			allDetails = append(allDetails, details...)
			modified = true
		}
	}
	if modified {
		req.System = contents
	}
	return allDetails
}

// CleanClaudeSystemPromptBody 对直通模式的原始请求体做字节级清洗，仅改写 system 字段文本、保留其余字节。
func CleanClaudeSystemPromptBody(body []byte) ([]byte, []dto.SystemPromptCleanDetail) {
	sys := gjson.GetBytes(body, "system")
	if !sys.Exists() {
		return body, nil
	}
	now := time.Now()
	if sys.Type == gjson.String {
		cleaned, details := cleanSystemText(sys.String(), now)
		if len(details) > 0 {
			if newBody, err := sjson.SetBytes(body, "system", cleaned); err == nil {
				body = newBody
			}
		}
		return body, details
	}
	if sys.IsArray() {
		var allDetails []dto.SystemPromptCleanDetail
		for i, item := range sys.Array() {
			if item.Get("type").String() != dto.ContentTypeText || !item.Get("text").Exists() {
				continue
			}
			cleaned, details := cleanSystemText(item.Get("text").String(), now)
			if len(details) > 0 {
				if newBody, err := sjson.SetBytes(body, "system."+strconv.Itoa(i)+".text", cleaned); err == nil {
					body = newBody
				}
				allDetails = append(allDetails, details...)
			}
		}
		return body, allDetails
	}
	return body, nil
}
