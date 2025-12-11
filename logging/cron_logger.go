package logging

import "log"

type CronLogger struct {
	// Implements gocron.Logger
}

func (c *CronLogger) Debug(msg string, args ...any) {
	log.Printf("[DEBUG] "+msg, args...)
}

func (c *CronLogger) Error(msg string, args ...any) {
	log.Printf("[ERROR] "+msg, args...)
}

func (c *CronLogger) Info(msg string, args ...any) {
	log.Printf("[INFO] "+msg, args...)
}

func (c *CronLogger) Warn(msg string, args ...any) {
	log.Printf("[WARN] "+msg, args...)
}
