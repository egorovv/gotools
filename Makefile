TOPDIR=../../../..

include $(TOPDIR)/mk/top.mk

GOROOT ?= $(SDK)/staging_dir/host/go
GO ?= $(GOROOT)/bin/go

export GOPATH = $(abspath ../..)

all::
	echo GOPATH=$(GOPATH)
	$(GO) get ./...
