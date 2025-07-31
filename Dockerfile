FROM golang

# Install protoc
RUN apt-get update && apt-get install -y protobuf-compiler && rm -rf /var/lib/apt/lists/*

RUN mkdir /app

ADD . /app

WORKDIR /app

# Install protobuf generators
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Generate protobuf code
RUN protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative --experimental_allow_proto3_optional internal/gen/signer.proto

RUN go install github.com/pressly/goose/v3/cmd/goose@latest
RUN go install github.com/a-h/templ/cmd/templ@latest
RUN templ generate .
RUN go build -o main cmd/nutmix/*.go


EXPOSE 8080
CMD [ "/app/main" ]
