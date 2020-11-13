package sdhook

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"cloud.google.com/go/compute/metadata"
	"github.com/fluent/fluent-logger-golang/fluent"
	"github.com/kenshaw/jwt/gserviceaccount"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	errorReporting "google.golang.org/api/clouderrorreporting/v1beta1"
	logging "google.golang.org/api/logging/v2"
)

// Option represents an option that modifies the Stackdriver hook settings.
type Option func(*Hook) error

// Levels is an option that sets the logrus levels that the StackdriverHook
// will create log entries for.
func Levels(levels ...logrus.Level) Option {
	return func(h *Hook) error {
		h.levels = levels
		return nil
	}
}

// ProjectID is an option that sets the project ID which is needed for the log
// name.
func ProjectID(projectID string) Option {
	return func(h *Hook) error {
		h.projectID = projectID
		return nil
	}
}

// EntriesService is an option that sets the Google API entry service to use
// with Stackdriver.
func EntriesService(service *logging.EntriesService) Option {
	return func(h *Hook) error {
		h.service = service
		return nil
	}
}

// LoggingService is an option that sets the Google API logging service to use.
func LoggingService(service *logging.Service) Option {
	return func(h *Hook) error {
		h.service = service.Entries
		return nil
	}
}

// ErrorService is an option that sets the Google API error reporting service to use.
func ErrorService(errorService *errorReporting.Service) Option {
	return func(h *Hook) error {
		h.errorService = errorService
		return nil
	}
}

// HTTPClient is an option that sets the http.Client to be used when creating
// the Stackdriver service.
func HTTPClient(client *http.Client) Option {
	return func(h *Hook) error {
		// create logging service
		l, err := logging.New(client)
		if err != nil {
			return err
		}
		// create error reporting service
		e, err := errorReporting.New(client)
		if err != nil {
			return err
		}
		if err = ErrorService(e)(h); err != nil {
			return err
		}
		return LoggingService(l)(h)
	}
}

// MonitoredResource is an option that sets the monitored resource to send with
// each log entry.
func MonitoredResource(resource *logging.MonitoredResource) Option {
	return func(h *Hook) error {
		h.resource = resource
		return nil
	}
}

// Resource is an option that sets the resource information to send with each
// log entry.
//
// Please see https://cloud.google.com/logging/docs/api/v2/resource-list for
// the list of labels required per ResType.
func Resource(typ ResType, labels map[string]string) Option {
	return func(h *Hook) error {
		return MonitoredResource(&logging.MonitoredResource{
			Type:   string(typ),
			Labels: labels,
		})(h)
	}
}

// LogName is an option that sets the log name to send with each log entry.
//
// Log names are specified as "projects/{projectID}/logs/{logName}"
// if the projectID is set. Otherwise, it's just "{logName}"
func LogName(name string) Option {
	return func(h *Hook) error {
		if h.projectID == "" {
			h.logName = name
		} else {
			h.logName = fmt.Sprintf("projects/%s/logs/%s", h.projectID, name)
		}
		return nil
	}
}

// ErrorReportingLogName is an option that sets the log name to send
// with each error message for error reporting.
// Only used when ErrorReportingService has been set.
func ErrorReportingLogName(name string) Option {
	return func(h *Hook) error {
		h.errorReportingLogName = name
		return nil
	}
}

// Labels is an option that sets the labels to send with each log entry.
func Labels(labels map[string]string) Option {
	return func(h *Hook) error {
		h.labels = labels
		return nil
	}
}

// PartialSuccess is an option that toggles whether or not to write partial log
// entries.
func PartialSuccess(enabled bool) Option {
	return func(h *Hook) error {
		h.partialSuccess = enabled
		return nil
	}
}

// ErrorReportingService is an option that defines the name of the service
// being tracked for Stackdriver error reporting.
// See:
// https://cloud.google.com/error-reporting/docs/formatting-error-messages
func ErrorReportingService(service string) Option {
	return func(h *Hook) error {
		h.errorReportingServiceName = service
		return nil
	}
}

// requiredScopes are the oauth2 scopes required for stackdriver logging.
var requiredScopes = []string{
	logging.CloudPlatformScope,
}

// GoogleServiceAccountCredentialsJSON is an option that creates the
// Stackdriver logging service using the supplied Google service account
// credentials.
//
// Google Service Account credentials can be downloaded from the Google Cloud
// console: https://console.cloud.google.com/iam-admin/serviceaccounts/
func GoogleServiceAccountCredentialsJSON(buf []byte) Option {
	return func(h *Hook) error {
		// load credentials
		gsa, err := gserviceaccount.FromJSON(buf)
		if err != nil {
			return err
		}
		// check project id
		if gsa.ProjectID == "" {
			return errors.New("google service account credentials missing project_id")
		}
		// set project id
		if err = ProjectID(gsa.ProjectID)(h); err != nil {
			return err
		}
		// set resource type
		err = Resource(ResTypeProject, map[string]string{
			"project_id": gsa.ProjectID,
		})(h)
		if err != nil {
			return err
		}
		// create token source
		ts, err := gsa.TokenSource(nil, requiredScopes...)
		if err != nil {
			return err
		}
		// set client
		return HTTPClient(&http.Client{
			Transport: &oauth2.Transport{
				Source: oauth2.ReuseTokenSource(nil, ts),
			},
		})(h)
	}
}

// GoogleServiceAccountCredentialsFile is an option that loads Google Service
// Account credentials for use with the StackdriverHook from the specified
// file.
//
// Google Service Account credentials can be downloaded from the Google Cloud
// console: https://console.cloud.google.com/iam-admin/serviceaccounts/
func GoogleServiceAccountCredentialsFile(path string) Option {
	return func(h *Hook) error {
		buf, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		return GoogleServiceAccountCredentialsJSON(buf)(h)
	}
}

// GoogleComputeCredentials is an option that loads the Google Service Account
// credentials from the GCE metadata associated with the GCE compute instance.
// If serviceAccount is empty, then the default service account credentials
// associated with the GCE instance will be used.
func GoogleComputeCredentials(serviceAccount string) Option {
	return func(h *Hook) error {
		// get compute metadata scopes associated with the service account
		scopes, err := metadata.Scopes(serviceAccount)
		if err != nil {
			return err
		}
		// check if all the necessary scopes are provided
		for _, s := range requiredScopes {
			if !sliceContains(scopes, s) {
				// NOTE: if you are seeing this error, you probably need to
				// recreate your compute instance with the correct scope
				//
				// as of August 2016, there is not a way to add a scope to an
				// existing compute instance
				return fmt.Errorf("missing required scope %s in compute metadata", s)
			}
		}
		return HTTPClient(&http.Client{
			Transport: &oauth2.Transport{
				Source: google.ComputeTokenSource(serviceAccount),
			},
		})(h)
	}
}

func GoogleLoggingAgent() Option {
	return func(h *Hook) error {
		// set agent client. It expects that the forward input fluentd plugin
		// is properly configured by the Google logging agent, which is by default.
		// See more at:
		// https://cloud.google.com/error-reporting/docs/setup/ec2
		var err error
		h.agentClient, err = fluent.New(
			fluent.Config{
				Async: true,
			},
		)
		if err != nil {
			return fmt.Errorf("could not find fluentd agent on 127.0.0.1:24224: %v", err)
		}
		return nil
	}
}

// sliceContains returns true if haystack contains needle.
func sliceContains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
