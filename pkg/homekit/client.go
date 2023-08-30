package homekit

import (
	"encoding/json"
	"errors"
	"math/rand"
	"net"
	"net/url"
	"time"

	"github.com/AlexxIT/go2rtc/pkg/core"
	"github.com/AlexxIT/go2rtc/pkg/hap"
	"github.com/AlexxIT/go2rtc/pkg/hap/camera"
	"github.com/AlexxIT/go2rtc/pkg/srtp"
	"github.com/pion/rtp"
)

type Client struct {
	core.SuperProducer

	hap  *hap.Client
	srtp *srtp.Server

	videoConfig camera.SupportedVideoStreamConfig
	audioConfig camera.SupportedAudioStreamConfig

	videoSession *srtp.Session
	audioSession *srtp.Session

	stream *camera.Stream
}

func Dial(rawURL string, server *srtp.Server) (*Client, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	query := u.Query()
	conn := &hap.Client{
		DeviceAddress: u.Host,
		DeviceID:      query.Get("device_id"),
		DevicePublic:  hap.DecodeKey(query.Get("device_public")),
		ClientID:      query.Get("client_id"),
		ClientPrivate: hap.DecodeKey(query.Get("client_private")),
	}

	if err = conn.Dial(); err != nil {
		return nil, err
	}

	return &Client{hap: conn, srtp: server}, nil
}

func (c *Client) Conn() net.Conn {
	return c.hap.Conn
}

func (c *Client) GetMedias() []*core.Media {
	if c.Medias != nil {
		return c.Medias
	}

	acc, err := c.hap.GetFirstAccessory()
	if err != nil {
		return nil
	}

	char := acc.GetCharacter(camera.TypeSupportedVideoStreamConfiguration)
	if char == nil {
		return nil
	}
	if err = char.ReadTLV8(&c.videoConfig); err != nil {
		return nil
	}

	char = acc.GetCharacter(camera.TypeSupportedAudioStreamConfiguration)
	if char == nil {
		return nil
	}
	if err = char.ReadTLV8(&c.audioConfig); err != nil {
		return nil
	}

	c.Medias = []*core.Media{
		videoToMedia(c.videoConfig.Codecs),
		audioToMedia(c.audioConfig.Codecs),
	}

	return c.Medias
}

func (c *Client) Start() error {
	if c.Receivers == nil {
		return errors.New("producer without tracks")
	}

	if c.Receivers[0].Codec.Name == core.CodecJPEG {
		return c.startMJPEG()
	}

	videoTrack := c.trackByKind(core.KindVideo)
	videoCodec := trackToVideo(videoTrack, &c.videoConfig.Codecs[0])

	audioTrack := c.trackByKind(core.KindAudio)
	audioCodec := trackToAudio(audioTrack, &c.audioConfig.Codecs[0])

	c.videoSession = &srtp.Session{Local: c.srtpEndpoint()}
	c.audioSession = &srtp.Session{Local: c.srtpEndpoint()}

	var err error
	c.stream, err = camera.NewStream(c.hap, videoCodec, audioCodec, c.videoSession, c.audioSession)
	if err != nil {
		return err
	}

	c.srtp.AddSession(c.videoSession)
	c.srtp.AddSession(c.audioSession)

	deadline := time.NewTimer(core.ConnDeadline)

	if videoTrack != nil {
		c.videoSession.OnReadRTP = func(packet *rtp.Packet) {
			deadline.Reset(core.ConnDeadline)
			videoTrack.WriteRTP(packet)
		}

		if audioTrack != nil {
			c.audioSession.OnReadRTP = audioTrack.WriteRTP
		}
	} else {
		c.audioSession.OnReadRTP = func(packet *rtp.Packet) {
			deadline.Reset(core.ConnDeadline)
			audioTrack.WriteRTP(packet)
		}
	}

	<-deadline.C

	return nil
}

func (c *Client) Stop() error {
	_ = c.SuperProducer.Close()

	c.srtp.DelSession(c.videoSession)
	c.srtp.DelSession(c.audioSession)

	return c.hap.Close()
}

func (c *Client) MarshalJSON() ([]byte, error) {
	info := &core.Info{
		Type: "HomeKit active producer",
		URL:  c.hap.URL(),
		//SDP:       fmt.Sprintf("%+v", *c.config),
		Medias:    c.Medias,
		Receivers: c.Receivers,
		Recv:      c.videoSession.Recv + c.audioSession.Recv,
	}
	return json.Marshal(info)
}

func (c *Client) trackByKind(kind string) *core.Receiver {
	for _, receiver := range c.Receivers {
		if receiver.Codec.Kind() == kind {
			return receiver
		}
	}
	return nil
}

func (c *Client) startMJPEG() error {
	receiver := c.Receivers[0]

	for {
		b, err := c.hap.GetImage(1920, 1080)
		if err != nil {
			return err
		}

		packet := &rtp.Packet{
			Header:  rtp.Header{Timestamp: core.Now90000()},
			Payload: b,
		}
		receiver.WriteRTP(packet)
	}
}

func (c *Client) srtpEndpoint() *srtp.Endpoint {
	return &srtp.Endpoint{
		Addr:       c.hap.LocalIP(),
		Port:       uint16(c.srtp.Port()),
		MasterKey:  []byte(core.RandString(16, 0)),
		MasterSalt: []byte(core.RandString(14, 0)),
		SSRC:       rand.Uint32(),
	}
}

func limitter(handler core.HandlerFunc) core.HandlerFunc {
	const sampleRate = 16000
	const sampleSize = 480

	var send time.Duration
	var firstTime time.Time

	return func(packet *rtp.Packet) {
		now := time.Now()

		if send != 0 {
			elapsed := now.Sub(firstTime) * sampleRate / time.Second
			if send+sampleSize > elapsed {
				return // drop overflow frame
			}
		} else {
			firstTime = now
		}

		send += sampleSize

		packet.Timestamp = uint32(send)

		handler(packet)
	}
}
