package maxapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/state"
)

type Client struct {
	BaseURL, Token, MiniAppURL, BotUsername string
	HTTP                                    *http.Client
	Store                                   state.Store
	Rate                                    int
}

var ErrDelivery = errors.New("MAX delivery failed")

// Menu uses MAX startapp deep links documented at dev.max.ru/help/deeplinks.
func (c *Client) Menu(ctx context.Context, userID int64, text, payload string) error {
	if userID <= 0 || c.Token == "" {
		return ErrDelivery
	}
	buttons := [][]any{}
	for _, item := range [][2]string{{"🏡 Мой дом", "houses"}, {"🔎 Найти дом", "find_house"}, {"📋 Мои заявки", "my_requests"}, {"🔔 Уведомления", "settings"}} {
		buttons = append(buttons, []any{map[string]string{"type": "link", "text": item[0], "url": "https://max.ru/" + url.PathEscape(c.BotUsername) + "?startapp=" + item[1]}})
	}
	if strings.HasPrefix(payload, "invite_") && len(payload) == 50 {
		valid := true
		for _, r := range payload {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
				valid = false
			}
		}
		if valid {
			buttons = append([][]any{{map[string]string{"type": "link", "text": "Открыть приглашение", "url": "https://max.ru/" + url.PathEscape(c.BotUsername) + "?startapp=" + payload}}}, buttons...)
		}
	}
	return c.post(ctx, "/messages?user_id="+strconv.FormatInt(userID, 10), map[string]any{"text": text, "attachments": []any{map[string]any{"type": "inline_keyboard", "payload": map[string]any{"buttons": buttons}}}}, strconv.FormatInt(userID, 10))
}

func (c *Client) Send(ctx context.Context, userID int64, text string, button bool) error {
	label := ""
	if button {
		label = "Открыть Умный Квартал"
	}
	return c.SendAction(ctx, userID, text, label, "")
}

func (c *Client) SendAction(ctx context.Context, userID int64, text, label, payload string) error {
	if userID <= 0 || c.Token == "" {
		return ErrDelivery
	}
	body := map[string]any{"text": text}
	if label != "" {
		body["attachments"] = []any{map[string]any{"type": "inline_keyboard", "payload": map[string]any{"buttons": [][]any{{map[string]string{"type": "open_app", "text": label, "web_app": c.BotUsername, "payload": payload}}}}}}
	}
	return c.post(ctx, "/messages?user_id="+strconv.FormatInt(userID, 10), body, strconv.FormatInt(userID, 10))
}
func (c *Client) Answer(ctx context.Context, id string) error {
	return c.post(ctx, "/answers?callback_id="+url.QueryEscape(id), map[string]string{"notification": "Откройте мини-приложение через кнопку бота."}, "callback")
}
func (c *Client) post(ctx context.Context, path string, body any, recipient string) error {
	b, e := json.Marshal(body)
	if e != nil {
		return ErrDelivery
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	for attempt := 0; attempt < 3; attempt++ {
		if c.Store.R != nil {
			ok, e := c.Store.Allow(ctx, "max-send:"+recipient, c.Rate, time.Second)
			if e != nil {
				return ErrDelivery
			}
			if !ok {
				if !wait(ctx, time.Second) {
					return ctx.Err()
				}
				continue
			}
		}
		req, e := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(c.BaseURL, "/")+path, bytes.NewReader(b))
		if e != nil {
			return ErrDelivery
		}
		req.Header.Set("Authorization", c.Token)
		req.Header.Set("Content-Type", "application/json")
		res, e := client.Do(req)
		if e != nil {
			return ErrDelivery
		}
		data, e := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if e != nil {
			return ErrDelivery
		}
		if res.StatusCode == 429 && attempt < 2 {
			delay := time.Second
			if n, e := strconv.Atoi(res.Header.Get("Retry-After")); e == nil && n > 0 {
				delay = time.Duration(n) * time.Second
			}
			if delay > 10*time.Second {
				return ErrDelivery
			}
			if !wait(ctx, delay) {
				return ctx.Err()
			}
			continue
		}
		if res.StatusCode < 200 || res.StatusCode >= 300 {
			return ErrDelivery
		}
		var result map[string]json.RawMessage
		if json.Unmarshal(data, &result) != nil {
			return ErrDelivery
		}
		if v, ok := result["success"]; ok && string(v) != "true" {
			return ErrDelivery
		}
		return nil
	}
	return ErrDelivery
}
func wait(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
