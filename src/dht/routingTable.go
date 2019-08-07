package dht

import (
	"encoding/json"
	"log"
	"net"
	"os"
	"sort"
	"time"
)


type Kbucket struct{
	//k  				int
	LastUpdateTime 	time.Time
	Entry			[]*node
	Candidate		[]*node
}

type IDType [keySize/8]byte

func newKbucket() *Kbucket{
	return &Kbucket{
	//	k,
		time.Now(),
		make([]*node,0),
		make([]*node,0),
	}
}

func (this *Kbucket) updateTime(){
	this.LastUpdateTime = time.Now()
}


func (this *Kbucket)TryUpdate(o *node) bool{
	for i := range this.Entry{
		if this.Entry[i].id == o.id{
			copy(this.Entry[1:],this.Entry[:i])
			this.Entry[0] = o
			this.updateTime()
			return true
		}
	}
	return false
}

func (this *Kbucket) PushFront(o *node){
	this.Entry = append(this.Entry,nil)
	copy(this.Entry[1:],this.Entry)
	this.Entry[0]  = o
	this.updateTime()
}

/*type <Kbucket> Ends here*/

type Checker interface {
	Ping(*net.UDPAddr) error
}

type routingTable struct {

	k 			int
	id			IDType
	bucket		[]*Kbucket

	timeoutTimers	map[*node]*time.Timer
	pingTimers      map[*node]*time.Timer
	failed			map[*node]int
	knownNodes		map[IDType]*node

	pingEvent 		chan *node
	timeoutEvent  	chan *node
	closeEvent 		chan struct{}
	notifyEvent		chan *node

	checker 		Checker

	file            string
}

func getBucketID(a,b *IDType) int{
	if a == b{
		return 0
	}
	var bitDiff int
	for i,bit := range a {
		v := bit ^ b[i]
		switch {
		case v > 0x70:
			bitDiff = 8
			return i*8 + (8 - bitDiff)
		case v > 0x40:
			bitDiff = 7
			return i*8 + (8 - bitDiff)
		case v > 0x20:
			bitDiff = 6
			return i*8 + (8 - bitDiff)
		case v > 0x10:
			bitDiff = 5
			return i*8 + (8 - bitDiff)
		case v > 0x08:
			bitDiff = 4
			return i*8 + (8 - bitDiff)
		case v > 0x04:
			bitDiff = 3
			return i*8 + (8 - bitDiff)
		case v > 0x02:
			bitDiff = 2
			return i*8 + (8 - bitDiff)
		}
	}
	return -1
}
/*Functions about events*/
const (
	responseTimeout			= 1	 * time.Second
	durationToBackup		= 1  * time.Minute
	durationToPing			= 15 * time.Minute
)

func(this *routingTable) deletePingEvent(o *node){
	timer,ok := this.pingTimers[o]
	if ok {
		timer.Stop()
		delete(this.pingTimers,o)
	}
}

func(this *routingTable) deleteTimeoutEvent(o *node){
	timer,ok := this.timeoutTimers[o]
	if ok {
		timer.Stop()
		delete(this.timeoutTimers,o)
	}
}

func (this *routingTable) registerPingEvent(dur time.Duration,o *node){
	this.pingTimers[o] = time.AfterFunc(dur, func() {
		select {
			case this.pingEvent <- o:
			case <- this.closeEvent :
		}
	})
}

func (this *routingTable) registerTimeoutEvent(dur time.Duration,o *node){
	this.pingTimers[o] = time.AfterFunc(dur, func() {
		select {
		case this.timeoutEvent <- o:
		case <- this.closeEvent :
		}
	})
}

func (this *routingTable)notify(o *node){
	n := getBucketID(&this.id,&o.id)
	if n == -1{
		log.Println("invaild bucket id from",this.id,o.id)
		return
	}
	bkt := this.bucket[n]

	this.failed[o] = 0
	if bkt.TryUpdate(o){
		this.deleteTimeoutEvent(o)
		this.deletePingEvent(o)
		this.registerPingEvent(durationToPing,o)
	}else if len(bkt.Entry) < this.k{
		bkt.PushFront(o)
		this.registerPingEvent(durationToPing,o)
	}else {
		bkt.Candidate = append(bkt.Candidate,o)
		if len(bkt.Candidate) > this.k {
			copy(bkt.Candidate,bkt.Candidate[1:])
			bkt.Candidate = bkt.Candidate[:this.k]
		}
	}
}

func closer(target,A,B *IDType) int {
	for i := range target{
		a := A[i] ^ target[i]
		b := B[i] ^ target[i]
		if a > b{
			return 1
		}else if a < b{
			return -1
		}
	}
	return 0
}

