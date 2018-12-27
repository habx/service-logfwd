# Log forward

## Goal
Using scalyr and datadog logging platforms as drop-in replacement for your previous logstash architecture.

## Testing it
You can start a logstash server that will redirect its stream both on datadog and scalyr servers at the same time with
the following command:
```bash
docker run --rm -p 5050:5050 -e DATADOG_TOKEN=... -e SCALYR_TOKEN=... habx/logfwd:dev 
```
At least one of the two is necessary.

## Why
Scalyr and datadog push you to use their agent to do logging but:
- Scalyr agent sucks, it consumes a LOT of CPU and RAM
- Datadog agent costs a LOT ($15/server)

Logstash has logging clients in all languages, it's good compromise between simplicity and performance.

## How it works

### General overview
For each logstash client connected to the server, it will create independent logging sessions, wether it's for sclayr
or datadog.

Each logstash event is parsed, converted to basic logging event, then passed to all the output clients that are
enabled.

### Performed translations
To easily re-use an existing logstash implementation, a few tricks are needed:

- `@timestamp` is converted to the scalyr's ts field
- `@message` becomes the `message` attributes
- `@fields` are brought moved to the root attribute properties (as scalyr doesn't support nested search)
- `level` / `levelname` fields are converted to the scalyr's `sev` fields
- `source` is set to `logfwd`

### Env vars
Everything is handled through environment variables

#### General
- `LISTEN_ADDR` (optional): Port to listen on. Defaults to `:5050`
- `LOG_ENV` (optional): Logging mode. Defaults to `prod`. Use `dev` for sort-of-pretty logging in debug level.
- `LOGSTASH_EVENT_MAX_SIZE` (optional): Maximum size of a logstash event. Defaults to `307200` (300 KB)
- `LOGSTASH_AUTH_KEY` (optional): Key to use for authentication. Not set by default
- `LOGSTASH_AUTH_VALUE` (optional): Value expected for the authentication key. Not set by default

#### Scalyr
- `SCALYR_WRITELOG_TOKEN` (enables it) : Your scalyr log write token
- `SCALYR_FIELDS_CONV_MESSAGE` (optional): Conversion to apply between logstash and scalyr event attributes
- `SCALYR_FIELDS_CONV_SESSION` (optional): Conversion to apply between logstash events and scalyr log session attributes
- `SCALYR_SERVER` (optional): URL to use for reporting logs. Defaults to `https://www.scalyr.com`, you can also use `https://eu.scalyr.com` for an european account
- `SCALYR_REQUEST_MAX_NB_EVENTS` (optional): Max number of events to send by request. Defaults to `20`
- `SCALYR_REQUEST_MAX_REQUEST_SIZE` (optional): Maximum size of a request. Defaults to `2097152` (2MB)
- `SCALYR_REQUEST_MIN_PERIOD` (optional): Minimum time between queries (mostly for testing, can also be used to reduce total bandwidth)
- `SCALYR_QUEUE_SIZE` (optional): Buffering queue between logstash and scalyr. Defaults to `1000`

#### Datadog
- `DATADOG_TOKEN` (enables it) : Your datadog token
- `DATADOG_SERVER` (optiona): Datadog server. Defaults to `intake.logs.datadoghq.com:15506`, use `tcp-intake.logs.datadoghq.eu:443` for europe
- `DATADOG_QUEUESIZE` (optional): Queue size. Defautls to `20`. As it's a TCP to TCP stream, it can be kept to a low value
- `DATADOG_FIELDS_CONV_MESSAGE` (optioanl): Conversion of message fields
- `DATADOG_FIELDS_CONV_TAGS` (optional): Conversion of message fields to tags

### Default scalyr conversion
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

- [zap](https://github.com/uber-go/zap) for logs
- [envconfig](github.com/kelseyhightower/envconfig) for config management through environemnt variables
- [go.uuid](github.com/satori/go.uuid) for scalyr sessions UUID generation

# Feedback
Any feedback is welcome.

# Possible evolutions
- Handling of scalyr's threads. I'm not entirely sure of how we could use them.

# Known issues
- Could be optimized (but probably handles tenths of megabytes per second)
- Some logstash fields might not be very well converted
- Some logstash fields might be transmitted in the session data to reduce the amount of data being sent
- There's not a single unit tests
- Each connection can consume a lot of memory (roughly 300KB * 1000 = 300MB), but will likely consume a lot less in standard usage
- Needs some refactoring
- SSL isn't supported (easy to add)
- No clean shutdown: We should stop to accept clients and disconnect existing ones
- Only supports TCP. UDP wouldn't be difficult to setup but the sessionInfo mechanism would have to be handheld by other means

# License
MIT
