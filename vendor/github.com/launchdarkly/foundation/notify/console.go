package notify

import (
	"fmt"
	"os"
)

type Console struct{}

func (c Console) Output(calldepth int, level Level, notification Notification) {
	logEntry := fmt.Sprintf("Notification: %s %v\n", notification.String(), notification.ErrorContext())
	for _, frame := range notification.Stack() {
		logEntry += frame.String()
	}
	os.Stderr.WriteString(logEntry)
}
