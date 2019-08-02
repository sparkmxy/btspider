package collect

import (
	"bytes"
	"encoding/binary"
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

func sendHandShake(conn net.Conn,query *)