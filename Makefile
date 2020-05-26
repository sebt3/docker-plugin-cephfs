.PHONY: build clean plugin push

PLUGIN_NAME = sebt3/docker-plugin-cephfs
PLUGIN_TAG ?= master

plugin: build
	docker plugin rm --force ${PLUGIN_NAME}:${PLUGIN_TAG} || true
	docker plugin create ${PLUGIN_NAME}:${PLUGIN_TAG} build

clean:
	rm -rf ./build

build: clean
	docker rmi --force ${PLUGIN_NAME}:rfs || true
	docker build --quiet --tag ${PLUGIN_NAME}:rfs .
	docker create --name tmp ${PLUGIN_NAME}:rfs sh
	mkdir -p build/rootfs
	docker export tmp | tar -x -C build/rootfs/
	cp config.json ./build/
	docker rm -vf tmp

push: plugin
	docker plugin push ${PLUGIN_NAME}:${PLUGIN_TAG}