// TODO: <load> <save> <routines>
func (this *routingTable) loadFromFile(filepath string) error {
	f, err := os.Open(filepath)
	if err != nil {
		return err
	}

	data := struct {
		id IDType
		Buckets []*Kbucket
	}{}

	err = json.NewDecoder(f).Decode(&data)
	if err != nil {
		return err
	}

	this.id = data.id
	for bktid := range data.Buckets {
		bkt := data.Buckets[bktid]
		if bkt != nil {
			for i := range bkt.Entry{
				this.Notify(bkt.Entry[i])
			}
		}
	}

	return nil
}

func (this *routingTable) saveToFile(filepath string) error {
	f, err := os.Create(filepath)
	if err != nil {
		return err
	}

	defer f.Close()

	data := make(map[string]interface{})
	data["OwnerID"] = this.id
	data["Buckets"] = this.bucket
	return json.NewEncoder(f).Encode(data)
}

func (this *routingTable) ping(o *node){
	this.checker.Ping(&o.addr)
	this.registerTimeoutEvent(responseTimeout,o)
}

func (this *routingTable) deleteNode(o *node){
	bkt := this.bucket[getBucketID(&this.id,&o.id)]
	i := 0
	for i < len(bkt.Entry){
		if bkt.Entry[i].id == o.id{
			bkt.Entry = append(bkt.Entry[:i],bkt.Entry[i+1:]...)
		}else {
			i++
		}
	}

	if len(bkt.Candidate) > 0 {
		l := len(bkt.Candidate) - 1
		bkt.PushFront(bkt.Candidate[l])
		this.registerPingEvent(durationToPing,bkt.Candidate[l])
		bkt.Candidate = bkt.Candidate[:l]
	}
}

func (this *routingTable) handleTimeoutNode(o *node){
	this.failed[o]++
	if this.failed[o] > 1{
		log.Println("delete node: ",o)
		delete(this.failed,o)
		this.deleteNode(o)
	}else{
		this.ping(o)
	}
}

func (this *routingTable) routine(){
	backupTicker := time.NewTicker(durationToBackup)
	defer backupTicker.Stop()
	for{
		select {
		case nodeToPing := <-this.pingEvent:
			log.Println("Wait to ping:",nodeToPing)
			this.ping(nodeToPing)

		case nodeTimeout := <-this.timeoutEvent:
			log .Println("node timeout : ",nodeTimeout)
			this.handleTimeoutNode(nodeTimeout)
		case o := <-this.notifyEvent:
			this.notify(o)
		case <- backupTicker.C:
			log.Println("routingTable backup.")
			this.saveToFile(this.file)
		}
	}
}
/*Public functions*/

func (this *routingTable) Notify(o *node) {
	log.Println("Notify: ",o.addr)
	if old,ok := this.knownNodes[o.id];ok{
		o = old
	}else{
		this.knownNodes[o.id] = o
	}

	select {
		case <- this.closeEvent:
		case this.notifyEvent <- o:
	}
}
func (this *routingTable) Stop(){
	close(this.closeEvent)
	this.saveToFile(this.file)
}

func (this *routingTable) ClosestNodes(target *IDType,N int) []node{
	ret := make([]node,0,N)
	for i:=0;i<keySize;i++{
		for _,cur := range this.bucket[i].Entry{
			length := len(ret)
			idx := sort.Search(length,func(i int) bool{
				return closer(target,&cur.id,&ret[i].id) < 0
			})
			if idx == length{
				if length < N {
					ret = append(ret, *cur)
				}
			}else {
				for j:= Min(N-1,length);j >idx; j--{
					ret[j] = ret[j-1]
				}
				ret[idx] = *cur
			}
		}
	}
	return ret
}

func NewRoutingTable(myid IDType,_k int,_checker Checker) *routingTable{

	ret := &routingTable{
		k:					_k,
		id:					myid,
		closeEvent: 		make(chan struct{}),
		timeoutEvent: 		make(chan *node),
		pingEvent: 			make(chan *node),
		notifyEvent:		make(chan *node),
		pingTimers: 		make(map[*node]*time.Timer),
		timeoutTimers: 		make(map[*node]*time.Timer),
		failed:				make(map[*node]int),
		knownNodes:			make(map[IDType]*node),
		checker:            _checker,
		file:				"data",
		bucket:            make([]*Kbucket,keySize),
	}
	for i:=0;i<keySize;i++{
		ret.bucket[i] = newKbucket()
	}
	go ret.routine()
	ret.loadFromFile(ret.file)
	return ret
}