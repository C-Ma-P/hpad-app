package actions

import (
	"context"
	"errors"
	"log"
	"os/exec"
	"strings"

	"github.com/google/shlex"

	"hpad-app/internal/config"
)

type Runner struct {
	ctx context.Context
}

func NewRunner(ctx context.Context) *Runner {
	return &Runner{ctx: ctx}
}

func (r *Runner) Run(index int, assignment config.KeyAssignment) error {
	command, args, workingDirectory, err := prepareCommand(assignment.Action)
	if err != nil {
		return err
	}

	keyLabel := assignment.Label
	if keyLabel == "" {
		keyLabel = assignment.ID
	}
	go func() {
		cmd := exec.CommandContext(r.ctx, command, args...)
		if workingDirectory != "" {
			cmd.Dir = workingDirectory
		}

		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("[actions] %s execution failed: %v (%s)", keyLabel, err, strings.TrimSpace(string(output)))
			return
		}
		trimmedOutput := strings.TrimSpace(string(output))
		if trimmedOutput != "" {
			log.Printf("[actions] %s executed: %s", keyLabel, trimmedOutput)
		}
	}()

	return nil
}

func (r *Runner) CanRun(action config.KeyAction) bool {
	switch config.NormalizeAction(action).Type {
	case config.ActionTypeRunCommand, config.ActionTypeOpenApplication:
		return true
	default:
		return false
	}
}

func parseArgs(input string) ([]string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}
	args, err := shlex.Split(input)
	if err != nil {
		return nil, errors.New("invalid command arguments")
	}
	return args, nil
}

func prepareCommand(action config.KeyAction) (string, []string, string, error) {
	action = config.NormalizeAction(action)
	switch action.Type {
	case config.ActionTypeRunCommand:
		if strings.TrimSpace(action.Command) == "" {
			return "", nil, "", errors.New("command is required")
		}
		args, err := parseArgs(action.Arguments)
		if err != nil {
			return "", nil, "", err
		}
		return action.Command, args, action.WorkingDirectory, nil
	case config.ActionTypeOpenApplication:
		if strings.TrimSpace(action.Application) == "" {
			return "", nil, "", errors.New("application path is required")
		}
		return action.Application, nil, "", nil
	default:
		return "", nil, "", errors.New("action cannot be executed")
	}
}
