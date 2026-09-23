package notification

import (
	"context"
	"errors"
	ipb "github.com/ZheglY/SmartQuarter/services/max-gateway/internal/gen/smartquarter/identity/v1"
	"github.com/ZheglY/SmartQuarter/services/max-gateway/internal/identity"
	"strconv"
	"strings"
	"time"
)

func validEvent(e Event) bool {
	switch e.Producer {
	case "issue-service":
		return identity.ValidID(e.Payload.IssueID) && identity.ValidID(e.Payload.HouseID) && identity.ValidID(e.Payload.CreatedBy) && (strings.HasPrefix(e.Type, "issue.") || e.Type == "statement.generated")
	case "identity-service":
		return strings.HasPrefix(e.Type, "house.") && identity.ValidID(e.Payload.RecipientUserID) && (e.Payload.HouseID == "" || identity.ValidID(e.Payload.HouseID)) && eventText(e.Type) != ""
	case "community-service":
		return identity.ValidID(e.Payload.HouseID) && !strings.HasPrefix(e.Type, "house.") && eventText(e.Type) != ""
	}
	return false
}
func eventText(kind string) string {
	return map[string]string{
		"house.join.cancelled":         "Заявка на вступление отменена.",
		"house.registration.created":   "Заявка на регистрацию дома принята. Решение появится в разделе «Мои заявки».",
		"house.registration.approved":  "Регистрация дома одобрена. Вы назначены председателем. Откройте приложение и выберите дом.",
		"house.registration.rejected":  "Заявка на регистрацию дома отклонена. Подробности доступны в разделе «Мои заявки».",
		"house.registration.cancelled": "Заявка на регистрацию дома отменена.",
		"house.created":                "Дом зарегистрирован.", "house.chairman.assigned": "Вам назначена роль председателя.",
		"house.join.created":  "Получена заявка на вступление в дом. Откройте раздел управления заявками.",
		"house.join.approved": "Ваша заявка на вступление одобрена. Выберите дом в приложении.", "house.join.rejected": "Ваша заявка на вступление отклонена.",
		"house.membership.created": "Участие в доме создано.", "house.membership.activated": "Ваш доступ к дому активирован.", "house.membership.deactivated": "Ваш доступ к дому приостановлен.", "house.membership.removed": "Ваше участие в доме удалено.",
		"house.invite.created": "Приглашение в дом создано.", "house.invite.redeemed": "Приглашение принято. Дождитесь одобрения заявки.", "house.invite.revoked": "Приглашение отозвано.",
		"house.chairman.transfer_requested": "Вам предложена роль председателя. Подтвердите или отклоните передачу в приложении.", "house.chairman.transfer_completed": "Роль председателя передана. Вы остались жителем дома.", "house.chairman.transfer_accepted": "Передача роли председателя подтверждена.", "house.chairman.transfer_rejected": "Предложение роли председателя отклонено.", "house.chairman.transfer_cancelled": "Предложение роли председателя отменено.",
		"announcement.created": "В вашем доме опубликовано объявление.", "poll.created": "В вашем доме началось голосование.", "poll.voted": "Результаты голосования в вашем доме обновились.", "calendar.event_created": "В календаре вашего дома новое событие.", "initiative.created": "В вашем доме появилась новая инициатива.",
		"service_contact.create": "В справочник служб дома добавлен контакт.", "service_contact.update": "Контакты служб дома обновлены.", "service_contact.archive": "Справочник служб дома обновлён.",
	}[kind]
}
func (c *Consumer) deliverRecipients(ctx context.Context, e Event, text string) error {
	category, user := "announcement", ""
	if e.Producer == "issue-service" {
		category, user = "issue", e.Payload.CreatedBy
	}
	if e.Producer == "identity-service" {
		category, user = "membership", e.Payload.RecipientUserID
		if e.Type == "house.join.created" {
			category, user = "join_request", ""
		}
	}
	after := ""
	for {
		page, err := c.House.ListNotificationRecipients(ctx, &ipb.ListNotificationRecipientsRequest{HouseId: e.Payload.HouseID, UserId: user, Category: category, AfterUserId: after})
		if err != nil {
			return err
		}
		for _, recipient := range page.Items {
			if !identity.ValidID(recipient.UserId) || recipient.UserId <= after {
				return errors.New("invalid recipient page")
			}
			done := "gateway:notification:" + e.ID + ":recipient:" + recipient.UserId
			exists, err := c.Redis.Exists(ctx, done).Result()
			if err != nil {
				return err
			}
			if exists > 0 {
				continue
			}
			// Re-evaluate preferences and active membership immediately before sending.
			fresh, err := c.House.ListNotificationRecipients(ctx, &ipb.ListNotificationRecipientsRequest{HouseId: e.Payload.HouseID, UserId: recipient.UserId, Category: category})
			if err != nil {
				return err
			}
			if len(fresh.Items) == 0 {
				continue
			}
			if len(fresh.Items) != 1 || fresh.Items[0].UserId != recipient.UserId {
				return errors.New("recipient mismatch")
			}
			id, err := strconv.ParseInt(fresh.Items[0].MaxUserId, 10, 64)
			if err != nil || id <= 0 {
				return errors.New("invalid MAX recipient")
			}
			if err = c.Bot.Send(ctx, id, text, true); err != nil {
				return err
			}
			if err = c.Redis.Set(ctx, done, "1", 7*24*time.Hour).Err(); err != nil {
				return err
			}
		}
		if len(page.Items) < 100 {
			return nil
		}
		next := page.Items[len(page.Items)-1].UserId
		if next <= after {
			return errors.New("invalid recipient cursor")
		}
		after = next
	}
}
