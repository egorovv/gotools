TOPDIR=../../../..

include $(TOPDIR)/mk/top.mk

GO ?= go

export GOPATH = $(abspath ../..)

all::
	$(GO) get ./...
