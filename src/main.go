
package main

import (
	"dht"
	"log"
)

func handlePeer(ip string,port int,infohash,peerid string){
	log.Println("new announce_peer query", infohash)
}

func main(){
	dhtnode := dht.NewNode()
	dhtnode.Create("random","0.0.0.0:8661",handlePeer)
	dhtnode.Run()
}