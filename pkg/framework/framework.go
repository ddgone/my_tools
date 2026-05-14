package framework

import "io"

type AppContext interface {
	ShowModal(title, message string)
	PromptInput(title, prompt, defaultValue string, callback func(string))
	PromptChoice(title, prompt string, options []string, callback func(string))
	ShowTerminal(title string, usage string, run func(args string, out io.Writer) error)
	ShowPythonTerminal(title string, usage string, run func(env string, args string, out io.Writer) error)
	GetLastParam(key string) string
	RecordUsage(params map[string]string)
}

type Tool interface {
	ID() string
	Name() string
	Category() string
	Execute(ctx AppContext)
}

var Registry = make([]Tool, 0)

func Register(t Tool) {
	Registry = append(Registry, t)
}
