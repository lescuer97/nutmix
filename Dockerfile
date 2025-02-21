FROM golang

RUN mkdir /app

ADD . /app

WORKDIR /app

RUN go install github.com/pressly/goose/v3/cmd/goose@latest
RUN go install github.com/a-h/templ/cmd/templ@latest
RUN templ generate .
RUN go build -o main cmd/nutmix/*.go


EXPOSE 8080
CMD [ "/app/main" ]
