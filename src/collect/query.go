package collect

import (
	"bufio"
	"bytes"
	"debug/dwarf"
	"errors"
	"io/ioutil"
	"log"
	"net"
	"strconv"
	"time"
)

type Request struct {
	IP 			string
	Port		int
	InfoHash	string
	PeerID 		string
}

func (this *Request) Address() string{
	return this.IP + ":" + strconv.Itoa(this.Port)
}

var TimeoutError = errors.New("dial timeout")

type metadataQuery struct {
	*Request
	result 		*Torrent

	err 		error

	done 		chan struct{}
}


func newMetadataQuery (request *Request) *metadataQuery{
	return &metadataQuery{
		Request: request,
		done:    make(chan struct{}),
	}
}

func (this *metadataQuery) work(collector *Collector){
	this.err = this.getMetadata()
	select {
	case <-collector.closeEvent:
	case collector.HandleQueryEvent <- struct{}{}:
	}
	if this.err != nil{
		this.handleError()
	}else {
		this.handleMetadata()
	}

	close(this.done)
}

func (this *metadataQuery)handleError()  {

}

func (this *metadataQuery)handleMetadata(){

}

const(
	dialTimeout = 10 * time.Second
	getPieceTimeout = 20 * time.Second
	BLOCKSIZE int64 = 16384
	maxPieceN = 10000
)

func (this *metadataQuery) getMetadata()error{
	defer func() {
		if err := recover(); err != nil{
			log.Println("query error")
			this.err = errors.New("query error")
		}
	}()

	conn,err := net.DialTimeout("tcp",this.Address(),dialTimeout)
	if err != nil{
		return err
	}
	tcp := conn.(*net.TCPConn)
	tcp.SetLinger(0)
	defer tcp.Close()

	if err := sendHandShake(conn,this); err != nil{
		return err
	}

	buffer := new(bytes.Buffer)

	if err := receiveHandShake(conn,buffer); err != nil{
		return err
	}
	buffer.Reset()

	if err := sendHandshakeExtended(conn,this); err != nil{
		return err
	}

	utMetadata,size,err := receiveHandshakeExtended(conn,buffer)
	if err != nil{
		return err
	}

	N := size / BLOCKSIZE

	if size % BLOCKSIZE != 0 {
		N++
	}

	if N > maxPieceN{
		return errors.New("too many pieces")
	}
	pieces := make([][]byte,int(N))

getPieces:
	for{
		select {
		case <-time.After(getPieceTimeout):
			return TimeoutError
		default:
			buffer.Reset()

			begin, _ , err := readMessage(conn,buffer)

			if err != nil{
				return err
			}
			if begin != 20{
				buffer.Reset()
				continue
			}

			Type,pieceID,_,offset,err := extractPieceInfo(buffer.Bytes())

			if err != nil{
				return err
			}

			if Type == rejectType{
				return errors.New("rejected")
			}else if Type == dataType{
				piece , _ := ioutil.ReadAll(buffer)
				pieces[pieceID] = piece[offset:]

				if int64(len(piece[offset:])) < BLOCKSIZE{
					break getPieces
				}
			}
		}
	}

	temp := bytes.Join(pieces,nil)
	this.result,err = NewTorrent(temp)
	return err
}