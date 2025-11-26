FROM golang:alpine3.22

RUN apk add --no-cache protobuf

RUN mkdir /app

ADD . /app

WORKDIR /app

# Install protobuf generators
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.10
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

# Generate protobuf code
RUN protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative --experimental_allow_proto3_optional internal/gen/signer.proto

RUN go install github.com/a-h/templ/cmd/templ@v0.3.960
RUN templ generate .
RUN go build -o main cmd/nutmix/*.go


EXPOSE 8080
CMD [ "/app/main" ]
