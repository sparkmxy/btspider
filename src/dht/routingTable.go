package dht

import (
	"container/list"
	"encoding/json"
	"log"
	"math/big"
	"net"
	"os"
	"sort"
	"time"
)


type Kbucket struct{
	k  				int
	lastUpdateTime 	time.Time
	entry			*list.List
	candidate		*list.List
}

type IDType [keySize/8]byte

func newKbucket(k int) *Kbucket{
	return &Kbucket{
		k,
		time.Now(),
		list.New(),
		list.New(),
	}
}

func (this *Kbucket) updateTime(){
	this.lastUpdateTime = time.Now()
}

func (this *Kbucket)TryUpdate(o *node) bool{
	for e := this.entry.Front(); e != nil; e = e.Next(){
		cur := e.Value.(*node)
		if cur.id == o.id{
			this.entry.Remove(e)
			this.entry.PushFront(o)
			this.updateTime()
			return true
		}
	}
	return false
}

func (this *Kbucket) PushFront(o *node){
	this.entry.PushFront(o)
	this.updateTime()
}

/*type <Kbucket> Ends here*/

type Checker interface {
	Ping(*net.UDPAddr) error
}

type routingTable struct {

	k 			int
	id			IDType
	bucket		[keySize]*Kbucket

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

var pow2 [keySize]big.Int

func getBucketID(a,b *IDType) int{
	A := new(big.Int).SetBytes(a[:])
	B := new(big.Int).SetBytes(b[:])
	dist := *new(big.Int).Xor(A,B)
	for i:=0;i<keySize;i++{
		if dist.Cmp(&pow2[i]) < 0 {
			return i
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
	}else if bkt.entry.Len() < this.k{
		bkt.PushFront(o)
		this.registerPingEvent(durationToPing,o)
	}else {
		bkt.candidate.PushFront(o)
		if bkt.candidate.Len() > this.k {
			bkt.candidate.Remove(bkt.candidate.Back())
		}
	}
}

func closer(target,A,B *IDType) bool{
	c := new(big.Int).SetBytes(target[:])
	a := new(big.Int).SetBytes(A[:])
	b := new(big.Int).SetBytes(B[:])
	d2 := new(big.Int).Xor(c,a)
	d1 := new(big.Int).Xor(c,b)
	return d1.Cmp(d2) < 0
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
			for e:=bkt.entry.Front();e!=nil;e=e.Next() {
				this.Notify(e.Value.(*node))
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
	for e:=bkt.entry.Front();e!=nil;e = e.Next(){
		cur := e.Value.(*node)
		if o.id == cur.id {
			bkt.entry.Remove(e)
			break;
		}
	}
	delete(this.failed,o)
	if bkt.candidate.Len() > 0{
		e := bkt.candidate.Front()
		bkt.entry.PushFront(e)
		bkt.candidate.Remove(e)
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
		for e:= this.bucket[i].entry.Front();e!=nil;e = e.Next(){
			cur := e.Value.(*node)
			length := len(ret)
			idx := sort.Search(length,func(i int) bool{
				return closer(target,&cur.id,&ret[i].id)
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
	two := big.NewInt(2)
	for i:=0;i<keySize;i++{
		pow2[i] = *new(big.Int).Exp(two,big.NewInt(int64(i)),nil)
	}
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
	}
	for i:=0;i<keySize;i++{
		ret.bucket[i] = newKbucket(_k)
	}
	go ret.routine()
	ret.loadFromFile(ret.file)
	return ret
}