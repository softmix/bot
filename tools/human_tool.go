package tools

func init() {
	RegisterTool(HumanTool{})
}

type HumanTool struct{}

func (tool HumanTool) Name() string {
	return "Human"
}

func (tool HumanTool) Description() string {
	return "You can ask a human for guidance when you think you got stuck or you are not sure what to do next. The input should be a question for the human."
}

func (tool HumanTool) Run(input string) (string, error) {
	return input, nil
}
