package collect

import "errors"

const maxpendendingN = 5000

type Collector struct {
	closeEvent 			chan struct{}
	queryEvent			chan *metadataQuery
	HandleQueryEvent 	chan struct{}
}


func NewCollector() *Collector{
	ret := &Collector{
		make(chan struct{}),
		make(chan *metadataQuery),
		make(chan struct{}),
	}

	go ret.work()

	return ret
}


func (this *Collector) work(){
	pending := 0
	
	for{
		if pending >= maxpendendingN{
			select {
			case <- this.closeEvent:
			case <- this.HandleQueryEvent:
				pending--
			}
		}else{
			select {
			case <- this.closeEvent:
			case query := <- this.queryEvent:
				pending++
				go query.work(this)
			case <- this.HandleQueryEvent:
				pending--
			}
		}
	}
}

func (this *Collector)Stop(){
	close(this.closeEvent)
	close(this.HandleQueryEvent)
	close(this.queryEvent)
}

func (this *Collector) Get(request *Request)  error{
	query := newMetadataQuery(request)

	select {
	case this.queryEvent <- query:
	default:
		return errors.New("too many queries")
	}

	return nil
}