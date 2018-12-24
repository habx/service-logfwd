# Logstash 2 scalyr

## Goal
This tool allows to use scalyr logging platform as drop-in replacement for your previous logging architecture. 
Switching your previous logs to it should be a matter of minutes.

## Why
Scalyr is a very convenient logging solution but it clearly lacks some third-party integrations.

Logstash has logging clients in all languages, it's good compromise between simplicity and performance.

The scalyr agent, at least in kubernetes, consumes a lot of CPU and memory. So I quickly discarded it.

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

- `LS2S_TOKEN` (mandatory) : Your log write token
- `LS2S_URL` (optional): URL to use for reporting logs. Defaults to `https://www.scalyr.com/addEvents`, you can also use `https://eu.scalyr.com/addEvents` for an european account
- `LS2S_MAXLINESIZE` (optional): Maximum size of each logstash line being parsed. Defaults to `307200` (300 KB)
- `LS2S_LISTENADDR` (optional): Port to listen on. Defaults to `:5050`
- `LS2S_LOGENV` (optional): Logging mode. Defaults to `prod`. Use `dev` for sort-of-pretty logging in debug level,

### Few implementation notes
Because each logstash TCP connection has its own logging session, you can easily separate them by filtering by the 
sessionId which is helpful during diagnostics.

We use an exponential backoff in case of errors. It's definitely not properly setup.

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
