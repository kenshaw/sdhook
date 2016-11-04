// Package sdhook provides a logrus compatible logging hook for Google
// Stackdriver logging.
package sdhook

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/fluent/fluent-logger-golang/fluent"

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

	// agentClient defines the fluentd logger object that can send data to
	// to the Google logging agent.
	agentClient *fluent.Fluent
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
	if sh.service == nil && sh.agentClient == nil {
		return nil, errors.New("no stackdriver service was provided")
	}
	if sh.resource == nil && sh.agentClient == nil {
		return nil, errors.New("the monitored resource was not provided")
	}
	if sh.projectID == "" && sh.agentClient == nil {
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
		var httpReq *logging.HttpRequest

		// convert entry data to labels
		labels := make(map[string]string, len(entry.Data))
		for k, v := range entry.Data {
			switch x := v.(type) {
			case string:
				labels[k] = x

			case *http.Request:
				httpReq = &logging.HttpRequest{
					Referer:       x.Referer(),
					RemoteIp:      x.RemoteAddr,
					RequestMethod: x.Method,
					RequestUrl:    x.URL.String(),
					UserAgent:     x.UserAgent(),
				}

			case *logging.HttpRequest:
				httpReq = x

			default:
				labels[k] = fmt.Sprintf("%v", v)
			}
		}

		// write log entry
		if sh.agentClient != nil {
			// The log entry payload schema is defined by the Google fluentd
			// logging agent. See more at:
			// https://github.com/GoogleCloudPlatform/fluent-plugin-google-cloud
			logEntry := map[string]interface{}{
				"severity":         strings.ToUpper(entry.Level.String()),
				"timestampSeconds": strconv.FormatInt(entry.Time.Unix(), 10),
				"timestampNanos":   strconv.FormatInt(entry.Time.UnixNano()-entry.Time.Unix()*1000000000, 10),
				"message":          entry.Message,
			}
			for k, v := range labels {
				logEntry[k] = v
			}
			if httpReq != nil {
				logEntry["httpRequest"] = httpReq
			}
			if err := sh.agentClient.Post(sh.logName, logEntry); err != nil {
				log.Printf("error posting log entries to logging agent: %s", err.Error())
			}
		} else {
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
						HttpRequest: httpReq,
					},
				},
			}).Do()
		}
	}()

	return nil
}
