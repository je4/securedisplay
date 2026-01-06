package proxy

import (
	"net"
	"time"

	"golang.org/x/net/ipv4"
)

func NewNTPConnection(
	Host string,
	Protocol string,
	Port string,
	LocalAddress string,
	TTL int,
	Timeout time.Duration) func(data []byte) ([]byte, error) {
	// Set a timeout on the connection.
	if Timeout == 0 {
		Timeout = 5 * time.Second
	}
	if Protocol == "" {
		Protocol = "udp"
	}
	if Port == "" {
		Port = "123"
	}

	return func(data []byte) ([]byte, error) {
		// Resolve the remote NTP server address.
		raddr, err := net.ResolveUDPAddr(Protocol, net.JoinHostPort(Host, Port))
		if err != nil {
			return nil, err
		}

		var laddr *net.UDPAddr
		if LocalAddress != "" {
			laddr, err = net.ResolveUDPAddr(Protocol, net.JoinHostPort(LocalAddress, "0"))
			if err != nil {
				return nil, err
			}
		}
		// Prepare a "connection" to the remote server.
		con, err := net.DialUDP(Protocol, laddr, raddr)
		if err != nil {
			return nil, err
		}
		defer con.Close()

		// Set a TTL for the packet if requested.
		if TTL != 0 {
			ipcon := ipv4.NewConn(con)
			err = ipcon.SetTTL(TTL)
			if err != nil {
				return nil, err
			}
		}

		con.SetDeadline(time.Now().Add(Timeout))

		// Transmit the query.
		_, err = con.Write(data)
		//err = binary.Write(con, binary.BigEndian, data)
		if err != nil {
			return nil, err
		}

		// Receive the response.
		recvMsg := make([]byte, len(data))
		_, err = con.Read(recvMsg)
		//err = binary.Read(con, binary.BigEndian, recvMsg)
		if err != nil {
			return nil, err
		}

		return recvMsg, nil
	}

}
