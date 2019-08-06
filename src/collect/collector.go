package collect


const maxpendendingN = 5000

type Collector struct {
	closeEvent 			chan struct{}
	queryEvent			chan *metadataQuery
	HandleQueryEvent 	chan struct{}
}


func NewCollector() *Collector{

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
