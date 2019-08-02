package collect


type Request struct {
	IP 			string
	Port		int
	InfoHash	string
	PeerID 		string
}

type metadataQuery struct {
	*Request
	result 		*Torrent
}