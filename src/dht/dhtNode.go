package dht

import (
	"encoding/hex"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"os/signal"
	"time"
)

var bootstrapNodes = []string{
	"router.bittorrent.com:6881",
	"dht.transmissionbt.com:6881",
	"router.utorrent.com:6881",
	"dht.libtorrent.org:25401",
}

var random = rand.New(rand.NewSource(19260817))

func generateID() IDType{
	temp := make([]byte,20)
	random.Read(temp)
	var ret IDType
	copy(ret[:],temp[:])
	return ret
}

func GenerateToken() []byte{
	buf := make([]byte,2)
	random.Read(buf)
	return buf
}

type DHTNode struct {
	node
	localAddress 	net.UDPAddr
	RT 				*routingTable
	udpconn 		*net.UDPConn

	running 		bool

	findNodeEvent 	chan *node
	quitEvent		chan struct{}

	PeerHandler  func(ip string, port int, infoHash, peerID string)
}

func NewNode() *DHTNode{
	ret := &DHTNode{
		node: 					node{},
		// routingTable will be initialzed in Create()
		running: 				false,
		findNodeEvent:			make(chan *node),
		quitEvent: 				make(chan struct{}),
	}

	return ret
}

func (this *DHTNode)handleKRPCPacket(address *net.UDPAddr, raw []byte){
	defer func(){
		if err := recover(); err != nil{
			log.Println("recover: ",err)
		}
	}()

	msg,err := decodeMessage(raw)   // msg is a <KRPCMessage> type, data is store in <msg.content> as a map.
	if err != nil{
		log.Println("decodeKRPCMessge: ",err)
	}

	if msg.isQuery(){
		Q := new(KRPCQuery)
		Q.LoadFormMap(msg.content)

		this.RT.Notify(&node{
			Q.id,
			*address,
		})
		switch Q.Type {
		case PingType:
			this.handlePing(Q,address)
		case FindNodeType:
			this.handleFindNode(Q,address)
		case GetPeersType:
			this.handleGetPeers(Q,address)
		case AnnoucePeerType:
			this.handleAnounce(Q,address)
		}
	}else if msg.isResponse(){
		R := new(KRPCResponse)
		R.LoadFromMap(msg.content)

		log.Println("get response: from ",R.queryID)

		this.RT.Notify(&node{
			R.queryID,
			*address,
		})

		if len(R.nodes) > 0 {
			for _,o := range R.nodes{
				this.findNodeEvent <- o
			}
		}
	}else{
		log.Println(msg)
	}
}

func (this *DHTNode) listenConsole() {
	ch := make(chan  os.Signal)
	signal.Notify(ch,os.Interrupt)
	<-ch

	log.Println("Quit...")
	close(this.quitEvent)
	time.Sleep(500 * time.Millisecond)
	close(this.findNodeEvent)
	this.RT.Stop()

	this.running = false
	_ = this.udpconn.Close()
}

func (this *DHTNode) Create(ID,addrString string,
	F func(ip string, port int, infoHash, peerID string))  {
	if ID == "random"{
		this.id = generateID()
	}else{
		id,err := hex.DecodeString(ID)
		if err !=nil || len(id) != 20{
			panic("Invalid ID.")
		}
		copy(this.id[:],id[:])
	}

	address,err := net.ResolveUDPAddr("udp",addrString)
	if err != nil{
		panic(err)
	}
	this.localAddress = *address
	this.RT = NewRoutingTable(this.id,8,this)
	this.PeerHandler = F
}

func (this *DHTNode) Join(){
	ticker := time.NewTicker(2*time.Second)
	defer ticker.Stop()

	for{
		id := generateID()
		select {
		case <-this.quitEvent:
			return
		case o := <-this.findNodeEvent:
			this.FindNode(&o.addr,id)
		case <- ticker.C:
			neighbors := this.RT.ClosestNodes(&id,8)

			for _,o := range neighbors{
				this.FindNode(&o.addr,id)
			}
		}
	}
}

func (this *DHTNode) Serve() error{
	conn,err := net.ListenUDP("udp",&this.localAddress)
	if err !=nil{
		return err
	}
	go func() {
		err := this.readUDP(conn)
		log.Println("readUDP: ",err)

	}()
	this.udpconn = conn
	return nil
}

func (this *DHTNode) Run(){
	this.running = true

	log.Println("nodeinfo = ",this.node.toString())
	if err :=this.Serve();err !=nil{
		log.Println("Serve udp error: ",err)
	}

	go this.Join()

	for i := range bootstrapNodes{
		udpAddr, err := net.ResolveUDPAddr("udp",bootstrapNodes[i])
		if err != nil || udpAddr == nil{
			log.Println(err)
			continue
		}
		fmt.Println(udpAddr)
		this.FindNode(udpAddr,generateID())
	}

	log.Println("Join the DHT network...")

	this.listenConsole()
}

func (this *node) toString()  string {
	return fmt.Sprintf("<node-info id:%s, address:%s>",hex.EncodeToString(this.id[:]),this.addr.String())
}