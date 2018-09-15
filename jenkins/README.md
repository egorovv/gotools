# Jenkins downloader

## Build

This tool is written in Go - to build it you need a reasonably recent
Go toolchain.

In velocloud workspace - $VCROOT

```
export GOPATH=$VCROOT/dev/vadim/
cd $GOPATH/src/govco
go get ./...

```

If everything is fine this will result in a binary $GOPATH/bin/jenkins,
it should be reasonably self contained and usable on any reasonably
recent x86_64 Linux.

## Usage

```
jenkins
  -host (default "jenkins2.eng.velocloud.net")
  -job <job>
  -token <api token>
  -user <user>
  -build <build no>
  -files <file glog pattern - e.g. edge-imageupdate-EDGE5X0-*.zip>
```

The same options could be configured in ~/.jenkinsrc -n json format.


```
{
    "host": "jenkins2.eng.velocloud.net",
    "user": "vadim",
    "job":  "Release-3.3-VNF",
    "token" : "XXXX",
    "files" : "edge-imageupdate-EDGE5X0-*.zip"
}
```
