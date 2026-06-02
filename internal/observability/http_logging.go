package observability

import (
	"bytes"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var redactedKeys = []string{
	"password", "token", "secret", "apikey", "api_key",
	"otp", "pin", "authorization", "access_token", "refresh_token",
}

var omittedKeys = []string{
	"base64", "image", "binary", "file", "avatar",
}

const maxBodyRunes = 512

func BaseHTTPFields(c *gin.Context, service, operation string) []zap.Field {
	fields := []zap.Field{
		zap.String("service", service),
		zap.String("operation", operation),
		zap.String("method", c.Request.Method),
		zap.String("path", c.Request.URL.Path),
		zap.String("client_ip", c.ClientIP()),
		zap.String("route", c.FullPath()),
	}

	if requestID, exists := c.Get("request_id"); exists {
		if id, ok := requestID.(string); ok {
			fields = append(fields, zap.String("request_id", id))
		}
	}

	if userID, exists := c.Get("user_id"); exists {
		if id, ok := userID.(string); ok {
			fields = append(fields, zap.String("user_id", id))
		}
	}

	for _, param := range c.Params {
		fields = append(fields, zap.String("path_param_"+param.Key, param.Value))
	}

	return fields
}

func RequestEnvelopeFields(c *gin.Context) []zap.Field {
	var fields []zap.Field

	if auth := c.GetHeader("Authorization"); auth != "" {
		fields = append(fields, zap.Bool("authorization_present", true))
	} else {
		fields = append(fields, zap.Bool("authorization_present", false))
	}

	if platform := c.GetHeader("X-App-Platform"); platform != "" {
		fields = append(fields, zap.String("app_platform", platform))
	}

	if version := c.GetHeader("X-App-Version"); version != "" {
		fields = append(fields, zap.String("app_version_code", version))
	}

	if c.Request.URL.RawQuery != "" {
		fields = append(fields, zap.String("query_params", c.Request.URL.RawQuery))
	}

	return fields
}

func RequestSnapshotFields(c *gin.Context, capture *RequestBodyCapture) []zap.Field {
	var fields []zap.Field

	if capture != nil && capture.buffer.Len() > 0 {
		body := capture.buffer.String()
		sanitized := sanitizeBody(body)
		fields = append(fields, zap.String("request_body", sanitized))
		fields = append(fields, zap.Int("body_bytes", len(body)))
	}

	return fields
}

func sanitizeBody(body string) string {
	if !utf8.ValidString(body) {
		return "[OMITTED len=" + itoa(len(body)) + "]"
	}

	runes := []rune(body)
	if len(runes) > maxBodyRunes {
		body = string(runes[:maxBodyRunes]) + "...[TRUNCATED len=" + itoa(len(runes)) + "]"
	}

	lower := strings.ToLower(body)
	for _, key := range redactedKeys {
		if strings.Contains(lower, key) {
			return "[REDACTED len=" + itoa(len(body)) + "]"
		}
	}

	for _, key := range omittedKeys {
		if strings.Contains(lower, key) {
			return "[OMITTED len=" + itoa(len(body)) + "]"
		}
	}

	return body
}

func itoa(n int) string {
	return strings.TrimSpace(strings.Replace(
		strings.Replace(
			strings.Replace(
				string(rune('0'+n/100))+string(rune('0'+n/10%10))+string(rune('0'+n%10)),
				string(rune('0')), "", -1,
			),
			" ", "", -1,
		),
		"\x00", "", -1,
	))
}

type RequestBodyCapture struct {
	reader io.ReadCloser
	buffer bytes.Buffer
}

func NewRequestBodyCapture(body io.ReadCloser) *RequestBodyCapture {
	return &RequestBodyCapture{
		reader: body,
	}
}

func (r *RequestBodyCapture) Read(p []byte) (n int, err error) {
	n, err = r.reader.Read(p)
	if n > 0 {
		r.buffer.Write(p[:n])
	}
	return n, err
}

func (r *RequestBodyCapture) Close() error {
	return r.reader.Close()
}
