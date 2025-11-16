.PHONY: default
default:

# host targets

.PHONY: docker
docker:
	docker/build.sh

.PHONY: clean
clean:
	@rm -rf build

# container targets

.PHONY: all
all:
