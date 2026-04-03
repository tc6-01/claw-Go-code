FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /claw ./cmd/claw

FROM alpine:3.21

RUN apk add --no-cache ca-certificates git bash

COPY --from=builder /claw /usr/local/bin/claw

RUN adduser -D -u 1000 claw
USER claw

WORKDIR /home/claw

RUN mkdir -p /home/claw/.claw/data /home/claw/.claw/sessions /home/claw/.claw/skills

EXPOSE 8080

ENTRYPOINT ["claw"]
CMD ["serve"]
