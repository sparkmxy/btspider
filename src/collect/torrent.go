package collect

import (
	"bencode"
	"encoding/hex"
	"errors"
)

type Torrent struct {
	rawData		interface{}

	Announce 	string  		//A string pointing to the tracker
	Files 		[]TorrentFile
	Length		int64	//文件的大小
	Name		string  //name of root file or folder
	PieceLength  int64   //整数,是BitTorrent文件块的大小.
	Pieces		string	//连续的存放着所有块的SHA1杂凑值,每一个文件块的杂凑值为20字节.
}

type TorrentFile struct {
	Length		int64		//当前文件的大小
	Path 		string		//由字符串组成的列表,每个列表元素指一个路径名中的一个目录或文件名.比如说:"l3:abc3:abc:6abc.txte",指文件路径"abc/abc/abc.txt".
}

func NewTorrent(data []byte) (*Torrent,error){
	metadata,err := bencode.Unmarshal(data)
	if err != nil{
		return nil,err
	}

	ret := Torrent{}

	ret.rawData = map[string]interface{}{
		"info" : metadata,
	}
	Map,ok := ret.rawData.(map[string]interface{})    //A dictionary

	if !ok{
		return nil,errors.New("Invalid Metadata")
	}

	if announce, ok := Map["announce"].([]byte); ok{
		ret.Announce = string(announce)
	}

	if info, ok := Map["info"].(map[string]interface{}); ok{
		// name of root folder
		if name,ok := info["name"].([]byte); ok {
			ret.Name = string(name)
		}
		// size of per piece
		if pieceLen, ok := info["piece length"].(int64); ok{
			ret.PieceLength = pieceLen
		}
		// SHA-1 hash value of all peices
		if pieces, ok := info["pieces"].([]byte); ok {
			ret.Pieces = hex.EncodeToString(pieces)
		}
		// files
		if files, ok := info["files"].([]interface{}); ok {
			fileN := len(files)
			ret.Files = make([]TorrentFile,0,fileN)
			for i:=0 ;i<fileN;i++{
				if file,ok := files[i].(map[string]interface{}); ok {
					File := TorrentFile{}
					if Len, ok := file["length"].(int64); ok {
						File.Length = Len
					}
					if path,ok := file["path"].([]interface{}); ok{
						File.Path = string(path[0].([]byte))
						ret.Files = append(ret.Files,File)
					}
				}
			}
		}
	}
	return &ret,nil
}