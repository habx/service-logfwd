# Logstash 2 scalyr

## Goal
This tool allows to use scalyr logging platform as drop-in replacement for your previous logging architecture. 
Switching your previous logs to it should be a matter of minutes.

## Why
Scalyr is a very convenient logging solution but it lacks some third-party integrations.

Logstash has logging clients in all languages, it's good compromise between simplicity and performance.

The scalyr agent, at least in kubernetes, consumes a lot of CPU and memory. So I quickly discarded it.

This all started because logmatic pushed us out of their service.

## How it works

### General overview
For each client connected to the server, it will create a new scalyr logging session.

Log events are sent as soon as possible and grouped (by at most 20) when more than one are waiting the previous query.

Each logstash event is converted to a scalyr event by parsing and translating some fields.

### Performed translations
To easily re-use an existing logstash implementation, a few tricks are needed:
 
- `@timestamp` is converted to the scalyr's ts field
- `@message` becomes the `message` attributes
- `@fields` are brought moved to the root attribute properties (as scalyr doesn't support nested search)
- `level` / `levelname` fields are converted to the scalyr's `sev` fields
- `source` is set to `logstash2scalyr`

### Env vars
Everything is handled through environment variables

- `SCALYR_WRITELOG_TOKEN` (mandatory) : Your log write token
- `LISTEN_ADDR` (optional): Port to listen on. Defaults to `:5050`
- `FIELDS_CONV_MESSAGE` (optional): Conversion to apply between logstash and scalyr event attributes
- `FIELDS_CONV_SESSION` (optional): Conversion to apply between logstash events and scalyr log session attributes
- `LOG_ENV` (optional): Logging mode. Defaults to `prod`. Use `dev` for sort-of-pretty logging in debug level.
- `SCALYR_SERVER` (optional): URL to use for reporting logs. Defaults to `https://www.scalyr.com`, you can also use `https://eu.scalyr.com` for an european account
- `SCALYR_REQUEST_MAX_NB_EVENTS` (optional): Max number of events to send by request. Defaults to `20`
- `SCALYR_REQUEST_MAX_REQUEST_SIZE` (optional): Maximum size of a request. Defaults to `2097152` (2MB)
- `SCALYR_REQUEST_MIN_PERIOD` (optional): Minimum time between queries (mostly for testing, can also be used to reduce total bandwidth)
- `LOGSTASH_EVENT_MAX_SIZE` (optional): Maximum size of a logstash event. Defaults to `307200` (300 KB)
- `LOGSTASH_AUTH_KEY` (optional): Key to use for authentication. Not set by default
- `LOGSTASH_AUTH_VALUE` (optional): Value expected for the authentication key. Not set by default
- `QUEUE_SIZE` (optional): Buffering queue between logstash and scalyr. Defaults to `1000`

### Few implementation notes
Because each logstash TCP connection has its own logging session, you can easily separate them by filtering by the 
sessionId which is helpful during diagnostics.

We use an exponential backoff in case of errors.

### Default conversions
#### For messages
```json
{
    "@source_host": "hostname",
    "@source_path": "file_path",
    "@message":     "message",
    "@type":        "logstash_type",
    "@source":      "logstash_source",
    "@tags":        "tags"
}
```

#### For sessions
```json
{
    "appname": "serverHost",
    "env":     "logfile"
}
```

# Dependencies
The dependencies outside the standard library are:
- https://github.com/uber-go/zap
- github.com/kelseyhightower/envconfig
- github.com/satori/go.uuid

# Feedback
Any feedback is welcome.

# Possible evolutions
- Handling of scalyr's threads. I'm not entirely sure of how we could use them. It seems like the most similar concept
are the tags in a logstash world.

# Known issues
- Could be optimized (but probably handles tenths of megabytes per second)
- Some logstash fields might not be very well converted
- Some logstash fields might be transmitted in the session data to reduce the amount of data being sent
- Each connection can consume a lot of memory (roughly 300KB * 1000 = 300MB), but will likely consume a lot less in standard usage
- Needs some refactoring
- SSL isn't supported (easy to add)
- No clean shutdown: We should stop to accept clients and disconnect existing ones
- Only supports TCP. UDP wouldn't be difficult to setup but the sessionInfo mechanism would have to be handheld by other means

# License
MIT
