package main

import (
	"context"

	backend "hpad-app/internal/app"
	"hpad-app/internal/config"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type AgentService struct {
	core       *backend.Core
	startupCtx context.Context
}

func NewAgentService() (*AgentService, error) {
	core, err := backend.NewCore()
	if err != nil {
		return nil, err
	}
	return &AgentService{core: core}, nil
}

func (s *AgentService) ServiceStartup(ctx context.Context, _ application.ServiceOptions) error {
	s.startupCtx = ctx
	return s.core.Start(ctx)
}

func (s *AgentService) ServiceShutdown() error {
	return s.core.Stop()
}

func (s *AgentService) GetDashboardState() backend.DashboardState {
	return s.core.GetDashboardState()
}

func (s *AgentService) ApplyKeyAction(index int, action config.KeyAction) (backend.DashboardState, error) {
	return s.core.ApplyKeyAction(index, action)
}

func (s *AgentService) ClearKeyAction(index int) (backend.DashboardState, error) {
	return s.core.ClearKeyAction(index)
}

func (s *AgentService) SaveToDevice() (backend.DashboardState, error) {
	return s.core.SaveToDevice()
}
