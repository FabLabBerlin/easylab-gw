# EASY LAB Gateway Server

Lightweight IoT Gateway that can run in a Makerspace. It has been tested on Linux and OS X.

It communicates mostly via XMPP so it is secured through TLS. Configuration happens through conf/gateway.conf. Alternatively on OpenWRT UCI parameters are accepted as well.

## Getting Started

1. Install [Go](https://golang.org). You can download binaries from the
  [Go Download Page](https://golang.org/dl/). For OS X you can get the
  .pkg file. On Ubuntu you can just enter `sudo apt-get install golang-go`.
2. Set Go environment variables:
   `export GOROOT=/usr/local/go` and
   `export GOPATH=$HOME/go` (you may need `mkdir $HOME/go` beforehands)

   Most users add both lines to their `$HOME/.bash_profile` file.

3. Download Gateway and run it

```
	go get gopkg.in/gcfg.v1
	go get github.com/mattn/go-xmpp
	go get github.com/FabLabBerlin/easylab-lib
	go get github.com/FabLabBerlin/easylab-gw
	cd $GOPATH/src/github.com/FabLabBerlin/easylab-gw
	go build .
	./easylab-gw
```

Credit goes to [Datanoise](https://github.com/DatanoiseTV) for concept of OpenWRT integration and using XMPP which allows for lightweight TLS usage.
