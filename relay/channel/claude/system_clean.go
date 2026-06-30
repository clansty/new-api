package claude

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/dto"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// "Today's date" 中的撇号可能被替换成的同形字符；用有序切片保证清洗详情顺序确定
var apostropheHomoglyphs = []struct {
	r     rune
	label string
}{
	{'\u2019', "U+2019"},
	{'\u02BC', "U+02BC"},
	{'\u02B9', "U+02B9"},
}

const cleanedApostropheLabel = "U+0027"

// 匹配 "Today's date [is|:] YYYY/M/D"，撇号容忍 ASCII 与三种同形字符；用于与服务器时钟无关的日期清洗
var todayDateSlashRe = regexp.MustCompile(`Today['\x{2019}\x{02BC}\x{02B9}]s date(?:\s+is|:)?\s+\d{4}/\d{1,2}/\d{1,2}`)
var slashDateRe = regexp.MustCompile(`\d{4}/\d{1,2}/\d{1,2}`)

// cleanText 规范化一段文本：还原 "Today's date" 中的同形撇号、把该短语后的斜杠日期改成短横线。
// useDateHeuristic=true 时额外按服务器日期 ±1 天兜底替换其余斜杠日期；仅用于系统提示词，避免改动用户消息中的合法日期。
func cleanText(text string, now time.Time, useDateHeuristic bool) (string, []dto.SystemPromptCleanDetail) {
	var details []dto.SystemPromptCleanDetail

	for _, variant := range apostropheHomoglyphs {
		needle := "Today" + string(variant.r) + "s date"
		if strings.Contains(text, needle) {
			text = strings.ReplaceAll(text, needle, "Today's date")
			details = append(details, dto.SystemPromptCleanDetail{
				Type: dto.SystemPromptCleanApostrophe,
				From: variant.label,
				To:   cleanedApostropheLabel,
			})
		}
	}

	text = todayDateSlashRe.ReplaceAllStringFunc(text, func(match string) string {
		slash := slashDateRe.FindString(match)
		if slash == "" {
			return match
		}
		dash := strings.ReplaceAll(slash, "/", "-")
		details = append(details, dto.SystemPromptCleanDetail{
			Type: dto.SystemPromptCleanDateFormat,
			From: slash,
			To:   dash,
		})
		return strings.ReplaceAll(match, slash, dash)
	})

	if useDateHeuristic {
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
	}

	return text, details
}

// CleanClaudeRequest 原地清洗请求的 system 与 messages 文本，返回清洗详情（转换路径使用）。
func CleanClaudeRequest(req *dto.ClaudeRequest) []dto.SystemPromptCleanDetail {
	if req == nil {
		return nil
	}
	now := time.Now()
	var details []dto.SystemPromptCleanDetail

	if req.System != nil {
		if req.IsStringSystem() {
			if cleaned, d := cleanText(req.GetStringSystem(), now, true); len(d) > 0 {
				req.SetStringSystem(cleaned)
				details = append(details, d...)
			}
		} else {
			contents := req.ParseSystem()
			modified := false
			for i := range contents {
				if contents[i].Type != dto.ContentTypeText || contents[i].Text == nil {
					continue
				}
				if cleaned, d := cleanText(*contents[i].Text, now, true); len(d) > 0 {
					contents[i].SetText(cleaned)
					details = append(details, d...)
					modified = true
				}
			}
			if modified {
				req.System = contents
			}
		}
	}

	// 指纹注入位于客户端构造的第一条用户消息（Claude Code 的 <env> 块），只清理它，避免改动后续真实用户输入
	for mi := range req.Messages {
		msg := &req.Messages[mi]
		if msg.Role != "user" {
			continue
		}
		if msg.Content == nil {
			break
		}
		if msg.IsStringContent() {
			if cleaned, d := cleanText(msg.GetStringContent(), now, false); len(d) > 0 {
				msg.SetStringContent(cleaned)
				details = append(details, d...)
			}
			break
		}
		blocks, err := msg.ParseContent()
		if err != nil {
			break
		}
		modified := false
		for bi := range blocks {
			if blocks[bi].Type != dto.ContentTypeText || blocks[bi].Text == nil {
				continue
			}
			if cleaned, d := cleanText(*blocks[bi].Text, now, false); len(d) > 0 {
				blocks[bi].SetText(cleaned)
				details = append(details, d...)
				modified = true
			}
		}
		if modified {
			msg.SetContent(blocks)
		}
		break
	}

	return details
}

// CleanClaudeRequestBody 对直通模式的原始请求体做字节级清洗，仅改写 system 与 messages 的文本、保留其余字节。
func CleanClaudeRequestBody(body []byte) ([]byte, []dto.SystemPromptCleanDetail) {
	now := time.Now()
	var details []dto.SystemPromptCleanDetail

	body, details = cleanBodyTextField(body, "system", now, true, details)

	msgs := gjson.GetBytes(body, "messages")
	if msgs.IsArray() {
		for mi, m := range msgs.Array() {
			if m.Get("role").String() != "user" {
				continue
			}
			body, details = cleanBodyTextField(body, "messages."+strconv.Itoa(mi)+".content", now, false, details)
			break
		}
	}

	return body, details
}

// cleanBodyTextField 清洗 body 中某路径的文本：取值可能是字符串，或 [{type:text,text}] 内容块数组。
func cleanBodyTextField(body []byte, path string, now time.Time, useDateHeuristic bool, details []dto.SystemPromptCleanDetail) ([]byte, []dto.SystemPromptCleanDetail) {
	field := gjson.GetBytes(body, path)
	if !field.Exists() {
		return body, details
	}
	if field.Type == gjson.String {
		if cleaned, d := cleanText(field.String(), now, useDateHeuristic); len(d) > 0 {
			if nb, err := sjson.SetBytes(body, path, cleaned); err == nil {
				body = nb
			}
			details = append(details, d...)
		}
		return body, details
	}
	if field.IsArray() {
		for i, item := range field.Array() {
			if item.Get("type").String() != dto.ContentTypeText || !item.Get("text").Exists() {
				continue
			}
			if cleaned, d := cleanText(item.Get("text").String(), now, useDateHeuristic); len(d) > 0 {
				if nb, err := sjson.SetBytes(body, path+"."+strconv.Itoa(i)+".text", cleaned); err == nil {
					body = nb
				}
				details = append(details, d...)
			}
		}
	}
	return body, details
}
