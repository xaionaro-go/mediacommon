//nolint:dupl
package mpegts

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/asticode/go-astits"
	"github.com/stretchr/testify/require"

	"github.com/bluenviron/mediacommon/pkg/codecs/h264"
	"github.com/bluenviron/mediacommon/pkg/codecs/mpeg4audio"
)

func durationGoToMPEGTS(d time.Duration) int64 {
	return int64(d.Seconds() * 90000)
}

func TestWriter(t *testing.T) {
	testSPS := []byte{
		0x67, 0x42, 0xc0, 0x28, 0xd9, 0x00, 0x78, 0x02,
		0x27, 0xe5, 0x84, 0x00, 0x00, 0x03, 0x00, 0x04,
		0x00, 0x00, 0x03, 0x00, 0xf0, 0x3c, 0x60, 0xc9,
		0x20,
	}

	h264Track := &Track{
		PID:   256,
		Codec: &CodecH264{},
	}

	aacTrack := &Track{
		PID: 257,
		Codec: &CodecMPEG4Audio{
			Config: mpeg4audio.Config{
				Type:         2,
				SampleRate:   44100,
				ChannelCount: 2,
			},
		},
	}

	type videoSample struct {
		AU  [][]byte
		PTS time.Duration
		DTS time.Duration
	}

	type audioSample struct {
		AU  []byte
		PTS time.Duration
	}

	type sample interface{}

	testSamples := []sample{
		videoSample{
			AU: [][]byte{
				testSPS, // SPS
				{8},     // PPS
				{5},     // IDR
			},
			PTS: 2 * time.Second,
			DTS: 2 * time.Second,
		},
		audioSample{
			AU: []byte{
				0x01, 0x02, 0x03, 0x04,
			},
			PTS: 3 * time.Second,
		},
		audioSample{
			AU: []byte{
				0x01, 0x02, 0x03, 0x04,
			},
			PTS: 3500 * time.Millisecond,
		},
		videoSample{
			AU: [][]byte{
				{1}, // non-IDR
			},
			PTS: 4 * time.Second,
			DTS: 4 * time.Second,
		},
		audioSample{
			AU: []byte{
				0x01, 0x02, 0x03, 0x04,
			},
			PTS: 4500 * time.Millisecond,
		},
		videoSample{
			AU: [][]byte{
				{1}, // non-IDR
			},
			PTS: 6 * time.Second,
			DTS: 6 * time.Second,
		},
	}

	t.Run("h264 + aac", func(t *testing.T) {
		var buf bytes.Buffer
		w := NewWriter(&buf, []*Track{h264Track, aacTrack})

		for _, sample := range testSamples {
			switch tsample := sample.(type) {
			case videoSample:
				err := w.WriteH26x(
					h264Track,
					durationGoToMPEGTS(tsample.PTS),
					durationGoToMPEGTS(tsample.DTS),
					h264.IDRPresent(tsample.AU),
					tsample.AU)
				require.NoError(t, err)

			case audioSample:
				err := w.WriteMPEG4Audio(
					aacTrack,
					durationGoToMPEGTS(tsample.PTS),
					[][]byte{tsample.AU})
				require.NoError(t, err)
			}
		}

		dem := astits.NewDemuxer(
			context.Background(),
			bytes.NewReader(buf.Bytes()),
			astits.DemuxerOptPacketSize(188))

		// PMT
		pkt, err := dem.NextPacket()
		require.NoError(t, err)
		require.Equal(t, &astits.Packet{
			Header: astits.PacketHeader{
				HasPayload:                true,
				PayloadUnitStartIndicator: true,
				PID:                       0,
			},
			Payload: append([]byte{
				0x00, 0x00, 0xb0, 0x0d, 0x00, 0x00, 0xc1, 0x00,
				0x00, 0x00, 0x01, 0xf0, 0x00, 0x71, 0x10, 0xd8,
				0x78,
			}, bytes.Repeat([]byte{0xff}, 167)...),
		}, pkt)

		// PAT
		pkt, err = dem.NextPacket()
		require.NoError(t, err)
		require.Equal(t, &astits.Packet{
			Header: astits.PacketHeader{
				HasPayload:                true,
				PayloadUnitStartIndicator: true,
				PID:                       4096,
			},
			Payload: append([]byte{
				0x00, 0x02, 0xb0, 0x17, 0x00, 0x01, 0xc1, 0x00,
				0x00, 0xe1, 0x00, 0xf0, 0x00, 0x1b, 0xe1, 0x00,
				0xf0, 0x00, 0x0f, 0xe1, 0x01, 0xf0, 0x00, 0x2f,
				0x44, 0xb9, 0x9b,
			}, bytes.Repeat([]byte{0xff}, 157)...),
		}, pkt)

		// PES (H264)
		pkt, err = dem.NextPacket()
		require.NoError(t, err)
		require.Equal(t, &astits.Packet{
			AdaptationField: &astits.PacketAdaptationField{
				Length:                130,
				StuffingLength:        129,
				RandomAccessIndicator: true,
			},
			Header: astits.PacketHeader{
				HasAdaptationField:        true,
				HasPayload:                true,
				PayloadUnitStartIndicator: true,
				PID:                       256,
			},
			Payload: []byte{
				0x00, 0x00, 0x01, 0xe0, 0x00, 0x00, 0x80, 0x80,
				0x05, 0x21, 0x00, 0x0b, 0x7e, 0x41, 0x00, 0x00,
				0x00, 0x01,
				0x67, 0x42, 0xc0, 0x28, 0xd9, 0x00, 0x78, 0x02,
				0x27, 0xe5, 0x84, 0x00, 0x00, 0x03, 0x00, 0x04,
				0x00, 0x00, 0x03, 0x00, 0xf0, 0x3c, 0x60, 0xc9,
				0x20, 0x00, 0x00, 0x00, 0x01, 0x08, 0x00, 0x00,
				0x00, 0x01, 0x05,
			},
		}, pkt)

		// PES (AAC)
		pkt, err = dem.NextPacket()
		require.NoError(t, err)
		require.Equal(t, &astits.Packet{
			AdaptationField: &astits.PacketAdaptationField{
				Length:                158,
				StuffingLength:        157,
				RandomAccessIndicator: true,
			},
			Header: astits.PacketHeader{
				HasAdaptationField:        true,
				HasPayload:                true,
				PayloadUnitStartIndicator: true,
				PID:                       257,
			},
			Payload: []byte{
				0x00, 0x00, 0x01, 0xc0, 0x00, 0x13, 0x80, 0x80,
				0x05, 0x21, 0x00, 0x11, 0x3d, 0x61, 0xff, 0xf1,
				0x50, 0x80, 0x01, 0x7f, 0xfc, 0x01, 0x02, 0x03,
				0x04,
			},
		}, pkt)
	})

	t.Run("h264", func(t *testing.T) {
		var buf bytes.Buffer
		w := NewWriter(&buf, []*Track{h264Track})

		for _, sample := range testSamples {
			if tsample, ok := sample.(videoSample); ok {
				err := w.WriteH26x(
					h264Track,
					durationGoToMPEGTS(tsample.PTS),
					durationGoToMPEGTS(tsample.DTS),
					h264.IDRPresent(tsample.AU),
					tsample.AU)
				require.NoError(t, err)
			}
		}

		dem := astits.NewDemuxer(
			context.Background(),
			bytes.NewReader(buf.Bytes()),
			astits.DemuxerOptPacketSize(188))

		// PMT
		pkt, err := dem.NextPacket()
		require.NoError(t, err)
		require.Equal(t, &astits.Packet{
			Header: astits.PacketHeader{
				HasPayload:                true,
				PayloadUnitStartIndicator: true,
				PID:                       0,
			},
			Payload: append([]byte{
				0x00, 0x00, 0xb0, 0x0d, 0x00, 0x00, 0xc1, 0x00,
				0x00, 0x00, 0x01, 0xf0, 0x00, 0x71, 0x10, 0xd8,
				0x78,
			}, bytes.Repeat([]byte{0xff}, 167)...),
		}, pkt)

		// PAT
		pkt, err = dem.NextPacket()
		require.NoError(t, err)
		require.Equal(t, &astits.Packet{
			Header: astits.PacketHeader{
				HasPayload:                true,
				PayloadUnitStartIndicator: true,
				PID:                       4096,
			},
			Payload: append([]byte{
				0x00, 0x02, 0xb0, 0x12, 0x00, 0x01, 0xc1, 0x00,
				0x00, 0xe1, 0x00, 0xf0, 0x00, 0x1b, 0xe1, 0x00,
				0xf0, 0x00, 0x15, 0xbd, 0x4d, 0x56,
			}, bytes.Repeat([]byte{0xff}, 162)...),
		}, pkt)

		// PES (H264)
		pkt, err = dem.NextPacket()
		require.NoError(t, err)
		require.Equal(t, &astits.Packet{
			AdaptationField: &astits.PacketAdaptationField{
				Length:                130,
				StuffingLength:        129,
				RandomAccessIndicator: true,
			},
			Header: astits.PacketHeader{
				HasAdaptationField:        true,
				HasPayload:                true,
				PayloadUnitStartIndicator: true,
				PID:                       256,
			},
			Payload: []byte{
				0x00, 0x00, 0x01, 0xe0, 0x00, 0x00, 0x80, 0x80,
				0x05, 0x21, 0x00, 0x0b, 0x7e, 0x41, 0x00, 0x00,
				0x00, 0x01,
				0x67, 0x42, 0xc0, 0x28, 0xd9, 0x00, 0x78, 0x02,
				0x27, 0xe5, 0x84, 0x00, 0x00, 0x03, 0x00, 0x04,
				0x00, 0x00, 0x03, 0x00, 0xf0, 0x3c, 0x60, 0xc9,
				0x20, 0x00, 0x00, 0x00, 0x01, 0x08, 0x00, 0x00,
				0x00, 0x01, 0x05,
			},
		}, pkt)
	})

	t.Run("mpeg4-audio", func(t *testing.T) {
		var buf bytes.Buffer
		w := NewWriter(&buf, []*Track{aacTrack})

		for _, sample := range testSamples {
			if tsample, ok := sample.(audioSample); ok {
				err := w.WriteMPEG4Audio(
					aacTrack,
					durationGoToMPEGTS(tsample.PTS),
					[][]byte{tsample.AU})
				require.NoError(t, err)
			}
		}

		dem := astits.NewDemuxer(
			context.Background(),
			bytes.NewReader(buf.Bytes()),
			astits.DemuxerOptPacketSize(188))

		// PMT
		pkt, err := dem.NextPacket()
		require.NoError(t, err)
		require.Equal(t, &astits.Packet{
			Header: astits.PacketHeader{
				HasPayload:                true,
				PayloadUnitStartIndicator: true,
				PID:                       0,
			},
			Payload: append([]byte{
				0x00, 0x00, 0xb0, 0x0d, 0x00, 0x00, 0xc1, 0x00,
				0x00, 0x00, 0x01, 0xf0, 0x00, 0x71, 0x10, 0xd8,
				0x78,
			}, bytes.Repeat([]byte{0xff}, 167)...),
		}, pkt)

		// PAT
		pkt, err = dem.NextPacket()
		require.NoError(t, err)
		require.Equal(t, &astits.Packet{
			Header: astits.PacketHeader{
				HasPayload:                true,
				PayloadUnitStartIndicator: true,
				PID:                       4096,
			},
			Payload: append([]byte{
				0x00, 0x02, 0xb0, 0x12, 0x00, 0x01, 0xc1, 0x00,
				0x00, 0xe1, 0x01, 0xf0, 0x00, 0x0f, 0xe1, 0x01,
				0xf0, 0x00, 0xec, 0xe2, 0xb0, 0x94,
			}, bytes.Repeat([]byte{0xff}, 162)...),
		}, pkt)

		// PES (AAC)
		pkt, err = dem.NextPacket()
		require.NoError(t, err)
		require.Equal(t, &astits.Packet{
			AdaptationField: &astits.PacketAdaptationField{
				Length:                158,
				StuffingLength:        157,
				RandomAccessIndicator: true,
			},
			Header: astits.PacketHeader{
				HasAdaptationField:        true,
				HasPayload:                true,
				PayloadUnitStartIndicator: true,
				PID:                       257,
			},
			Payload: []byte{
				0x00, 0x00, 0x01, 0xc0, 0x00, 0x13, 0x80, 0x80,
				0x05, 0x21, 0x00, 0x11, 0x3d, 0x61, 0xff, 0xf1,
				0x50, 0x80, 0x01, 0x7f, 0xfc, 0x01, 0x02, 0x03,
				0x04,
			},
		}, pkt)
	})

	t.Run("opus", func(t *testing.T) {
		track := &Track{
			PID: 258,
			Codec: &CodecOpus{
				ChannelCount: 2,
			},
		}

		var buf bytes.Buffer
		w := NewWriter(&buf, []*Track{track})

		err := w.WriteOpus(track, 1*90000, [][]byte{{3}, {2}})
		require.NoError(t, err)

		err = w.WriteOpus(track, 2*90000, [][]byte{{1}})
		require.NoError(t, err)

		err = w.WriteOpus(track, 3*90000, [][]byte{{0}})
		require.NoError(t, err)

		dem := astits.NewDemuxer(
			context.Background(),
			bytes.NewReader(buf.Bytes()),
			astits.DemuxerOptPacketSize(188))

		// PMT
		pkt, err := dem.NextPacket()
		require.NoError(t, err)
		require.Equal(t, &astits.Packet{
			Header: astits.PacketHeader{
				HasPayload:                true,
				PayloadUnitStartIndicator: true,
				PID:                       0,
			},
			Payload: []byte{
				0x00, 0x00, 0xb0, 0x0d, 0x00, 0x00, 0xc1, 0x00,
				0x00, 0x00, 0x01, 0xf0, 0x00, 0x71, 0x10, 0xd8,
				0x78, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			},
		}, pkt)

		// PAT
		pkt, err = dem.NextPacket()
		require.NoError(t, err)
		require.Equal(t, &astits.Packet{
			Header: astits.PacketHeader{
				HasPayload:                true,
				PayloadUnitStartIndicator: true,
				PID:                       4096,
			},
			Payload: []byte{
				0x00, 0x02, 0xb0, 0x1c, 0x00, 0x01, 0xc1, 0x00,
				0x00, 0xe1, 0x02, 0xf0, 0x00, 0x06, 0xe1, 0x02,
				0xf0, 0x0a, 0x05, 0x04, 0x4f, 0x70, 0x75, 0x73,
				0x7f, 0x02, 0x80, 0x02, 0x76, 0x65, 0xbd, 0xc8,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			},
		}, pkt)

		// PES
		pkt, err = dem.NextPacket()
		require.NoError(t, err)
		require.Equal(t, &astits.Packet{
			AdaptationField: &astits.PacketAdaptationField{
				Length:                161,
				StuffingLength:        160,
				RandomAccessIndicator: true,
			},
			Header: astits.PacketHeader{
				HasAdaptationField:        true,
				HasPayload:                true,
				PayloadUnitStartIndicator: true,
				PID:                       258,
			},
			Payload: []byte{
				0x00, 0x00, 0x01, 0xc0, 0x00, 0x10, 0x80, 0x80,
				0x05, 0x21, 0x00, 0x05, 0xbf, 0x21, 0x7f, 0xe0,
				0x01, 0x03, 0x7f, 0xe0, 0x01, 0x02,
			},
		}, pkt)

		// PMT
		pkt, err = dem.NextPacket()
		require.NoError(t, err)
		require.Equal(t, &astits.Packet{
			Header: astits.PacketHeader{
				HasPayload:                true,
				PayloadUnitStartIndicator: true,
				PID:                       0,
				ContinuityCounter:         1,
			},
			Payload: []byte{
				0x00, 0x00, 0xb0, 0x0d, 0x00, 0x00, 0xc1, 0x00,
				0x00, 0x00, 0x01, 0xf0, 0x00, 0x71, 0x10, 0xd8,
				0x78, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			},
		}, pkt)

		// PAT
		pkt, err = dem.NextPacket()
		require.NoError(t, err)
		require.Equal(t, &astits.Packet{
			Header: astits.PacketHeader{
				HasPayload:                true,
				PayloadUnitStartIndicator: true,
				PID:                       4096,
				ContinuityCounter:         1,
			},
			Payload: []byte{
				0x00, 0x02, 0xb0, 0x1c, 0x00, 0x01, 0xc1, 0x00,
				0x00, 0xe1, 0x02, 0xf0, 0x00, 0x06, 0xe1, 0x02,
				0xf0, 0x0a, 0x05, 0x04, 0x4f, 0x70, 0x75, 0x73,
				0x7f, 0x02, 0x80, 0x02, 0x76, 0x65, 0xbd, 0xc8,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			},
		}, pkt)

		// PES
		pkt, err = dem.NextPacket()
		require.NoError(t, err)
		require.Equal(t, &astits.Packet{
			AdaptationField: &astits.PacketAdaptationField{
				Length:                165,
				StuffingLength:        164,
				RandomAccessIndicator: true,
			},
			Header: astits.PacketHeader{
				HasAdaptationField:        true,
				HasPayload:                true,
				PayloadUnitStartIndicator: true,
				PID:                       258,
				ContinuityCounter:         1,
			},
			Payload: []byte{
				0x00, 0x00, 0x01, 0xc0, 0x00, 0x0c, 0x80, 0x80,
				0x05, 0x21, 0x00, 0x0b, 0x7e, 0x41, 0x7f, 0xe0,
				0x01, 0x01,
			},
		}, pkt)
	})

	t.Run("mpeg-1 audio", func(t *testing.T) {
		track := &Track{
			PID:   258,
			Codec: &CodecMPEG1Audio{},
		}

		var buf bytes.Buffer
		w := NewWriter(&buf, []*Track{track})

		err := w.WriteMPEG1Audio(track, 1*90000, [][]byte{{3}, {2}})
		require.NoError(t, err)

		err = w.WriteMPEG1Audio(track, 2*90000, [][]byte{{1}})
		require.NoError(t, err)

		err = w.WriteMPEG1Audio(track, 3*90000, [][]byte{{0}})
		require.NoError(t, err)

		dem := astits.NewDemuxer(
			context.Background(),
			bytes.NewReader(buf.Bytes()),
			astits.DemuxerOptPacketSize(188))

		// PMT
		pkt, err := dem.NextPacket()
		require.NoError(t, err)
		require.Equal(t, &astits.Packet{
			Header: astits.PacketHeader{
				HasPayload:                true,
				PayloadUnitStartIndicator: true,
				PID:                       0,
			},
			Payload: []byte{
				0x00, 0x00, 0xb0, 0x0d, 0x00, 0x00, 0xc1, 0x00,
				0x00, 0x00, 0x01, 0xf0, 0x00, 0x71, 0x10, 0xd8,
				0x78, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			},
		}, pkt)

		// PAT
		pkt, err = dem.NextPacket()
		require.NoError(t, err)
		require.Equal(t, &astits.Packet{
			Header: astits.PacketHeader{
				HasPayload:                true,
				PayloadUnitStartIndicator: true,
				PID:                       4096,
			},
			Payload: []byte{
				0x00, 0x02, 0xb0, 0x12, 0x00, 0x01, 0xc1, 0x00,
				0x00, 0xe1, 0x02, 0xf0, 0x00, 0x03, 0xe1, 0x02,
				0xf0, 0x00, 0x63, 0x74, 0xa4, 0xc6, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			},
		}, pkt)

		// PES
		pkt, err = dem.NextPacket()
		require.NoError(t, err)
		require.Equal(t, &astits.Packet{
			AdaptationField: &astits.PacketAdaptationField{
				Length:                167,
				StuffingLength:        166,
				RandomAccessIndicator: true,
			},
			Header: astits.PacketHeader{
				HasAdaptationField:        true,
				HasPayload:                true,
				PayloadUnitStartIndicator: true,
				PID:                       258,
			},
			Payload: []byte{
				0x00, 0x00, 0x01, 0xc0, 0x00, 0x0a, 0x80, 0x80,
				0x05, 0x21, 0x00, 0x05, 0xbf, 0x21, 0x00, 0x00,
			},
		}, pkt)

		// PMT
		pkt, err = dem.NextPacket()
		require.NoError(t, err)
		require.Equal(t, &astits.Packet{
			Header: astits.PacketHeader{
				HasPayload:                true,
				PayloadUnitStartIndicator: true,
				PID:                       0,
				ContinuityCounter:         1,
			},
			Payload: []byte{
				0x00, 0x00, 0xb0, 0x0d, 0x00, 0x00, 0xc1, 0x00,
				0x00, 0x00, 0x01, 0xf0, 0x00, 0x71, 0x10, 0xd8,
				0x78, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			},
		}, pkt)

		// PAT
		pkt, err = dem.NextPacket()
		require.NoError(t, err)
		require.Equal(t, &astits.Packet{
			Header: astits.PacketHeader{
				HasPayload:                true,
				PayloadUnitStartIndicator: true,
				PID:                       4096,
				ContinuityCounter:         1,
			},
			Payload: []byte{
				0x00, 0x02, 0xb0, 0x12, 0x00, 0x01, 0xc1, 0x00,
				0x00, 0xe1, 0x02, 0xf0, 0x00, 0x03, 0xe1, 0x02,
				0xf0, 0x00, 0x63, 0x74, 0xa4, 0xc6, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
				0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			},
		}, pkt)

		// PES
		pkt, err = dem.NextPacket()
		require.NoError(t, err)
		require.Equal(t, &astits.Packet{
			AdaptationField: &astits.PacketAdaptationField{
				Length:                168,
				StuffingLength:        167,
				RandomAccessIndicator: true,
			},
			Header: astits.PacketHeader{
				HasAdaptationField:        true,
				HasPayload:                true,
				PayloadUnitStartIndicator: true,
				PID:                       258,
				ContinuityCounter:         1,
			},
			Payload: []byte{
				0x00, 0x00, 0x01, 0xc0, 0x00, 0x09, 0x80, 0x80,
				0x05, 0x21, 0x00, 0x0b, 0x7e, 0x41, 0x00,
			},
		}, pkt)
	})
}
