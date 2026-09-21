package observability

import "go.uber.org/zap"

func NewLogger(environment, level string) (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	if err := cfg.Level.UnmarshalText([]byte(level)); err != nil {
		return nil, err
	}
	cfg.InitialFields = map[string]interface{}{"service": "issue-service", "environment": environment}
	cfg.DisableStacktrace = true
	return cfg.Build()
}
