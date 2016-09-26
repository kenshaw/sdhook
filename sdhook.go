// Package sdhook provides a logrus compatible logging hook for Google
// Stackdriver logging.
package sdhook

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"

	logging "google.golang.org/api/logging/v2beta1"
)

const (
	// DefaultName is the default name passed to LogName when using service
	// account credentials.
	DefaultName = "default"
)

// StackdriverHook provides a logrus hook to Google Stackdriver logging.
type StackdriverHook struct {
	// levels are the levels that logrus will hook to.
	levels []logrus.Level

	// projectID is the projectID
	projectID string

	// service is the logging service.
	service *logging.EntriesService

	// resource is the monitored resource.
	resource *logging.MonitoredResource

	// logName is the name of the log.
	logName string

	// labels are the labels to send with each log entry.
	labels map[string]string

	// partialSuccess allows partial writes of log entries if there is a badly
	// formatted log.
	partialSuccess bool
}

// New creates a StackdriverHook using the provided options that is suitible
// for using with logrus for logging to Google Stackdriver.
func New(opts ...Option) (*StackdriverHook, error) {
	var err error

	sh := &StackdriverHook{
		levels: logrus.AllLevels,
	}

	// apply opts
	for _, o := range opts {
		err = o(sh)
		if err != nil {
			return nil, err
		}
	}

	// check service, resource, logName set
	if sh.service == nil {
		return nil, errors.New("no stackdriver service was provided")
	}
	if sh.resource == nil {
		return nil, errors.New("the monitored resource was not provided")
	}
	if sh.projectID == "" {
		return nil, errors.New("the project id was not provided")
	}

	// set default project name
	if sh.logName == "" {
		err = LogName(DefaultName)(sh)
		if err != nil {
			return nil, err
		}
	}

	return sh, nil
}

// Levels returns the logrus levels that this hook is applied to. This can be
// set using the Levels Option.
func (sh *StackdriverHook) Levels() []logrus.Level {
	return sh.levels
}

// Fire writes the message to the Stackdriver entry service.
func (sh *StackdriverHook) Fire(entry *logrus.Entry) error {
	go func() {
		// convert entry data to labels
		labels := make(map[string]string, len(entry.Data))
		for k, v := range entry.Data {
			labels[k] = fmt.Sprintf("%v", v)
		}

		// write log entry
		_, _ = sh.service.Write(&logging.WriteLogEntriesRequest{
			LogName:        sh.logName,
			Resource:       sh.resource,
			Labels:         sh.labels,
			PartialSuccess: sh.partialSuccess,
			Entries: []*logging.LogEntry{
				&logging.LogEntry{
					Severity:    strings.ToUpper(entry.Level.String()),
					Timestamp:   entry.Time.Format(time.RFC3339),
					TextPayload: entry.Message,
					Labels:      labels,
				},
			},
		}).Do()
	}()

	return nil
}
