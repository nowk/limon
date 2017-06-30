VERSION=0.0.1

default: $(VERSION)

$(VERSION): clean
	@docker-compose run --rm \
		-e GO15VENDOREXPERIMENT=1 \
		-e CGO_ENABLED=0 \
		-e GOOS=linux \
		go build -ldflags "-s" -o limon -a -installsuffix cgo main.go

clean:
	@sudo rm -f limon
.PHONY: clean


NAME=nowk/limon
TAG=$(VERSION)

build:
	@docker build --rm --no-cache -t $(NAME):$(TAG) .
.PHONY: build

push:
	@docker push $(NAME):$(TAG)
.PHONY: push


all:
	@$(MAKE) -s
	@$(MAKE) -s build
.PHONY: all

