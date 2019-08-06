package dht

import (
	"bencode"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
)

const (
	PingType = "ping"
	FindNodeType = "find_node"
	GetPeersType = "get_peers"
	AnnoucePeerType = "announce_peer"
)

type node struct {
	id 					IDType
	addr 				net.UDPAddr
}

type KRPCMessage struct {
	t string
	y string
	content map[string]interface{}
}


func decodeMessage(data []byte)(*KRPCMessage,error){
	// decode a message from bencode form.
	temp, err := bencode.Unmarshal(data)
	if err != nil{
		return nil,err
	}

	msg := new(KRPCMessage)
	content := temp.(map[string]interface{})
	if t,ok := content["t"].([]byte); ok {
		msg.t = string(t)
	}
	if y,ok := content["y"].([]byte); ok{
		msg.y = string(y)
	}
	msg.content = content  // why?
	return msg,err
}

func (this *KRPCMessage) isQuery() bool {
	return this.y == "q"
}


func (this *KRPCMessage) isResponse() bool {
	return this.y == "r"
}

func (this *KRPCMessage) isError() bool {
	return this.y == "e"
}

type KRPCResponse struct {
	transactionID 	[]byte    	// transaction ID
	Type 			string   	// type
	queryID 		IDType
	token 			string
	nodes 			[]*node
	values			[]string
}


func (this *KRPCResponse) LoadFromMap(data map[string]interface{}) error{
	this.transactionID = data["t"].([]byte)
	data = data["r"].(map[string]interface{})
	if queryID,ok := data["token"]; ok {
		copy(this.queryID[:],queryID.([]byte)[:])
	}
	if token,ok := data["token"]; ok{
		this.token = string(token.([]byte))
	}
	if nodes,ok := data["nodes"]; ok{
		this.nodes = decodeCompactNodes(nodes.([]byte))
	}
	if values,ok := data["values"]; ok{
		vector := values.([]interface{})
		for _,value := range  vector{
			this.values = append(this.values,string(value.([]byte)))
		}
	}
	return nil
}

func (this *KRPCResponse) Encode() ([]byte,error){
	data := map[string]interface{}{
		"t" : this.transactionID,
		"y" : []byte("r"),
	}

	switch this.Type {
	case PingType:
		data["r"] = map[string]interface{}{
			"id" : this.queryID[:],
		}
	case FindNodeType:
		data["r"] = map[string]interface{}{
			"id" : this.queryID[:],
			"nodes" : encodeCompactForm(this.nodes),
		}
	case GetPeersType:
		data["r"] = map[string]interface{}{
			"id" : this.queryID[:],
			"token" : this.token,
			"nodes" : encodeCompactForm(this.nodes),
		}
	case AnnoucePeerType:
		data["r"] = map[string]interface{}{
			"id" : this.queryID[:],
		}
	default:
		return nil,errors.New("Unkown type.")
	}
	return bencode.Marshal(data)
}

type KRPCQuery struct {
	transactionID		[]byte
	Type    			string

	id 					IDType
	queryingID     		IDType
	infoHash  			[]byte
	impliedPort    		int8
	port				int
	token   			string
}

func (this *KRPCQuery) LoadFormMap(data map[string]interface{}) error{
	if t, ok := data["t"]; ok {
		this.transactionID = t.([]byte)
	}
	if qtype, ok := data["q"]; ok{
		this.Type = string(qtype.(string))
	}

	data = data["a"].(map[string]interface{})

	if nodeID, ok := data["id"]; ok{
		copy(this.id[:],nodeID.([]byte))
	}

	if target, ok := data["target"]; ok{
		copy(this.queryingID[:],target.([]byte))
	}
	if info, ok := data["infohash"]; ok {
		this.infoHash = info.([]byte)
	}
	if impliedPort, ok := data["implied_port"]; ok {
		this.impliedPort = int8(impliedPort.(int64))
	}

	if port, ok := data["port"]; ok {
		this.port = int(port.(int64))
	}

	if token, ok := data["token"]; ok {
		this.token =  string(token.([]byte))
	}
	return nil
}

func (this *KRPCQuery) Encode()([]byte,error){
	data := map[string]interface{}{
		"t" : this.transactionID,
		"q" : []byte(this.Type),
		"y" : []byte("q"),
	}
	switch this.Type {
	case PingType:
		data["a"] = map[string]interface{}{
			"id" : this.id[:],
		}
	case FindNodeType:
		data["a"] = map[string]interface{}{
			"id" : this.id[:],
			"target" : this.queryingID[:],
		}
	case GetPeersType:
		data["a"] = map[string]interface{}{
			"id" : this.id[:],
			"infohash" : this.infoHash,
		}
	case AnnoucePeerType:
		data["a"] = map[string]interface{}{
			"id" : this.id[:],
			"implied_port" : this.impliedPort,
			"infohash" : this.infoHash,
			"port" : this.port,
			"token" : this.token,
		}
	default:
		return nil,errors.New("Unknown type.")
	}
	return bencode.Marshal(data)
}

const (
	keySize = 160
	compactNodeSize = 26
)

func encodeCompactForm (nodes []*node)[]byte{
	data := make([]byte,0,compactNodeSize * len(nodes))
	ipbuf := make([]byte,4)
	portbuf := make([]byte,2)
	for _,node := range nodes{
		if len(node.addr.IP) == net.IPv4len{
			ipbuf = []byte(node.addr.IP)
		}else if len(node.addr.IP) == net.IPv6len{
			ipbuf = []byte(node.addr.IP[12:16])
		}else{
			continue
		}
		binary.LittleEndian.PutUint16(portbuf,uint16(node.addr.Port))
		data = append(data,node.id[:]...)
		data = append(data,ipbuf...)
		data = append(data,portbuf...)
	}
	return data
}

func decodeCompactNodes(data []byte) []*node {
	l := len(data)
	if l % compactNodeSize != 0{
		return nil
	}

	ret := make([]*node,0,l/compactNodeSize)
	for i:=0;i<l; i+= compactNodeSize{
		var id IDType
		copy(id[:],data[i:i+20])
		o := &node{
			id : id,
			addr : net.UDPAddr{
				IP : net.IPv4(data[i+20],data[i+21],data[i+22],data[i+23]),
				Port : int(binary.LittleEndian.Uint16(data[i+24:i+26])),
			},
		}
		ret = append(ret, o)
	}
	return ret
}

/*ErrorType*/

var(
	KRPCErrGeneric = newError(201, "A Generic Error Ocurred")
	KRPCErrServer = newError(201, "A Server Error Ocurred")
	KPRCErrProtocol = newError(203, "A Protocol Error Ocurred")
	KPRCErrMalformedPacket = newError(203, "A Protocol Error Ocurred")
	KRPCErrMethodUnknown = newError(204, "Method Unknown")
)

type ErrorType struct {
	code	int
	msg		string
}

func (err *ErrorType) Error() string{
	return err.msg
}

func newError(code int,str string) error{
	return &ErrorType{code,fmt.Sprintf("Error<%d>: %s",code,str)}
}

/*Other Functions*/
func Min(x,y int) int {
	if x < y {
		return x
	}
	return y
}