package models

type ChatCompletionRequest struct {
	Model              string
	Content            Content
	SystemInstruction  Content
	Temperature        float32
	JsonSchemaResponse string
	TemplateId         string
}

type ChatCompletionResponse struct {
	Content Content
}

type Content struct {
	Role string
	Text string
}
