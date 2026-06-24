package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

type input struct {
	Name     string
	ID       string
	Kind     string
	Category string
	Entry    string
}

func main() {
	var in input
	flag.StringVar(&in.Name, "name", "", "工具名称")
	flag.StringVar(&in.ID, "id", "", "工具 ID")
	flag.StringVar(&in.Kind, "kind", "go", "工具类型: go/rust")
	flag.StringVar(&in.Category, "category", "未分类", "分类路径，用 > 分隔")
	flag.Parse()

	if in.Name == "" || in.ID == "" {
		fmt.Println("用法: go run scripts/new-tool -name \"工具名\" -id tool_id -kind go -category \"分类>子类\"")
		os.Exit(1)
	}

	rootDir, _ := os.Getwd()

	tmplDir := filepath.Join(rootDir, "scripts", "new-tool", "templates")
	if _, err := os.Stat(tmplDir); os.IsNotExist(err) {
		tmplDir = "" // use embedded
	}

	switch in.Kind {
	case "go":
		generateGo(in, rootDir)
	case "rust":
		generateRust(in, rootDir)
	default:
		fmt.Printf("不支持的类型: %s\n", in.Kind)
		os.Exit(1)
	}
}

func generateGo(in input, rootDir string) {
	toolDir := filepath.Join(rootDir, "tools", "go_tools", in.ID)
	if err := os.MkdirAll(toolDir, 0755); err != nil {
		fmt.Printf("创建工具目录失败: %v\n", err)
		os.Exit(1)
	}

	toolGo := filepath.Join(toolDir, "tool.go")
	toolTest := filepath.Join(toolDir, "tool_test.go")

	if err := writeTMpl(toolGo, goToolTmpl, in); err != nil {
		fmt.Printf("写入 tool.go 失败: %v\n", err)
		os.Exit(1)
	}
	if err := writeTMpl(toolTest, goTestTmpl, in); err != nil {
		fmt.Printf("写入 tool_test.go 失败: %v\n", err)
		os.Exit(1)
	}

	manifestDir := filepath.Join(rootDir, "libs", "catalog", "builtin", "manifests")
	os.MkdirAll(manifestDir, 0755)
	manifestPath := filepath.Join(manifestDir, in.ID+".yaml")
	if err := writeTMpl(manifestPath, goManifestTmpl, in); err != nil {
		fmt.Printf("写入 manifest 失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Go 工具已生成: %s\n", toolDir)
	fmt.Printf("   清单: %s\n", manifestPath)
}

func generateRust(in input, rootDir string) {
	crateDir := filepath.Join(rootDir, "tools", "rust_tools", in.ID)
	srcDir := filepath.Join(crateDir, "src")

	for _, d := range []string{srcDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			fmt.Printf("创建目录失败: %v\n", err)
			os.Exit(1)
		}
	}

	cargoToml := filepath.Join(crateDir, "Cargo.toml")
	libRs := filepath.Join(srcDir, "lib.rs")

	if err := writeTMpl(cargoToml, rustCargoTmpl, in); err != nil {
		fmt.Printf("写入 Cargo.toml 失败: %v\n", err)
		os.Exit(1)
	}
	if err := writeTMpl(libRs, rustLibTmpl, in); err != nil {
		fmt.Printf("写入 lib.rs 失败: %v\n", err)
		os.Exit(1)
	}

	manifestDir := filepath.Join(rootDir, "libs", "catalog", "builtin", "manifests")
	os.MkdirAll(manifestDir, 0755)
	manifestPath := filepath.Join(manifestDir, in.ID+".yaml")
	if err := writeTMpl(manifestPath, rustManifestTmpl, in); err != nil {
		fmt.Printf("写入 manifest 失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Rust 工具已生成: %s\n", crateDir)
	fmt.Printf("   清单: %s\n", manifestPath)
}

func writeTMpl(path string, tmplStr string, in input) error {
	if _, statErr := os.Stat(path); statErr == nil {
		fmt.Printf("⚠ 文件已存在，跳过: %s\n", path)
		return nil
	}

	tmpl, err := template.New("").Parse(tmplStr)
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, in)
}

var goToolTmpl = `package {{.ID}}

import (
	"context"
	"flag"
	"fmt"
	"io"
)

// Run 是工具的唯一入口，由 builder 生成的 wrapper 调用。
func Run(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("{{.ID}}", flag.ContinueOnError)
	fs.SetOutput(out)

	var input string
	var output string

	fs.StringVar(&input, "input", "", "输入路径")
	fs.StringVar(&output, "output", "", "输出路径")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if input == "" {
		return fmt.Errorf("必须指定 -input 参数")
	}

	fmt.Fprintf(out, "{{.Name}}: input=%s, output=%s\n", input, output)
	return nil
}
`

var goTestTmpl = `package {{.ID}}

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestShouldPrintArgsWhenRunNormally(t *testing.T) {
	var buf bytes.Buffer
	err := Run(context.Background(), []string{"-input", "/tmp/data"}, &buf)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "input: /tmp/data") {
		t.Fatalf("expected output to contain input, got: %s", output)
	}
}
`

var goManifestTmpl = `id: {{.ID}}
name: {{.Name}}
kind: go
category: {{.Category}}
icon: go
description: 一句话说明工具用途。
docs:
  summary: 用于列表页和详情区的短说明。
  usage: |
    桌面宿主里的使用说明。
params:
  - key: input
    argKey: input
    type: path
    label: 输入路径
    required: true
  - key: output
    argKey: output
    type: path
    label: 输出路径
execution:
  local:
    adapter: go-binary
  remote:
    strategy: upload-binary-and-run
export:
  strategy: export-binary
source:
  entry: tools/go_tools/{{.ID}}/tool.go
`

var rustCargoTmpl = `[package]
name = "{{.ID}}"
version = "0.1.0"
edition = "2024"

[lib]
name = "{{.ID}}"
path = "src/lib.rs"

[dependencies]
`

var rustLibTmpl = `/// 工具的唯一入口，由 builder 生成的 wrapper crate 调用。
pub fn run(args: &[String]) -> Result<(), Box<dyn std::error::Error>> {
    let mut input = String::new();
    let mut output = String::new();

    let mut i = 0;
    while i < args.len() {
        match args[i].as_str() {
            "--input" if i + 1 < args.len() => { input = args[i + 1].clone(); i += 2; }
            "--output" if i + 1 < args.len() => { output = args[i + 1].clone(); i += 2; }
            _ => i += 1,
        }
    }

    if input.is_empty() {
        return Err("必须指定 --input 参数".into());
    }

    println!("{{.Name}}: input={}, output={}", input, output);
    Ok(())
}
`

var rustManifestTmpl = `id: {{.ID}}
name: {{.Name}}
kind: rust
category: {{.Category}}
icon: rust
description: 一句话说明工具用途。
docs:
  summary: 简短摘要。
  usage: |
    使用说明。
params:
  - key: input
    argKey: input
    type: path
    label: 输入路径
    required: true
  - key: output
    argKey: output
    type: path
    label: 输出路径
execution:
  local:
    adapter: rust-binary
  remote:
    strategy: upload-binary-and-run
export:
  strategy: export-binary
source:
  entry: tools/rust_tools/{{.ID}}/src/lib.rs
`
