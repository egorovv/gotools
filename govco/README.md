# Go VCO - device settings push simulator.

## Build

This tool is written in Go - to build it you need a reasonably recent
Go toolchain.

In velocloud workspace - $VCROOT

```
export GOPATH=$VCROOT/dev/vadim/
cd $GOPATH/src/govco
go get ./...

```

If everything is fine this will result in a binary $GOPATH/bin/govco,
it should be reasonably self contained and usable on any reasonably
recent x86_64 Linux.

## How it works

GoVCO leverages an RPC server built into MGD. The RPC endpoint is
bound to localhost so to access it the tool establishes a TCP
port-forwarding channel over an SSH connection. Of course Edge device
needs to be configured to accept SSH connections.

After the channel is established GoVCO sens getConfiguration RPC and
obtains the device serrings from the edge. Then it replaces the
portion of the settings with custom json blob and sends it back via
updateConfiguration RPC. This emulates receiving of the VCO heartbeat
response with updated settings.

Ta-da.

If the device settings version is not changed the VCO never notices -
othervise it will push the original

The custom json replaces the portion of the original settings using
recursive merge - scalar values and arrays in the new settings replace
ones in the old, the object values are merged recursively.


## Usage

```
govco --host <edge> --user --password --file <json>
```

If public-key authentication is set up the `~/.ssh/id_rsa` whill be
used for authentication.


Sample json:

```
{
    "management": {
        "cidrIp": "10.0.0.2",
        "cidrPrefix": 32
    }
}

```
