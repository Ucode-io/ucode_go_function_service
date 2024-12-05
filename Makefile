CURRENT_DIR=$(shell pwd)

APP=$(shell basename ${CURRENT_DIR})
APP_CMD_DIR=${CURRENT_DIR}/cmd

TAG=latest
ENV_TAG=latest
DOCKERFILE=Dockerfile

pull-proto-module:
	git submodule update --init --recursive

update-proto-module:
	git submodule update --remote --merge

copy-proto-module:
	rm -rf ${CURRENT_DIR}/protos
	rsync -rv --exclude=.git ${CURRENT_DIR}/ucode_protos/* ${CURRENT_DIR}/protos

gen-proto-module:
	sudo rm -rf ${CURRENT_DIR}/genproto
	./scripts/gen_proto.sh ${CURRENT_DIR}

build:
	CGO_ENABLED=0 GOOS=linux go build -mod=vendor -a -installsuffix cgo -o ${CURRENT_DIR}/bin/${APP} ${APP_CMD_DIR}/main.go

build-image:
	docker build --rm -t ${REGISTRY}/${PROJECT_NAME}/${APP}:${TAG} . -f ${DOCKERFILE}
	docker tag ${REGISTRY}/${PROJECT_NAME}/${APP}:${TAG} ${REGISTRY}/${PROJECT_NAME}/${APP}:${ENV_TAG}

push-image:
	docker push ${REGISTRY}/${PROJECT_NAME}/${APP}:${TAG}
	docker push ${REGISTRY}/${PROJECT_NAME}/${APP}:${ENV_TAG}

clear-image:
	docker rmi ${REGISTRY}/${PROJECT_NAME}/${APP}:${TAG}
	docker rmi ${REGISTRY}/${PROJECT_NAME}/${APP}:${ENV_TAG}

swag-init:
	swag init -g api/api.go -o api/docs --parseDependency --parseVendor
run:
	go run cmd/main.go

linter:
	golangci-lint run

# ! SHOULD CHANGE 
migration-up:
	migrate -path ./migrations/postgres -database 'postgres://postgres:admin1234@0.0.0.0:5432/udevs_go_auth_service?sslmode=disable' up
	# migrate -path ./migrations/postgres -database="postgres://auth_service:IeX7ieso@95.217.155.57:30034/auth_service?sslmode=disable&x-migrations-table=migrations_ucode_go_auth_service" up

# migration-down:
# 	migrate -path ./migrations/postgres -database 'postgres://postgres:qwerty123@0.0.0.0:5432/ucode_auth_service?sslmode=disable' down 

