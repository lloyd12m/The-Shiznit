package utils

import (
	"bufio"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const telegramConfigFile = "config"

type TelegramConfig struct {
	BotToken string
	ChatID   string
}

// ReadTelegramConfig reads Telegram credentials from the plain-text config file.
// Expected format:
// telegram_bot_token=123456:ABC...
// telegram_chat_id=123456789
func ReadTelegramConfig() (TelegramConfig, error) {
	file, err := os.Open(telegramConfigFile)
	if err != nil {
		return TelegramConfig{}, fmt.Errorf("open %s: %w", telegramConfigFile, err)
	}
	defer file.Close()

	config := TelegramConfig{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(strings.ToLower(key)) {
		case "telegram_bot_token", "bot_token":
			config.BotToken = strings.TrimSpace(value)
		case "telegram_chat_id", "chat_id":
			config.ChatID = strings.TrimSpace(value)
		}
	}
	if err := scanner.Err(); err != nil {
		return TelegramConfig{}, fmt.Errorf("read %s: %w", telegramConfigFile, err)
	}
	if config.BotToken == "" || config.ChatID == "" {
		return TelegramConfig{}, fmt.Errorf("%s must contain telegram_bot_token and telegram_chat_id", telegramConfigFile)
	}
	return config, nil
}

// SendClaimToTelegram sends the same successful claim line that is written to claims.txt.
func SendClaimToTelegram(claimLine string) error {
	config, err := ReadTelegramConfig()
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", config.BotToken)
	payload := url.Values{}
	payload.Set("chat_id", config.ChatID)
	payload.Set("text", "Claim result:\n"+claimLine)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.PostForm(endpoint, payload)
	if err != nil {
		return fmt.Errorf("send Telegram message: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Telegram API returned status %s", resp.Status)
	}
	return nil
}
