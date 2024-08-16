FROM golang:bullseye AS build

WORKDIR /run
COPY ./main.go /run
COPY ./go.mod /run
COPY ./go.sum /run
COPY ./.env /run

RUN go build \
    && go install

FROM golang:bullseye
EXPOSE 9141
COPY --from=build /go/bin/apex_collector /go/bin/
ENTRYPOINT ["apex_collector"]