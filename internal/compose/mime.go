package compose

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"strings"
)

const (
	mixedBoundary       = "gandt-mixed-boundary"
	alternativeBoundary = "gandt-alternative-boundary"
)

func AssembleMIME(draft Draft) ([]byte, error) {
	return assembleMIME(draft, true)
}

func AssembleDraftMIME(draft Draft) ([]byte, error) {
	return assembleMIME(draft, false)
}

func assembleMIME(draft Draft, requireRecipient bool) ([]byte, error) {
	if requireRecipient {
		if err := draft.ValidateForSend(); err != nil {
			return nil, err
		}
	} else if err := draft.ValidateDraft(); err != nil {
		return nil, err
	}
	var body bytes.Buffer
	writeMessageHeaders(&body, draft.Headers)
	if len(draft.Attachments) > 0 {
		writeHeader(&body, "MIME-Version", "1.0")
		writeHeader(&body, "Content-Type", fmt.Sprintf("multipart/mixed; boundary=%q", mixedBoundary))
		body.WriteString("\r\n")
		writeBoundary(&body, mixedBoundary)
		if err := writeBodyPart(&body, draft.Body); err != nil {
			return nil, err
		}
		for _, attachment := range draft.Attachments {
			writeBoundary(&body, mixedBoundary)
			writeAttachmentPart(&body, attachment)
		}
		writeClosingBoundary(&body, mixedBoundary)
		return body.Bytes(), nil
	}

	if err := writeBodyPart(&body, draft.Body); err != nil {
		return nil, err
	}
	return body.Bytes(), nil
}

func writeMessageHeaders(body *bytes.Buffer, headers Headers) {
	writeHeader(body, "From", headers.SendAs.String())
	writeHeader(body, "To", formatAddressList(headers.To))
	if len(headers.Cc) > 0 {
		writeHeader(body, "Cc", formatAddressList(headers.Cc))
	}
	if len(headers.Bcc) > 0 {
		writeHeader(body, "Bcc", formatAddressList(headers.Bcc))
	}
	if strings.TrimSpace(headers.Subject) != "" {
		writeHeader(body, "Subject", encodeHeader(headers.Subject))
	}
}

func writeBodyPart(body *bytes.Buffer, source BodySource) error {
	if strings.TrimSpace(source.HTML) != "" {
		writeHeader(body, "MIME-Version", "1.0")
		writeHeader(body, "Content-Type", fmt.Sprintf("multipart/alternative; boundary=%q", alternativeBoundary))
		body.WriteString("\r\n")
		writeBoundary(body, alternativeBoundary)
		writeTextPart(body, "text/plain", source.PlainText)
		writeBoundary(body, alternativeBoundary)
		writeTextPart(body, "text/html", source.HTML)
		writeClosingBoundary(body, alternativeBoundary)
		return nil
	}
	writeTextPart(body, "text/plain", source.PlainText)
	return nil
}

func writeTextPart(body *bytes.Buffer, contentType string, text string) {
	writeHeader(body, "Content-Type", contentType+"; charset=utf-8")
	if isASCII(text) {
		writeHeader(body, "Content-Transfer-Encoding", "7bit")
	} else {
		writeHeader(body, "Content-Transfer-Encoding", "quoted-printable")
	}
	body.WriteString("\r\n")
	body.WriteString(encodeTextBody(text))
	body.WriteString("\r\n")
}

func writeAttachmentPart(body *bytes.Buffer, attachment Attachment) {
	contentType := strings.TrimSpace(attachment.MimeType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	name := mime.QEncoding.Encode("utf-8", attachment.Filename)
	writeHeader(body, "Content-Type", fmt.Sprintf("%s; name=%q", contentType, name))
	writeHeader(body, "Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	writeHeader(body, "Content-Transfer-Encoding", "base64")
	if attachment.ContentID != "" {
		writeHeader(body, "Content-ID", "<"+attachment.ContentID+">")
	}
	body.WriteString("\r\n")
	body.WriteString(wrapBase64(attachment.Data))
	body.WriteString("\r\n")
}

func writeHeader(body *bytes.Buffer, key string, value string) {
	value = cleanHeaderValue(value)
	if strings.TrimSpace(value) == "" {
		return
	}
	body.WriteString(key)
	body.WriteString(": ")
	body.WriteString(value)
	body.WriteString("\r\n")
}

func writeBoundary(body *bytes.Buffer, boundary string) {
	body.WriteString("--")
	body.WriteString(boundary)
	body.WriteString("\r\n")
}

func writeClosingBoundary(body *bytes.Buffer, boundary string) {
	body.WriteString("--")
	body.WriteString(boundary)
	body.WriteString("--\r\n")
}

func encodeHeader(value string) string {
	if isASCII(value) {
		return value
	}
	return mime.QEncoding.Encode("utf-8", value)
}

func normalizeCRLF(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.ReplaceAll(value, "\n", "\r\n")
}

func encodeTextBody(value string) string {
	value = normalizeCRLF(value)
	if isASCII(value) {
		return value
	}
	var out bytes.Buffer
	writer := quotedprintable.NewWriter(&out)
	_, _ = writer.Write([]byte(value))
	_ = writer.Close()
	return out.String()
}

func cleanHeaderValue(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.Join(strings.Fields(value), " ")
}

func wrapBase64(data []byte) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	if len(encoded) <= 76 {
		return encoded
	}
	var out strings.Builder
	for len(encoded) > 76 {
		out.WriteString(encoded[:76])
		out.WriteString("\r\n")
		encoded = encoded[76:]
	}
	out.WriteString(encoded)
	return out.String()
}

func isASCII(value string) bool {
	for _, r := range value {
		if r > 127 {
			return false
		}
	}
	return true
}

func ParseMIMEMessage(raw []byte) (*mail.Message, error) {
	message, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	if message.Header.Get("From") == "" || message.Header.Get("To") == "" {
		return nil, errors.New("message missing sender or recipient headers")
	}
	return message, nil
}
