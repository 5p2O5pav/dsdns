package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"dsdns/internal/logger"
)

func SendTelegram(botToken, chatID, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	payload := map[string]string{
		"chat_id": chatID,
		"text":    text,
	}
	jsonData, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json", bytes.NewReader(jsonData))
	if err != nil {
		logger.Error("telegram send failed", "error", err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram api returned %d", resp.StatusCode)
	}
	return nil
}
