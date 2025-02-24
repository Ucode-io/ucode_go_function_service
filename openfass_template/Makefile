CURRENT_DIR := $(shell pwd)
FUNCTION_PATH := $(shell basename ${CURRENT_DIR})
PREFIX := gitlab.udevs.io:5050/ucode_functions_group/${FUNCTION_PATH}
GATEWAY := https://ofs.u-code.io
YML_FILE := ${FUNCTION_PATH}.yml

gen-function:
	faas-cli new ${FUNCTION_PATH} --lang golang-middleware --prefix ${PREFIX} --gateway ${GATEWAY}
	make add-row

add-row:
	@if ! grep -q "constraints:" $(YML_FILE); then \
		echo "    constraints:" >> $(YML_FILE); \
		echo "    - \"workload=openfaas-fn\"" >> $(YML_FILE); \
	else \
		echo "   constraints already exist in $(YML_FILE)"; \
	fi