prepare:
	docker build --pull -t $${BUILD_IMAGE:-copyql-builder} .

build:
	for GOOS in $${GOOS_LIST:-darwin linux}; do \
		for GOARCH in $${GOARCH_LIST:-amd64 386}; do \
			echo "Building $$GOOS on $$GOARCH"; \
			GOOS=$$GOOS GOARCH=$$GOARCH go build -o copyql-$$GOOS-$$GOARCH ; \
		done \
	done

build-mac:
	export GOOS_LIST=darwin; make build

build-docker:
	docker run -e GOOS_LIST="$${GOOS_LIST:=darwin linux}" -v $$PWD:/go/src/github.com/dolfelt/copyql --name $${PACKAGE_NAME:=copyql-builder} $${BUILD_IMAGE:-copyql-builder} /bin/sh -c 'make build'; \
	CONTAINER_ID=$$(docker ps -aqf "name=$${PACKAGE_NAME:=copyql-builder}"); \
		for GOOS in $${GOOS_LIST:-darwin linux}; do \
			for GOARCH in $${GOARCH_LIST:-amd64 386}; do \
				docker cp $$CONTAINER_ID:/go/src/github.com/dolfelt/copyql/copyql-$$GOOS-$$GOARCH copyql-$$GOOS-$$GOARCH ; \
				chmod +x copyql-$$GOOS-$$GOARCH; \
			done ; \
		done ; \
		docker rm $$CONTAINER_ID
