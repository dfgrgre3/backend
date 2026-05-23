package services

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
)

type EmailService struct {
	enabled   bool
	host      string
	port      string
	username  string
	password  string
	fromEmail string
	fromName  string
}

var emailServiceInstance *EmailService

func GetEmailService() *EmailService {
	if emailServiceInstance == nil {
		// Support both naming conventions: SMTP_USER/SMTP_PASS (from .env) and SMTP_USERNAME/SMTP_PASSWORD
		username := os.Getenv("SMTP_USER")
		if username == "" {
			username = os.Getenv("SMTP_USERNAME")
		}
		password := os.Getenv("SMTP_PASS")
		if password == "" {
			password = os.Getenv("SMTP_PASSWORD")
		}
		fromEmail := os.Getenv("SMTP_FROM_EMAIL")
		fromName := os.Getenv("SMTP_FROM_NAME")
		// Parse SMTP_FROM format: "Name <email>" into separate fields
		if fromRaw := os.Getenv("SMTP_FROM"); fromRaw != "" && fromEmail == "" {
			if idx := strings.Index(fromRaw, "<"); idx > 0 {
				fromName = strings.TrimSpace(fromRaw[:idx])
				fromEmail = strings.Trim(fromRaw[idx:], "<> ")
			} else {
				fromEmail = fromRaw
			}
		}

		// Auto-enable if SMTP_HOST is set and SMTP_ENABLED is not explicitly false
		enabled := os.Getenv("SMTP_ENABLED") == "true"
		if !enabled && os.Getenv("SMTP_ENABLED") == "" && os.Getenv("SMTP_HOST") != "" {
			enabled = true
		}

		emailServiceInstance = &EmailService{
			enabled:   enabled,
			host:      os.Getenv("SMTP_HOST"),
			port:      os.Getenv("SMTP_PORT"),
			username:  username,
			password:  password,
			fromEmail: fromEmail,
			fromName:  fromName,
		}
	}
	return emailServiceInstance
}

// SendEmail sends an email to a single recipient
func (s *EmailService) SendEmail(to, subject, body string, isHTML bool) error {
	if !s.enabled {
		fmt.Printf("[Email] Service disabled, skipping email to %s: %s\n", to, subject)
		return nil
	}

	if s.host == "" || s.username == "" {
		return fmt.Errorf("SMTP configuration is incomplete")
	}

	// Build message
	contentType := "text/plain; charset=UTF-8"
	if isHTML {
		contentType = "text/html; charset=UTF-8"
	}

	msg := fmt.Sprintf("From: %s <%s>\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"Content-Type: %s\r\n\r\n"+
		"%s",
		s.fromName, s.fromEmail, to, subject, contentType, body)

	// SMTP authentication
	auth := smtp.PlainAuth("", s.username, s.password, s.host)

	// Send email
	addr := fmt.Sprintf("%s:%s", s.host, s.port)
	err := smtp.SendMail(addr, auth, s.fromEmail, []string{to}, []byte(msg))
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// SendBulkEmail sends the same email to multiple recipients
func (s *EmailService) SendBulkEmail(to []string, subject, body string, isHTML bool) map[string]error {
	results := make(map[string]error)

	for _, recipient := range to {
		err := s.SendEmail(recipient, subject, body, isHTML)
		results[recipient] = err
	}

	return results
}

// SendTemplateEmail sends an email using a template
func (s *EmailService) SendTemplateEmail(to, subject, templateName string, data map[string]interface{}) error {
	if !s.enabled {
		fmt.Printf("[Email] Service disabled, skipping template email to %s: %s (Template: %s)\n", to, subject, templateName)
		return nil
	}

	templatesDir := os.Getenv("EMAIL_TEMPLATES_DIR")
	if templatesDir == "" {
		templatesDir = "templates"
	}

	templatePath := filepath.Join(templatesDir, templateName+".html")

	var bodyBytes []byte
	var err error

	if _, statErr := os.Stat(templatePath); statErr == nil {
		bodyBytes, err = renderTemplateFile(templatePath, data)
		if err != nil {
			return fmt.Errorf("failed to parse/execute email template %s: %w", templateName, err)
		}
	} else {
		bodyBytes, err = renderFallbackTemplate(subject, data)
		if err != nil {
			return err
		}
	}

	return s.SendEmail(to, subject, string(bodyBytes), true)
}

func renderTemplateFile(templatePath string, data map[string]interface{}) ([]byte, error) {
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}
	return buf.Bytes(), nil
}

func renderFallbackTemplate(subject string, data map[string]interface{}) ([]byte, error) {
	const fallbackTmplSrc = `
			<div dir="rtl" style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px; border: 1px solid #eee; border-radius: 5px;">
				<h2 style="color: #333;">{{.Title}}</h2>
				<div style="color: #666; line-height: 1.6; margin-top: 20px;">
					{{.Message}}
				</div>
				{{if .ActionURL}}
				<div style="margin-top: 25px; text-align: center;">
					<a href="{{.ActionURL}}" style="display: inline-block; padding: 10px 20px; background: #007bff; color: white; text-decoration: none; border-radius: 5px; font-weight: bold;">{{if .ActionText}}{{.ActionText}}{{else}}تحقق من التفاصيل{{end}}</a>
				</div>
				{{end}}
				<hr style="border: none; border-top: 1px solid #eee; margin: 25px 0;">
				<p style="color: #999; font-size: 12px; text-align: center;">هذه رسالة آلية من منصة التعلّم</p>
			</div>
		`
	tmpl, err := template.New("fallback").Parse(fallbackTmplSrc)
	if err != nil {
		return nil, fmt.Errorf("failed to parse fallback template: %w", err)
	}

	prepareFallbackData(subject, data)

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to execute fallback template: %w", err)
	}
	return buf.Bytes(), nil
}

func prepareFallbackData(subject string, data map[string]interface{}) {
	if _, ok := data["Title"]; !ok {
		data["Title"] = subject
	}
	if _, ok := data["Message"]; !ok {
		if msg, ok := data["message"].(string); ok {
			data["Message"] = msg
		} else {
			var fields []string
			for k, v := range data {
				fields = append(fields, fmt.Sprintf("<strong>%s:</strong> %v", k, v))
			}
			data["Message"] = strings.Join(fields, "<br>")
		}
	}
}

// BuildNotificationEmail builds a notification email body
func (s *EmailService) BuildNotificationEmail(title, message, actionURL string) string {
	return fmt.Sprintf(`
		<div dir="rtl" style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 20px;">
			<h2 style="color: #333;">%s</h2>
			<p style="color: #666; line-height: 1.6;">%s</p>
			%s
			<hr style="border: none; border-top: 1px solid #eee; margin: 20px 0;">
			<p style="color: #999; font-size: 12px;">هذه رسالة آلية من منصة التعلّم</p>
		</div>
	`, title, message, func() string {
		if actionURL != "" {
			return fmt.Sprintf(`<a href="%s" style="display: inline-block; padding: 10px 20px; background: #007bff; color: white; text-decoration: none; border-radius: 5px;">تحقق من التفاصيل</a>`, actionURL)
		}
		return ""
	}())
}

// ValidateEmail checks if an email address is valid (basic check)
func (s *EmailService) ValidateEmail(email string) bool {
	return strings.Contains(email, "@") && strings.Contains(email, ".")
}
