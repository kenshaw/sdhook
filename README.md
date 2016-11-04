# About sdhook

Package sdhook provides a [logrus](https://github.com/Sirupsen/logrus)
compatible hook for [Google Stackdriver logging](https://cloud.google.com/logging/).

# Installation

Install in the usual Go way:
```sh
go get -u github.com/knq/sdhook
```

# Usage

Simply create the hook, and add it to a logrus logger:

```go
// create hook using service account credentials
h, err := sdhook.New(
	sdhook.GoogleServiceAccountCredentialsFile("./credentials.json"),
)

// create logger with extra fields
//
// logrus fields will be converted to Stackdriver labels
logger := logrus.New().WithFields(logrus.Fields{
	"field1": 15,
	"field2": 20,
})

// add hook
logger.Hooks.Add(h)

// log something
logger.Printf("something %d", 15)
```

The example above sends log entries directly to the logging API. If you have the logging agent running, you can send log entries to it instead, with the added benefit of having extra instance metadata added to your log entries by the agent. In the example above, the initialization would simply be:

```go
// create hook using the logging agent
h, err := sdhook.New(
	sdhook.GoogleLoggingAgent(),
)
```

Please also see [example/example.go](example/example.go) for a more complete
example.

See [GoDoc](https://godoc.org/github.com/knq/sdhook) for a full API listing.
