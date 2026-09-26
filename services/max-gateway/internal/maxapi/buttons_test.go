package maxapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNotificationButtonPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Attachments []struct {
				Payload struct {
					Buttons [][]map[string]string `json:"buttons"`
				} `json:"payload"`
			} `json:"attachments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		if len(body.Attachments) != 1 || len(body.Attachments[0].Payload.Buttons) != 1 {
			t.Error("missing keyboard")
			return
		}
		button := body.Attachments[0].Payload.Buttons[0][0]
		if button["type"] != "open_app" || button["text"] != "Перейти к опросу" || button["web_app"] != "test_bot" || button["payload"] != "poll_abc" {
			t.Error(button)
		}
		w.Write([]byte(`{"success":true}`))
	}))
	defer srv.Close()
	c := Client{BaseURL: srv.URL, Token: "test", BotUsername: "test_bot", HTTP: srv.Client()}
	if err := c.SendAction(context.Background(), 1, "🗳 Новый опрос", "Перейти к опросу", "poll_abc"); err != nil {
		t.Fatal(err)
	}
}
