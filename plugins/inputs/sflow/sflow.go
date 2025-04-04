package sflow

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"

	"github.com/shanas-swi/telegraf-v1.16.3"
	"github.com/shanas-swi/telegraf-v1.16.3/internal"
	"github.com/shanas-swi/telegraf-v1.16.3/plugins/inputs"
)

const sampleConfig = `
  ## Address to listen for sFlow packets.
  ##   example: service_address = "udp://:6343"
  ##            service_address = "udp4://:6343"
  ##            service_address = "udp6://:6343"
  service_address = "udp://:6343"

  ## Set the size of the operating system's receive buffer.
  ##   example: read_buffer_size = "64KiB"
  # read_buffer_size = ""
`

const (
	maxPacketSize = 64 * 1024
)

type SFlow struct {
	ServiceAddress string        `toml:"service_address"`
	ReadBufferSize internal.Size `toml:"read_buffer_size"`

	Log telegraf.Logger `toml:"-"`

	addr    net.Addr
	decoder *PacketDecoder
	closer  io.Closer
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// Description answers a description of this input plugin
func (s *SFlow) Description() string {
	return "SFlow V5 Protocol Listener"
}

// SampleConfig answers a sample configuration
func (s *SFlow) SampleConfig() string {
	return sampleConfig
}

func (s *SFlow) Init() error {
	s.decoder = NewDecoder()
	s.decoder.Log = s.Log
	return nil
}

// Start starts this sFlow listener listening on the configured network for sFlow packets
func (s *SFlow) Start(acc telegraf.Accumulator) error {
	s.decoder.OnPacket(func(p *V5Format) {
		metrics, err := makeMetrics(p)
		if err != nil {
			s.Log.Errorf("Failed to make metric from packet: %s", err)
			return
		}
		for _, m := range metrics {
			acc.AddMetric(m)
		}
	})

	u, err := url.Parse(s.ServiceAddress)
	if err != nil {
		return err
	}

	conn, err := listenUDP(u.Scheme, u.Host)
	if err != nil {
		return err
	}
	s.closer = conn
	s.addr = conn.LocalAddr()

	if s.ReadBufferSize.Size > 0 {
		conn.SetReadBuffer(int(s.ReadBufferSize.Size))
	}

	s.Log.Infof("Listening on %s://%s", s.addr.Network(), s.addr.String())

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.read(acc, conn)
	}()

	return nil
}

// Gather is a NOOP for sFlow as it receives, asynchronously, sFlow network packets
func (s *SFlow) Gather(_ telegraf.Accumulator) error {
	return nil
}

func (s *SFlow) Stop() {
	if s.closer != nil {
		s.closer.Close()
	}
	s.wg.Wait()
}

func (s *SFlow) Address() net.Addr {
	return s.addr
}

func (s *SFlow) read(acc telegraf.Accumulator, conn net.PacketConn) {
	buf := make([]byte, maxPacketSize)
	for {
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			if !strings.HasSuffix(err.Error(), ": use of closed network connection") {
				acc.AddError(err)
			}
			break
		}
		s.process(acc, buf[:n])
	}
}

func (s *SFlow) process(acc telegraf.Accumulator, buf []byte) {

	if err := s.decoder.Decode(bytes.NewBuffer(buf)); err != nil {
		acc.AddError(fmt.Errorf("unable to parse incoming packet: %s", err))
	}
}

func listenUDP(network string, address string) (*net.UDPConn, error) {
	switch network {
	case "udp", "udp4", "udp6":
		addr, err := net.ResolveUDPAddr(network, address)
		if err != nil {
			return nil, err
		}
		return net.ListenUDP(network, addr)
	default:
		return nil, fmt.Errorf("unsupported network type: %s", network)
	}
}

// init registers this SFlow input plug in with the Telegraf framework
func init() {
	inputs.Add("sflow", func() telegraf.Input {
		return &SFlow{}
	})
}
