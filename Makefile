# RedisCli — GNU Make，macOS 与 Windows（Git Bash / scoop make）均可
# MyGo 无 cgo，可从任意主机交叉编译

APP      ?= RedisCli
OUT      ?= build
MYGO     ?= go tool mygo
GO       ?= go
NCPU     ?= $(shell nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4)
HOST_OS  := $(shell $(GO) env GOOS)

# -H windowsgui：Windows 子系统为 GUI，双击不弹出 cmd 黑框
LDFLAGS_WIN := -H windowsgui -s -w
LDFLAGS_NIX := -s -w

ifeq ($(HOST_OS),windows)
  BIN := $(OUT)/$(APP).exe
else
  BIN := $(OUT)/$(APP)
endif

.DEFAULT_GOAL := help

help:
	@echo "RedisCli"
	@echo "  make dev          热重载开发 (mygo dev)"
	@echo "  make test         跑测试"
	@echo "  make build        按当前系统打生产包 (mygo)"
	@echo "  make mac          打 macOS universal (.app / .dmg)"
	@echo "  make windows      打 Windows amd64+arm64"
	@echo "  make all          mac + windows"
	@echo "  make bin          当前系统裸二进制 (快，无安装包)"
	@echo "  make bin-mac      交叉编译 macOS arm64+amd64 二进制"
	@echo "  make bin-windows  交叉编译 Windows amd64+arm64 二进制"
	@echo "  make run          编译当前系统并启动"
	@echo "  make clean        清理 build / .mygo"

dev:
	$(MYGO) dev

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

# 当前机器的生产包：mac 为 .app/.dmg，Windows 为 exe/安装包
build:
	$(MYGO) build -o $(OUT)

mac:
	$(MYGO) build -platform darwin/universal -o $(OUT)

windows:
	$(MYGO) build -platform windows/amd64,windows/arm64 -o $(OUT)

all: mac windows

# 裸二进制，不走 mygo 打包，交叉编译更快
bin:
	@mkdir -p "$(OUT)"
ifeq ($(HOST_OS),windows)
	CGO_ENABLED=0 $(GO) build -p $(NCPU) -ldflags="$(LDFLAGS_WIN)" -o "$(BIN)" .
else
	CGO_ENABLED=0 $(GO) build -p $(NCPU) -ldflags="$(LDFLAGS_NIX)" -o "$(BIN)" .
endif
	@echo "已生成 $(BIN)"

bin-mac:
	@mkdir -p "$(OUT)"
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build -p $(NCPU) -ldflags="$(LDFLAGS_NIX)" -o "$(OUT)/$(APP)-darwin-arm64" .
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build -p $(NCPU) -ldflags="$(LDFLAGS_NIX)" -o "$(OUT)/$(APP)-darwin-amd64" .
	@echo "已生成 $(OUT)/$(APP)-darwin-arm64  $(OUT)/$(APP)-darwin-amd64"

bin-windows:
	@mkdir -p "$(OUT)"
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -p $(NCPU) -ldflags="$(LDFLAGS_WIN)" -o "$(OUT)/$(APP)-amd64.exe" .
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 $(GO) build -p $(NCPU) -ldflags="$(LDFLAGS_WIN)" -o "$(OUT)/$(APP)-arm64.exe" .
	@echo "已生成 $(OUT)/$(APP)-amd64.exe  $(OUT)/$(APP)-arm64.exe"

run: bin
	"$(BIN)"

# Git Bash / macOS / Linux 都有 rm；Windows cmd 请用 Git Bash 或手动删 build
clean:
	rm -rf $(OUT) .mygo
	rm -f $(APP) $(APP).exe

.PHONY: help dev test vet build mac windows all bin bin-mac bin-windows run clean
