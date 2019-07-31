package bencode


func Marshal(data interface{}) ([]byte,error){
	result := make([]byte,512)
	length,_,err := marshal(data,&result,0,512)
	if err != nil{
		return nil, err
	}
	return result[:length],nil
}

func marshal(data interface{}, result *[]byte, offset,length int)(int,int,error){

}
