FROM golang:1.18-alpine as builder

RUN apk update && apk upgrade && apk add git && apk add bash && apk add ca-certificates

WORKDIR /image-validation-admission-controller

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags='-w -s' -o /go/bin/image-validation-admission-controller cmd/main.go

# Runtime Image
FROM scratch
COPY --from=builder /go/bin/image-validation-admission-controller /bin/image-validation-admission-controller
ENTRYPOINT [ "/bin/image-validation-admission-controller" ]