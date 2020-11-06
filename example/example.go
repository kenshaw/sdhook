package main

import (
	"time"

	"github.com/kenshaw/sdhook"
	"github.com/sirupsen/logrus"
)

func main() {
	// create a logger with some fields
	logger := logrus.New()
	logger.WithFields(logrus.Fields{
		"my_field":  115888,
		"my_field2": 898858,
	})

	// create stackdriver hook
	hook, err := sdhook.New(
		sdhook.GoogleServiceAccountCredentialsFile("./credentials.json"),
		sdhook.LogName("some_log"),
	)
	if err != nil {
		logger.Fatal(err)
	}

	// add to logrus
	logger.Hooks.Add(hook)

	// log some message
	logger.Printf("a random message @ %s", time.Now().Format("15:04:05"))

	// wait for the writes to finish
	time.Sleep(10 * time.Second)
}
