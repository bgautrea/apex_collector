FROM golang:bullseye

#RUN mkdir /run



WORKDIR /run
COPY ./main.go /run
COPY ./go.mod /run
COPY ./go.sum /run
COPY ./.env /run

RUN go build

EXPOSE 9141

ENTRYPOINT ["go", "run", "main.go"]