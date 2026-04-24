package render

import "fmt"

type Attachment struct {
	Name      string
	MimeType  string
	SizeBytes int
}

func FormatAttachments(attachments []Attachment) []string {
	if len(attachments) == 0 {
		return nil
	}

	lines := make([]string, 0, len(attachments)+1)
	lines = append(lines, fmt.Sprintf("-- %d attachments --", len(attachments)))
	for _, attachment := range attachments {
		name := attachment.Name
		if name == "" {
			name = "unnamed"
		}
		detail := humanBytes(attachment.SizeBytes)
		if attachment.MimeType != "" {
			detail = attachment.MimeType + ", " + detail
		}
		lines = append(lines, fmt.Sprintf("- %s (%s)", name, detail))
	}
	return lines
}

func humanBytes(size int) string {
	if size < 0 {
		size = 0
	}
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	value := float64(size)
	units := []string{"KB", "MB", "GB"}
	for _, suffix := range units {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f TB", value/unit)
}
