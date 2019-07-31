package dht

import (
	"encoding/hex"
	"errors"
	"log"
	"net"
	"time"
)

func (this *DHTNode) writeToUDP(addr *net.UDPAddr,data []byte) error{
	this.udpconn.SetWriteDeadline(time.Now().Add(5*time.Second))
	n,err := this.udpconn.WriteToUDP(data,addr)
	if err != nil{
		log.Println("writeToUDP: ",err)
		return err
	}
	if n != len(data){
		return errors.New("writeToUDP: partial failed.")
	}
	return nil
}

func (this *DHTNode) readUDP(conn *net.UDPConn) error{
	msg := make([]byte,8192)
	for{
		n,address,err := conn.ReadFromUDP(msg)
		if err != nil{
			return err
		}
		go this.handleKRPCPacket(address,msg[:n])

	}
}

/*Query handlers*/
func (this *DHTNode) handlePing(query *KRPCQuery,address *net.UDPAddr){
	response := KRPCResponse{
		transactionID: 	GenerateToken(),
		Type:			PingType,
		queryID:        this.id,
	}

	data,err := response.Encode()
	if err != nil{
		log.Println("Handling ping: ",err)
	}
	_ = this.writeToUDP(address,data)
}

func (this *DHTNode) handleFindNode(query *KRPCQuery,address *net.UDPAddr){
	response := KRPCResponse{
		transactionID: 	query.transactionID,
		Type: 			FindNodeType,
		queryID: 		this.id,
		nodes: 			make([]*node,0),
	}
	data,err := response.Encode()
	if err != nil{
		log.Println("Handling FindNode: ",err)
	}
	_ = this.writeToUDP(address,data)
}

func (this *DHTNode) handleGetPeers(query *KRPCQuery,address *net.UDPAddr){
	tempID := generateID()
	response := KRPCResponse{
		transactionID: 	query.transactionID,
		Type: 			GetPeersType,
		queryID:		tempID,
		token:			string(query.infoHash[:2]),
		nodes:			make([]*node,0),
	}
	data,err := response.Encode()
	if err != nil{
		log.Println("Handling FindNode: ",err)
	}
	_ = this.writeToUDP(address,data)
}

func (this *DHTNode) handleAnounce(query *KRPCQuery,address *net.UDPAddr){
	port := query.port
	if query.impliedPort == 1{
		port = address.Port
	}

	this.PeerHandler(address.IP.String(),port,
		hex.EncodeToString(query.infoHash),
		hex.EncodeToString(query.queryingID[:]),
	)

	response := KRPCResponse{
		transactionID: 	query.transactionID,
		Type: 			AnnoucePeerType,
		queryID:		this.id,
	}
	data,err := response.Encode()
	if err != nil{
		log.Println("Handling ping: ",err)
	}
	_ = this.writeToUDP(address,data)
}

/*Functions to make requests*/
func (this *DHTNode) Ping(addr *net.UDPAddr) error{
	request := KRPCQuery{
		transactionID: 	GenerateToken(),
		Type:			PingType,
		id:				this.id,
	}

	data,err := request.Encode()
	if err != nil{
		return err
	}
	return this.writeToUDP(addr,data)
}

func (this *DHTNode) FindNode(address *net.UDPAddr, targetID IDType) {
	request := KRPCQuery{
		transactionID: 	GenerateToken(),
		Type: 			FindNodeType,
		id:				this.id,
		queryingID:		targetID,
	}

	msg,err := request.Encode()
	if err != nil{
		log.Println(err)
	}
	_ = this.writeToUDP(address,msg)
}

func (this *DHTNode) GetPeers(address *net.UDPAddr, infohash []byte) {
	tempid := generateID()
	request := KRPCQuery{
		transactionID:		GenerateToken(),
		id:					this.id,
		infoHash:   		tempid[:],
	}

	msg,err := request.Encode()
	if err != nil{
		log.Println(err)
	}
	_ = this.writeToUDP(address,msg)
}