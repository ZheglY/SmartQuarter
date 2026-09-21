package statement

import (
	"bytes"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"strings"
	"testing"
	"time"
)

func TestDeterministicStatement(t *testing.T) {
	issue := domain.Issue{HouseAddressSnapshot: "Дом 42", Category: domain.Safety, CreatedAt: time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC), LocationText: "Подъезд 2", Description: "Описание {{.Secret}}", ConfirmationsCount: 3}
	text, snapshot, err := Generate(issue, "Проверено председателем")
	if err != nil {
		t.Fatal(err)
	}
	again, snapshotAgain, err := Generate(issue, "Проверено председателем")
	if err != nil || text != again || !bytes.Equal(snapshot, snapshotAgain) {
		t.Fatal("not deterministic")
	}
	for _, part := range []string{"Дом 42", "SAFETY", "2026-09-16T10:00:00Z", "Подъезд 2", "Описание {{.Secret}}", "3", "Проверено председателем"} {
		if !strings.Contains(text, part) {
			t.Errorf("missing %q", part)
		}
	}
}
