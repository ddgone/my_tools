package echo_tool

import (
	"context"
	"flag"
	"fmt"
	"io"
)

// Run 是工具的唯一入口，由 builder 生成的 wrapper 调用。
func Run(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("echo_tool", flag.ContinueOnError)
	fs.SetOutput(out)

	var input string
	var label string

	fs.StringVar(&input, "input", "", "输入路径")
	fs.StringVar(&label, "label", "echo", "标签")

	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Fprintf(out, "echo_tool started\n")
	fmt.Fprintf(out, "  label: %s\n", label)
	fmt.Fprintf(out, "  input: %s\n", input)
	fmt.Fprintf(out, "  args:  %v\n", fs.Args())

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	fmt.Fprintf(out, "echo_tool finished\n")
	return nil
}
