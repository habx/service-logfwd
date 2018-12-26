FROM alpine:3.8
EXPOSE 5050
ENTRYPOINT [ "/usr/bin/logfwd" ]
COPY logfwd /usr/bin/logfwd
