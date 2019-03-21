FROM golang:alpine
MAINTAINER Daniel Olfelt <dolfelt@gmail.com>

RUN apk --no-cache add curl git make

ENV DEP_VERSION="0.5.0"
RUN curl -L -s https://github.com/golang/dep/releases/download/v${DEP_VERSION}/dep-linux-amd64 -o $GOPATH/bin/dep \
 && chmod +x $GOPATH/bin/dep

ENV APP_PATH=/go/src/github.com/dolfelt/copyql

WORKDIR $APP_PATH
COPY . $APP_PATH
RUN dep ensure

CMD ["go", "build"]
