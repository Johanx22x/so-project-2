package models

type Peer struct {
	Host string `json:"host"`
	Port string `json:"port"`
}

func NewPeer(host, port string) *Peer {
	return &Peer{
		Host: host,
		Port: port,
	}
}
