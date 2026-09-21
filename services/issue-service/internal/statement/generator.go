package statement

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"github.com/ZheglY/SmartQuarter/services/issue-service/internal/domain"
	"text/template"
	"time"
)

//go:embed templates/statement.txt
var source string
var document = template.Must(template.New("statement").Option("missingkey=error").Parse(source))

type Snapshot struct {
	HouseAddress       string          `json:"house_address"`
	Category           domain.Category `json:"category"`
	CreatedAt          string          `json:"created_at"`
	Location           string          `json:"location"`
	Description        string          `json:"description"`
	ConfirmationsCount int             `json:"confirmations_count"`
	ChairmanNote       string          `json:"chairman_note"`
}

func Generate(issue domain.Issue, note string) (string, json.RawMessage, error) {
	snapshot := Snapshot{HouseAddress: issue.HouseAddressSnapshot, Category: issue.Category, CreatedAt: issue.CreatedAt.UTC().Format(time.RFC3339), Location: issue.LocationText, Description: issue.Description, ConfirmationsCount: issue.ConfirmationsCount, ChairmanNote: note}
	var buffer bytes.Buffer
	if err := document.Execute(&buffer, snapshot); err != nil {
		return "", nil, err
	}
	data, err := json.Marshal(snapshot)
	return buffer.String(), data, err
}
