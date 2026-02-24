# Makefile for Chaos Load Balancer RPM build

NAME := chaoslb
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//')
ifeq ($(VERSION),)
  VERSION := unknown
endif
RELEASE := 1
ARCH := x86_64

TOPDIR := $(shell pwd)/rpmbuild
SOURCEDIR := $(TOPDIR)/SOURCES
SPECDIR := $(TOPDIR)/SPECS
RPMDIR := $(TOPDIR)/RPMS
SRPMDIR := $(TOPDIR)/SRPMS

.PHONY: all clean build build-rpm install-deps help prepare-dirs

all: help

help:
	@echo "Available targets:"
	@echo "  all               Show this help (default)"
	@echo "  help              Show this help"
	@echo "  build             Build Go binary only"
	@echo "  build-rpm         Build RPM package"
	@echo "  install-deps      Install build dependencies"
	@echo "  clean             Clean build artifacts"
	@echo "  help              Show this help"
     
install-deps:
	@echo "Installing RPM build dependencies..."
	sudo yum install -y rpm-build rpm-devel libtool gcc golang
	go mod download

TARGET_DIR := target

build:
	@echo "Building Go binary..."
	@mkdir -p $(TARGET_DIR)
	CGO_ENABLED=0 go build -ldflags="-w -s -X main.version=$(VERSION)" -o $(TARGET_DIR)/$(NAME) ./cmd/$(NAME)
	@if command -v upx >/dev/null; then \
		echo "Compressing binary with UPX..."; \
		upx --best $(TARGET_DIR)/$(NAME); \
	fi

prepare-dirs:
	@echo "Creating RPM build directories..."
	mkdir -p $(TOPDIR)/{BUILD,RPMS,SOURCES,SPECS,SRPMS}
	mkdir -p $(TOPDIR)/RPMS/{noarch,x86_64,i386}

prepare-source: prepare-dirs
	@echo "Preparing source package..."
	mkdir -p $(NAME)-$(VERSION)
	cp -r cmd pkg go.mod go.sum $(NAME)-$(VERSION)/
	cp chaoslb.service config.json README.md LICENSE $(NAME)-$(VERSION)/
	tar czf $(SOURCEDIR)/$(NAME)-$(VERSION).tar.gz $(NAME)-$(VERSION)
	cp $(NAME).spec $(SPECDIR)/
	rm -rf $(NAME)-$(VERSION)
	@echo "Creating RPM macros file..."
	echo '%_missing_build_ids_terminate_build 0' > ~/.rpmmacros
	echo '%_build_id_links none' >> ~/.rpmmacros
	echo '%debug_package %{nil}' >> ~/.rpmmacros

build-rpm: prepare-source
	@echo "Building RPM package..."
	rpmbuild --define "_topdir $(TOPDIR)" -ba $(SPECDIR)/$(NAME).spec
	@echo "RPM build completed!"
	@echo "Location: $(RPMDIR)/$(ARCH)/"
	ls -la $(RPMDIR)/$(ARCH)/

install-rpm: build-rpm
	@echo "Installing RPM package..."
	sudo rpm -Uvh $(RPMDIR)/$(ARCH)/$(NAME)-$(VERSION)-$(RELEASE).*.$(ARCH).rpm

clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(TOPDIR)
	rm -rf $(TARGET_DIR)
	rm -f *.rpm

dev: build
	@echo "Development build completed: ./$(NAME)"
