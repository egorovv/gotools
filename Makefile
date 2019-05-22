TOPDIR=../../..

include $(TOPDIR)/mk/top.mk

GOROOT ?= $(SDK)/staging_dir/host/go
GO ?= $(GOROOT)/bin/go

export GOPATH = $(abspath ..)

all::
	$(GO) get ./...
