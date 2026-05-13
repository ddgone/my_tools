package text

import (
	"fmt"
	"strings"

	"my_tools/pkg/framework"
)

type CaseTool struct{}

func (t *CaseTool) ID() string       { return "text_case" }
func (t *CaseTool) Name() string     { return "大小写转换" }
func (t *CaseTool) Category() string { return "文本处理" }

func (t *CaseTool) Execute(ctx framework.AppContext) {
	ctx.PromptInput("大小写转换", "请输入文本:", "", func(text string) {
		if text == "" {
			return
		}
		ctx.PromptChoice("选择转换模式", "模式:", []string{"转换为大写", "转换为小写"}, func(choice string) {
			var result string
			if choice == "转换为大写" {
				result = strings.ToUpper(text)
			} else {
				result = strings.ToLower(text)
			}
			ctx.ShowModal("转换结果", result)
			ctx.RecordUsage(map[string]string{"text": text, "mode": choice})
		})
	})
}

type ReverseTool struct{}

func (t *ReverseTool) ID() string       { return "text_reverse" }
func (t *ReverseTool) Name() string     { return "文本反转" }
func (t *ReverseTool) Category() string { return "文本处理" }

func (t *ReverseTool) Execute(ctx framework.AppContext) {
	ctx.PromptInput("文本反转", "请输入文本:", "", func(text string) {
		if text == "" {
			return
		}
		runes := []rune(text)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		ctx.ShowModal("反转结果", string(runes))
		ctx.RecordUsage(map[string]string{"text": text})
	})
}

type TrimTool struct{}

func (t *TrimTool) ID() string       { return "text_trim" }
func (t *TrimTool) Name() string     { return "去除首尾空格" }
func (t *TrimTool) Category() string { return "文本处理" }

func (t *TrimTool) Execute(ctx framework.AppContext) {
	ctx.PromptInput("去除首尾空格", "请输入文本:", "", func(text string) {
		if text == "" {
			return
		}
		result := strings.TrimSpace(text)
		ctx.ShowModal("处理结果", fmt.Sprintf("'%s'", result))
		ctx.RecordUsage(map[string]string{"text": text})
	})
}

func init() {
	framework.Register(&CaseTool{})
	framework.Register(&ReverseTool{})
	framework.Register(&TrimTool{})
}
