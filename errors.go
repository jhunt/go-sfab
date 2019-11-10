package sfab

import (
	"errors"
)

var (
	AgentNotFoundError      = errors.New("agent not found")
	AgentNotAuthorizedError = errors.New("agent not authorized")
)

func IsAgentNotAvailableError(e error) bool {
	return e == AgentNotFoundError || e == AgentNotAuthorizedError
}
