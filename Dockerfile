FROM alpine:3.8
EXPOSE 5050
ENTRYPOINT [ "/bin/logfwd" ]
COPY logfwd /bin/logfwd
