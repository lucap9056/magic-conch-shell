package structs

// Content part types for multimodal prompts
const (
	PartTypeText  = "text"
	PartTypeImage = "image"
)

// NewTextPart creates a new PromptPart containing text.
func NewTextPart(content string) *PromptPart {
	return &PromptPart{
		Type:    PartTypeText,
		Content: content,
	}
}

// NewImagePart creates a new PromptPart containing image data reference.
func NewImagePart(mimeType string, content string) *PromptPart {
	return &PromptPart{
		Type:     PartTypeImage,
		MimeType: mimeType,
		Content:  content,
	}
}
