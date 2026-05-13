package text_tool

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"my_tools/utils/tool"
)

// TextTool 文本处理工具示例
type TextTool struct {
	*tool.BaseTool
	
	// 配置参数
	upperCase   bool
	lowerCase   bool
	reverse     bool
	trimSpace   bool
	inputFile   string
	outputFile  string
}

// NewTextTool 创建文本处理工具
func NewTextTool() *TextTool {
	return &TextTool{
		BaseTool: tool.NewBaseTool("text", "文本处理工具：大小写转换、反转、去空格等"),
	}
}

// RegisterFlags 注册命令行参数
func (t *TextTool) RegisterFlags() {
	// 注意：实际使用时会在 Execute 中创建新的 FlagSet
	// 这里只是为了展示参数
}

// registerFlagsToSet 将参数注册到指定的 FlagSet
func (t *TextTool) registerFlagsToSet(fs *flag.FlagSet) {
	fs.BoolVar(&t.upperCase, "upper", false, "转换为大写")
	fs.BoolVar(&t.lowerCase, "lower", false, "转换为小写")
	fs.BoolVar(&t.reverse, "reverse", false, "反转文本")
	fs.BoolVar(&t.trimSpace, "trim", false, "去除首尾空格")
	fs.StringVar(&t.inputFile, "input", "", "输入文件路径（不指定则从stdin读取）")
	fs.StringVar(&t.outputFile, "output", "", "输出文件路径（不指定则输出到stdout）")
}

// Execute 执行文本处理
func (t *TextTool) Execute(ctx context.Context, args []string) error {
	// 解析参数
	fs := flag.NewFlagSet(t.Name(), flag.ExitOnError)
	t.registerFlagsToSet(fs)
	
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("解析参数失败: %w", err)
	}
	
	// 验证参数
	if t.upperCase && t.lowerCase {
		return fmt.Errorf("不能同时使用 -upper 和 -lower")
	}
	
	// 读取输入
	input, err := t.readInput()
	if err != nil {
		return fmt.Errorf("读取输入失败: %w", err)
	}
	
	// 处理文本
	result := t.processText(input)
	
	// 写入输出
	if err := t.writeOutput(result); err != nil {
		return fmt.Errorf("写入输出失败: %w", err)
	}
	
	return nil
}

// readInput 读取输入文本
func (t *TextTool) readInput() (string, error) {
	var reader io.Reader
	
	if t.inputFile != "" {
		file, err := os.Open(t.inputFile)
		if err != nil {
			return "", err
		}
		defer file.Close()
		reader = file
	} else {
		reader = os.Stdin
	}
	
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	
	return string(data), nil
}

// processText 处理文本
func (t *TextTool) processText(text string) string {
	result := text
	
	if t.trimSpace {
		result = strings.TrimSpace(result)
	}
	
	if t.upperCase {
		result = strings.ToUpper(result)
	}
	
	if t.lowerCase {
		result = strings.ToLower(result)
	}
	
	if t.reverse {
		runes := []rune(result)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		result = string(runes)
	}
	
	return result
}

// writeOutput 写入输出
func (t *TextTool) writeOutput(text string) error {
	var writer io.Writer
	
	if t.outputFile != "" {
		file, err := os.Create(t.outputFile)
		if err != nil {
			return err
		}
		defer file.Close()
		writer = file
	} else {
		writer = os.Stdout
	}
	
	_, err := writer.Write([]byte(text))
	return err
}
