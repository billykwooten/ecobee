FROM golang

RUN go get github.com/dichro/ecobee

RUN go install github.com/dichro/ecobee

ENTRYPOINT [ "/go/bin/ecobee", "--app_id", "NF4GPNseYoJ5CoosM5eEycP19zpHm4ut", "--cache_file", "/db/auth.cache", "--port", "8080", "--logtostderr" ]

EXPOSE 8080
