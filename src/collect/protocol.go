package collect

import (
	"bencode"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"time"
)


const (
	writeTimeLimit = 1 * time.Second
	readTimeLimit  = 2 * time.Second
)
func writePacket(conn net.Conn,data []byte) error{
	conn.SetWriteDeadline(time.Now().Add(writeTimeLimit))

	l , err := conn.Write(data)
	if err!=nil{
		return err
	}
	if l != len(data){
		return errors.New("write error")
	}

	return nil
}

func readPacket(conn net.Conn,buf *bytes.Buffer,size int64) error{
	conn.SetReadDeadline(time.Now().Add(readTimeLimit))

	l,err := io.CopyN(buf,conn,size)

	if err != nil{
		return err
	}

	if l != size{
		return errors.New("read error")
	}
	return nil
}

func readMessage(conn net.Conn,buf *bytes.Buffer)(int,int,error){
	err := readPacket(conn,buf,4)
	if err != nil{
		return 0,0,err
	}
	l := int64(binary.BigEndian.Uint32(buf.Next(4)))

	if l == 0{
		return 0,0,errors.New("read message error")
	}

	err = readPacket(conn,buf,l)
	if err != nil{
		return 0,0,err
	}
	temp,err := buf.ReadByte()
	if err != nil{
		return 0,0,err
	}
	st := int(temp)

	temp,err = buf.ReadByte()
	if err != nil{
		return 0,0,err
	}

	ed := int(temp)

	return st,ed,nil
}


/*
Handshake format(total length = 68 bytes):
character#19(lenth prefix) + "BitTorrent protocol"(length = 19) + 00000000(reserverd for future use) +
infohash(length = 20) + peedID(length = 20)
*/

func sendHandShake(conn net.Conn,query *metadataQuery) error{
	packet := make([]byte,68)
	packet[0] = 19
	copy(packet[1:20],"BitTorrent protocol")
	packet[25] = 0x10

	infohash,_ := hex.DecodeString(query.InfoHash)
	peer,_ := hex.DecodeString(query.PeerID)

	copy(packet[28:48],infohash)
	copy(packet[48:68],peer)

	return writePacket(conn,packet)
}

func receiveHandShake(conn net.Conn,buf *bytes.Buffer) error{
	return readPacket(conn,buf,68)
}

/*Extention message bep_0009*/
const (
	requestType = 0
	dataType 	= 1
	rejectType  = 2
)


func writePacket2(conn net.Conn, data []byte, begin,end int) error {
	length := len(data) + 2
	packet := make([]byte, 4+length)
	binary.BigEndian.PutUint32(packet[:4], uint32(length))
	packet[4] = byte(begin)
	packet[5] = byte(end)
	copy(packet[6:], data)

	return writePacket(conn, packet)
}

func sendRequest(conn net.Conn,metaInfo int, pieceID int) error{
	msg,_ := bencode.Marshal(map[string]interface{}{
		"msg_type":  requestType,
		"piece":		pieceID,
	})
	return writePacket2(conn,msg,20,metaInfo)
}

func extractPieceInfo(data []byte) (Type,pieceID,size,offset int,err error) {
	temp,err := bencode.Unmarshal(data)
	if err != nil{
		return
	}
	Map := temp.(map[string]interface{})
	Type = int(Map["msg_type"].(int64))
	pieceID = int(Map["piece"].(int64))

	bcode,_ := bencode.Marshal(temp)
	offset = len(bcode)
	return
}


/*From bep10*/
func sendHandshakeExtended(conn net.Conn,query *metadataQuery) error  {
	Map := map[string]interface{}{
		"m": map[string]interface{}{
			"ut_metadata": 0,
		},
	}

	msg,_ := bencode.Marshal(Map)

	return writePacket2(conn,msg,20,0)
}


func receiveHandshakeExtended(conn net.Conn,buffer *bytes.Buffer)(int64,int64,error){
	_,_,err := readMessage(conn,buffer)
	if buffer.Len() == 0{
		err = errors.New("read error")
	}
	if err != nil{
		return 0,0,err
	}

	temp,err := bencode.Unmarshal(buffer.Bytes())

	if err != nil{
		return 0,0,err
	}

	Map := temp.(map[string]interface{})
	size,ok := Map["metadata_size"]

	if !ok{
		err = errors.New("no metadata_size")
		return 0,0,err
	}

	utMetadata := Map["m"].(map[string]interface{})["ut_metadata"].(int64)

	return utMetadata,size.(int64),nil
}