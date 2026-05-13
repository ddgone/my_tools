# My Tools

个人工作工具合集项目，基于插件化架构的命令行工具箱。

## 特性

- ✅ **插件化架构**：每个工具独立开发，易于扩展
- ✅ **统一接口**：所有工具实现统一的 Tool 接口
- ✅ **跨平台**：纯 Go 实现，支持 Windows/Linux/Mac
- ✅ **简单易用**：清晰的命令行参数和友好的帮助信息
- ✅ **快速开发**：提供工具模板，5分钟创建新工具

## 快速开始

### 编译

```bash
go build -o my_tools.exe    # Windows
go build -o my_tools        # Linux/Mac
```

### 使用

```bash
# 查看所有工具
./my_tools

# 列出工具列表
./my_tools list

# 使用文本工具
echo "hello" | ./my_tools text -upper
```

详细用法请查看 [QUICKSTART.md](QUICKSTART.md)

## 项目结构

```
my_tools/
├── main.go                    # 主程序入口
├── utils/
│   ├── tool/                  # 工具接口定义
│   ├── registry/              # 工具注册表
│   ├── template/              # 新工具模板
│   └── text_tool/             # 示例工具：文本处理
├── config/                    # 配置文件
├── scripts/                   # 脚本文件
└── docs/                      # 文档
```

## 已包含的工具

- **text** - 文本处理工具（大小写转换、反转、去空格等）

## 添加新工具

1. 复制 `utils/template` 目录
2. 修改代码实现你的功能
3. 在 `main.go` 中注册工具
4. 编译运行

详见 [docs/USAGE.md](docs/USAGE.md)

## 技术栈

- Go 1.x
- 标准库（flag, context, io 等）
- 无第三方依赖

## 许可证

MIT
