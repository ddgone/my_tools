package toolspec

import (
	"encoding/json"
	"strings"

	"gopkg.in/yaml.v3"
)

type CategoryPath []string

func (c *CategoryPath) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err == nil {
		parts := strings.Split(s, ">")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		*c = result
		return nil
	}
	var arr []string
	if err := value.Decode(&arr); err != nil {
		return err
	}
	*c = arr
	return nil
}

func (c CategoryPath) MarshalJSON() ([]byte, error) {
	if c == nil {
		return json.Marshal([]string{})
	}
	return json.Marshal([]string(c))
}

type ToolKind string

const (
	ToolKindGo     ToolKind = "go"
	ToolKindPython ToolKind = "python"
	ToolKindRust   ToolKind = "rust"
)

type ParameterFieldType string

const (
	FieldTypeText     ParameterFieldType = "text"
	FieldTypeTextarea ParameterFieldType = "textarea"
	FieldTypeNumber   ParameterFieldType = "number"
	FieldTypeBoolean  ParameterFieldType = "boolean"
	FieldTypePath     ParameterFieldType = "path"
	FieldTypeSelect   ParameterFieldType = "select"
)

type ExecutionAdapter string

const (
	ExecutionAdapterGoBinary     ExecutionAdapter = "go-binary"
	ExecutionAdapterPythonScript ExecutionAdapter = "python-script"
	ExecutionAdapterRustBinary   ExecutionAdapter = "rust-binary"
)

type RemoteStrategy string

const (
	RemoteStrategyUploadBinary RemoteStrategy = "upload-binary-and-run"
	RemoteStrategyUploadScript RemoteStrategy = "upload-script-and-run"
)

type ExportStrategy string

const (
	ExportStrategyBinary ExportStrategy = "export-binary"
	ExportStrategyScript ExportStrategy = "export-script"
)

type ToolDocs struct {
	Summary string `yaml:"summary" json:"summary"`
	Usage   string `yaml:"usage" json:"usage"`
}

type ParameterOption struct {
	Label string `yaml:"label" json:"label"`
	Value string `yaml:"value" json:"value"`
}

type ParameterCondition struct {
	Key    string `yaml:"key" json:"key"`
	Equals any    `yaml:"equals,omitempty" json:"equals,omitempty"`
}

type ParameterVisibility struct {
	All []ParameterCondition `yaml:"all,omitempty" json:"all,omitempty"`
	Any []ParameterCondition `yaml:"any,omitempty" json:"any,omitempty"`
}

type ParameterSpec struct {
	Key         string               `yaml:"key" json:"key"`
	ArgKey      string               `yaml:"argKey,omitempty" json:"argKey,omitempty"`
	Type        ParameterFieldType   `yaml:"type" json:"type"`
	Label       string               `yaml:"label" json:"label"`
	Placeholder string               `yaml:"placeholder,omitempty" json:"placeholder,omitempty"`
	Required    bool                 `yaml:"required,omitempty" json:"required,omitempty"`
	Default     any                  `yaml:"default,omitempty" json:"default,omitempty"`
	Help        string               `yaml:"help,omitempty" json:"help,omitempty"`
	Options     []ParameterOption    `yaml:"options,omitempty" json:"options,omitempty"`
	Group       string               `yaml:"group,omitempty" json:"group,omitempty"`
	PathMode    string               `yaml:"pathMode,omitempty" json:"pathMode,omitempty"`
	Emit        *bool                `yaml:"emit,omitempty" json:"emit,omitempty"`
	VisibleWhen *ParameterVisibility `yaml:"visibleWhen,omitempty" json:"visibleWhen,omitempty"`
}

type LocalExecutionSpec struct {
	Adapter ExecutionAdapter `yaml:"adapter" json:"adapter"`
}

type RemoteExecutionSpec struct {
	Strategy RemoteStrategy `yaml:"strategy" json:"strategy"`
}

type ExecutionSpec struct {
	Local  LocalExecutionSpec  `yaml:"local" json:"local"`
	Remote RemoteExecutionSpec `yaml:"remote" json:"remote"`
}

type ExportSpec struct {
	Strategy ExportStrategy `yaml:"strategy" json:"strategy"`
}

type SourceSpec struct {
	Entry string `yaml:"entry" json:"entry"`
}

type ToolManifest struct {
	ID          string          `yaml:"id" json:"id"`
	Name        string          `yaml:"name" json:"name"`
	Kind        ToolKind        `yaml:"kind" json:"kind"`
	Category    CategoryPath    `yaml:"category" json:"category"`
	Icon        string          `yaml:"icon" json:"icon"`
	Description string          `yaml:"description" json:"description"`
	Docs        ToolDocs        `yaml:"docs" json:"docs"`
	Params      []ParameterSpec `yaml:"params" json:"params"`
	Execution   ExecutionSpec   `yaml:"execution" json:"execution"`
	Export      ExportSpec      `yaml:"export" json:"export"`
	Source      SourceSpec      `yaml:"source" json:"source"`
}
