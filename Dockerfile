FROM golang:alpine
MAINTAINER Daniel Olfelt <dolfelt@gmail.com>

ENV GLIDE_VERSION v0.12.3
ENV GLIDE_DOWNLOAD_URL https://github.com/Masterminds/glide/releases/download/$GLIDE_VERSION/glide-$GLIDE_VERSION-linux-amd64.zip

RUN apk --no-cache add curl git make
RUN curl -fsSL "$GLIDE_DOWNLOAD_URL" -o glide.zip \
	&& unzip glide.zip  linux-amd64/glide \
	&& mv linux-amd64/glide /usr/local/bin \
	&& rm -rf linux-amd64 \
	&& rm glide.zip

ENV APP_PATH=/go/src/github.com/dolfelt/copyql

WORKDIR $APP_PATH
COPY . $APP_PATH
RUN glide install

CMD ["go", "build"]
