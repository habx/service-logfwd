FROM alpine:3.8
EXPOSE 5050
RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*
ENTRYPOINT [ "/usr/bin/logfwd" ]
COPY logfwd /usr/bin/logfwd
