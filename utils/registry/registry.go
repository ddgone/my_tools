package registry

import (
	"fmt"
	"my_tools/utils/tool"
	"sort"
)

// Registry 工具注册表
type Registry struct {
	tools map[string]tool.Tool
}

// NewRegistry 创建新的工具注册表
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]tool.Tool),
	}
}

// Register 注册一个工具
func (r *Registry) Register(t tool.Tool) error {
	name := t.Name()
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("工具 %s 已存在", name)
	}
	r.tools[name] = t
	return nil
}

// Get 获取指定名称的工具
func (r *Registry) Get(name string) (tool.Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// List 列出所有已注册的工具
func (r *Registry) List() []tool.Tool {
	var tools []tool.Tool
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	
	// 按名称排序
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name() < tools[j].Name()
	})
	
	return tools
}

// Names 获取所有工具名称
func (r *Registry) Names() []string {
	var names []string
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
