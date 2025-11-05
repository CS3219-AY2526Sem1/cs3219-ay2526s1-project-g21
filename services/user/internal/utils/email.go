package utils

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"os"
)

type SMTPCfg struct {
	Host string
	Port string
	User string
	Pass string
	From string
}

func loadSMTP() (*SMTPCfg, error) {
	cfg := &SMTPCfg{
		Host: os.Getenv("SMTP_HOST"),
		Port: os.Getenv("SMTP_PORT"),
		User: os.Getenv("SMTP_USER"),
		Pass: os.Getenv("SMTP_PASS"),
		From: os.Getenv("SMTP_FROM"),
	}
	if cfg.Host == "" {
		cfg.Host = "smtp.gmail.com"
	}
	if cfg.Port == "" {
		cfg.Port = "587"
	}
	if cfg.From == "" {
		cfg.From = cfg.User
	}
	if cfg.User == "" || cfg.Pass == "" || cfg.From == "" {
		return nil, fmt.Errorf("SMTP not configured")
	}
	return cfg, nil
}

func SendEmail(to, subject, body string) error {
	cfg, err := loadSMTP()
	if err != nil {
		return err
	}

	addr := cfg.Host + ":" + cfg.Port
	auth := smtp.PlainAuth("", cfg.User, cfg.Pass, cfg.Host)
	msg := []byte("From: \"Peerprep\" <" + cfg.User + ">\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=\"utf-8\"\r\n\r\n" +
		body + "\r\n")

	if err := smtp.SendMail(addr, auth, cfg.User, []string{to}, msg); err != nil {
		if cfg.Port == "465" {
			tlsconfig := &tls.Config{ServerName: cfg.Host}
			conn, cerr := tls.Dial("tcp", addr, tlsconfig)
			if cerr != nil {
				return cerr
			}
			c, cerr := smtp.NewClient(conn, cfg.Host)
			if cerr != nil {
				return cerr
			}
			defer c.Quit()
			if err = c.Auth(auth); err != nil {
				return err
			}
			if err = c.Mail(cfg.From); err != nil {
				return err
			}
			if err = c.Rcpt(to); err != nil {
				return err
			}
			wc, cerr := c.Data()
			if cerr != nil {
				return cerr
			}
			if _, cerr = wc.Write(msg); cerr != nil {
				return cerr
			}
			if cerr = wc.Close(); cerr != nil {
				return cerr
			}
			return nil
		}
		return err
	}
	return nil
}
