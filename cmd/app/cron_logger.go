package main

import "log"

type cronLogger struct {
	// Implements gocron.Logger
}

func (c *cronLogger) Debug(msg string, args ...any) {
	log.Printf("[DEBUG] "+msg, args...)
}

func (c *cronLogger) Error(msg string, args ...any) {
	log.Printf("[ERROR] "+msg, args...)
}

func (c *cronLogger) Info(msg string, args ...any) {
	log.Printf("[INFO] "+msg, args...)
}

func (c *cronLogger) Warn(msg string, args ...any) {
	log.Printf("[WARN] "+msg, args...)
}
