FROM golang:1.18

RUN go install github.com/cespare/reflex@v0.3.1


COPY reflex.conf /

ENTRYPOINT ["reflex", "-c", "/reflex.conf"]
