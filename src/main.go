
package main

import (
	"collect"
	"dht"
)

var (
	dhtnode = dht.NewNode()
	collector = collect.NewCollector()
)

func handlePeer(ip string,port int,infohash,peerid string){
	if err := collector.Get(&collect.Request{
			IP: 		ip,
			Port:		port,
			InfoHash:	infohash,
			PeerID:		peerid,
		});err != nil {
		panic(err)
	}
}

func main(){
	defer collector.Stop()
	dhtnode.Create("random","0.0.0.0:8661",handlePeer)
	dhtnode.Run()
}