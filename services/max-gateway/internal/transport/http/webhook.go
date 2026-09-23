package http

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/state"
)

type maxUpdate struct {
	Payload   string `json:"payload"`
	Type      string `json:"update_type"`
	Timestamp int64  `json:"timestamp"`
	User      struct {
		ID    int64 `json:"user_id"`
		IsBot bool  `json:"is_bot"`
	} `json:"user"`
	Message struct {
		Sender struct {
			ID    int64 `json:"user_id"`
			IsBot bool  `json:"is_bot"`
		} `json:"sender"`
		Body struct {
			MID  string `json:"mid"`
			Text string `json:"text"`
		} `json:"body"`
	} `json:"message"`
	Callback struct {
		ID      string `json:"callback_id"`
		Payload string `json:"payload"`
	} `json:"callback"`
}

func (a *API) webhook(w http.ResponseWriter, r *http.Request) {
	a.Metrics.Webhooks.Inc()
	secret := r.Header.Get("X-Max-Bot-Api-Secret")
	if a.Config.WebhookSecret == "" || subtle.ConstantTimeCompare([]byte(secret), []byte(a.Config.WebhookSecret)) != 1 {
		a.fail(w, r, 401, "UNAUTHENTICATED", "invalid webhook secret")
		return
	}
	if !a.limit(w, r, "webhook", a.Config.WebhookRate) {
		return
	}
	b, e := io.ReadAll(r.Body)
	if e != nil {
		a.fail(w, r, 413, "BODY_TOO_LARGE", "request body too large")
		return
	}
	var u maxUpdate
	if json.Unmarshal(b, &u) != nil || u.Type == "" || u.Timestamp <= 0 {
		a.fail(w, r, 400, "INVALID_ARGUMENT", "invalid MAX update")
		return
	}
	// MAX Update has no universal update_id. Canonical JSON hash also survives key ordering.
	var canonical any
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	if decoder.Decode(&canonical) != nil {
		a.fail(w, r, 400, "INVALID_ARGUMENT", "invalid MAX update")
		return
	}
	b, _ = json.Marshal(canonical)
	key := "gateway:webhook:" + state.Digest(string(b))
	done := key + ":done"
	exists, e := a.Store.R.Exists(r.Context(), done).Result()
	if e != nil {
		a.fail(w, r, 503, "DEPENDENCY_UNAVAILABLE", "webhook store unavailable")
		return
	}
	if exists > 0 {
		a.Metrics.WebhookDuplicates.Inc()
		write(w, 200, map[string]bool{"ok": true})
		return
	}
	ok, e := a.Store.R.SetNX(r.Context(), key, "processing", time.Minute).Result()
	if e != nil || !ok {
		a.fail(w, r, 503, "WEBHOOK_PENDING", "retry webhook later")
		return
	}
	defer a.Store.R.Del(r.Context(), key)
	switch u.Type {
	case "bot_started":
		if u.User.ID <= 0 || u.User.IsBot {
			a.fail(w, r, 400, "INVALID_ARGUMENT", "invalid MAX user")
			return
		}
		e = a.Bot.Menu(r.Context(), u.User.ID, "Умный Квартал: найдите свой дом, подайте заявку или зарегистрируйте новый.", u.Payload)
	case "message_created":
		if !u.Message.Sender.IsBot && u.Message.Sender.ID > 0 {
			parts := strings.Fields(u.Message.Body.Text)
			command, payload := "", ""
			if len(parts) > 0 {
				command = parts[0]
			}
			if len(parts) > 1 {
				payload = parts[1]
			}
			text := "Используйте /start, /help или /settings либо выберите раздел приложения."
			switch command {
			case "/start":
				text = "Добро пожаловать в Умный Квартал. Выберите действие."
			case "/help":
				text = "Найдите дом и подайте заявку председателю. Если дома ещё нет, зарегистрируйте его. Статусы доступны в разделе «Мои заявки»."
			case "/settings":
				text = "Настройки доставки уведомлений доступны в Mini App."
			}
			e = a.Bot.Menu(r.Context(), u.Message.Sender.ID, text, payload)
		}
	case "message_callback":
		if u.Callback.ID == "" {
			a.fail(w, r, 400, "INVALID_ARGUMENT", "callback id required")
			return
		}
		e = a.Bot.Answer(r.Context(), u.Callback.ID)
	}
	if e != nil {
		a.fail(w, r, 503, "MAX_UNAVAILABLE", "MAX delivery failed")
		return
	}
	if a.Store.R.Set(r.Context(), done, "1", 7*24*time.Hour).Err() != nil {
		a.fail(w, r, 503, "DEPENDENCY_UNAVAILABLE", "webhook store unavailable")
		return
	}
	write(w, 200, map[string]bool{"ok": true})
}
