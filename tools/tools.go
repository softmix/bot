package tools

import "errors"

type Tool interface {
	Name() string
	Description() string
	Run(args string) (string, error)
}

var AvailableTools = make(map[string]Tool)

func RegisterTool(tool Tool) {
	AvailableTools[tool.Name()] = tool
}

func RunToolByName(name, input string) (string, error) {
	tool, ok := AvailableTools[name]
	if !ok {
		return "", errors.New("Tool not found")
	}

	return tool.Run(input)
}
