package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"
)

func saveToDisk(track *webrtc.TrackRemote) {
	//defer func() {
	//	if err := i.Close(); err != nil {
	//		panic(err)
	//	}
	//}()

	var counter int
	for {
		rtpPacket, _, err := track.ReadRTP()
		if err != nil {
			panic(err)
		}

		counter++

		if counter%25 == 0 {
			fmt.Println(len(rtpPacket.Payload))
		}
	}
}

func main() {
	// Everything below is the Pion WebRTC API! Thanks for using it ❤️.

	// Create a MediaEngine object to configure the supported codec
	m := &webrtc.MediaEngine{}

	// Setup the codecs you want to use.
	// We'll use a VP8 and Opus but you can also define your own
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil},
		PayloadType:        96,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		panic(err)
	}
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil},
		PayloadType:        111,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		panic(err)
	}

	// Create a InterceptorRegistry. This is the user configurable RTP/RTCP Pipeline.
	// This provides NACKs, RTCP Reports and other features. If you use `webrtc.NewPeerConnection`
	// this is enabled by default. If you are manually managing You MUST create a InterceptorRegistry
	// for each PeerConnection.
	i := &interceptor.Registry{}

	// Use the default set of Interceptors
	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		panic(err)
	}

	// Create the API object with the MediaEngine
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i))

	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a new RTCPeerConnection
	peerConnection, err := api.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	// Allow us to receive 1 audio track, and 1 video track
	if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		panic(err)
	} else if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		panic(err)
	}

	//oggFile, err := oggwriter.New("output.ogg", 48000, 2)
	//if err != nil {
	//	panic(err)
	//}
	//ivfFile, err := ivfwriter.New("output.ivf")
	//if err != nil {
	//	panic(err)
	//}

	// Set a handler for when a new remote track starts, this handler saves buffers to disk as
	// an ivf file, since we could have multiple video tracks we provide a counter.
	// In your application this is where you would handle/process video
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
		go func() {
			ticker := time.NewTicker(time.Second * 3)
			for range ticker.C {
				errSend := peerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(track.SSRC())}})
				if errSend != nil {
					fmt.Println(errSend)
				}
			}
		}()

		codec := track.Codec()
		if strings.EqualFold(codec.MimeType, webrtc.MimeTypeOpus) {
			fmt.Println("Got Opus track, saving to disk as output.opus (48 kHz, 2 channels)")
			saveToDisk(track)
		} else if strings.EqualFold(codec.MimeType, webrtc.MimeTypeVP8) {
			fmt.Println("Got VP8 track, saving to disk as output.ivf")
			saveToDisk(track)
		}
	})

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())

		if connectionState == webrtc.ICEConnectionStateConnected {
			fmt.Println("Ctrl+C the remote client to stop the demo")
		} else if connectionState == webrtc.ICEConnectionStateFailed {
			//if closeErr := oggFile.Close(); closeErr != nil {
			//	panic(closeErr)
			//}
			//
			//if closeErr := ivfFile.Close(); closeErr != nil {
			//	panic(closeErr)
			//}

			fmt.Println("Done writing media files")

			// Gracefully shutdown the peer connection
			if closeErr := peerConnection.Close(); closeErr != nil {
				panic(closeErr)
			}

			fmt.Println("os.Exit(0)")
			return
		}
	})

	// Wait for the offer to be pasted
	sdp := "eyJ0eXBlIjoib2ZmZXIiLCJzZHAiOiJ2PTBcclxubz0tIDg0NjY5MjA0OTMxMzM5MDcxMjEgMiBJTiBJUDQgMTI3LjAuMC4xXHJcbnM9LVxyXG50PTAgMFxyXG5hPWdyb3VwOkJVTkRMRSAwIDFcclxuYT1leHRtYXAtYWxsb3ctbWl4ZWRcclxuYT1tc2lkLXNlbWFudGljOiBXTVMgbkVScG9iUzZKMklhMnZ5NTRoUzE5b1hDUjhYV1JMV1hBSEZmXHJcbm09YXVkaW8gNjQ3NzQgVURQL1RMUy9SVFAvU0FWUEYgMTExIDYzIDEwMyAxMDQgOSAwIDggMTA2IDEwNSAxMyAxMTAgMTEyIDExMyAxMjZcclxuYz1JTiBJUDQgMjMuODguMTA2LjIzNlxyXG5hPXJ0Y3A6OSBJTiBJUDQgMC4wLjAuMFxyXG5hPWNhbmRpZGF0ZTozNTk2NzM4ODY4IDEgdWRwIDIxMjIyNjAyMjMgMTkyLjE2OC4xLjE2IDYyNzU2IHR5cCBob3N0IGdlbmVyYXRpb24gMCBuZXR3b3JrLWlkIDEgbmV0d29yay1jb3N0IDEwXHJcbmE9Y2FuZGlkYXRlOjI3NzQ5MDA4ODEgMSB1ZHAgMjEyMjE5NDY4NyAxNzIuMjcuMjQxLjE4NiA2NDc3NCB0eXAgaG9zdCBnZW5lcmF0aW9uIDAgbmV0d29yay1pZCAyIG5ldHdvcmstY29zdCA1MFxyXG5hPWNhbmRpZGF0ZToyNTY0OTU1NTg4IDEgdGNwIDE1MTgyODA0NDcgMTkyLjE2OC4xLjE2IDkgdHlwIGhvc3QgdGNwdHlwZSBhY3RpdmUgZ2VuZXJhdGlvbiAwIG5ldHdvcmstaWQgMSBuZXR3b3JrLWNvc3QgMTBcclxuYT1jYW5kaWRhdGU6Mzk1Nzc0MjY4OSAxIHRjcCAxNTE4MjE0OTExIDE3Mi4yNy4yNDEuMTg2IDkgdHlwIGhvc3QgdGNwdHlwZSBhY3RpdmUgZ2VuZXJhdGlvbiAwIG5ldHdvcmstaWQgMiBuZXR3b3JrLWNvc3QgNTBcclxuYT1jYW5kaWRhdGU6MTc3Njg1MjczOCAxIHVkcCAxNjg1OTg3MDcxIDIzLjg4LjEwNi4yMzYgNjQ3NzQgdHlwIHNyZmx4IHJhZGRyIDE3Mi4yNy4yNDEuMTg2IHJwb3J0IDY0Nzc0IGdlbmVyYXRpb24gMCBuZXR3b3JrLWlkIDIgbmV0d29yay1jb3N0IDUwXHJcbmE9aWNlLXVmcmFnOlFYa25cclxuYT1pY2UtcHdkOnEwazFVL2psaitseHA1REhEdDBIVG1Vb1xyXG5hPWljZS1vcHRpb25zOnRyaWNrbGVcclxuYT1maW5nZXJwcmludDpzaGEtMjU2IDFBOkRDOjc3OjI3OkM5OjlBOjQ5OkQ3OkFEOkZGOjZGOjU5OjQ1OjYzOjFEOjkzOjJBOkQ1OjAyOjA2OjRFOjE4OjQwOjY3OkRBOjUwOkY2OkY2OjNFOjIzOkZBOkRFXHJcbmE9c2V0dXA6YWN0cGFzc1xyXG5hPW1pZDowXHJcbmE9ZXh0bWFwOjEgdXJuOmlldGY6cGFyYW1zOnJ0cC1oZHJleHQ6c3NyYy1hdWRpby1sZXZlbFxyXG5hPWV4dG1hcDoyIGh0dHA6Ly93d3cud2VicnRjLm9yZy9leHBlcmltZW50cy9ydHAtaGRyZXh0L2Ficy1zZW5kLXRpbWVcclxuYT1leHRtYXA6MyBodHRwOi8vd3d3LmlldGYub3JnL2lkL2RyYWZ0LWhvbG1lci1ybWNhdC10cmFuc3BvcnQtd2lkZS1jYy1leHRlbnNpb25zLTAxXHJcbmE9ZXh0bWFwOjQgdXJuOmlldGY6cGFyYW1zOnJ0cC1oZHJleHQ6c2RlczptaWRcclxuYT1zZW5kcmVjdlxyXG5hPW1zaWQ6bkVScG9iUzZKMklhMnZ5NTRoUzE5b1hDUjhYV1JMV1hBSEZmIGY0MWM2ZmY0LTA5Y2EtNGZkOS04YTY5LWM5YmQ3ZDZmOTI3M1xyXG5hPXJ0Y3AtbXV4XHJcbmE9cnRwbWFwOjExMSBvcHVzLzQ4MDAwLzJcclxuYT1ydGNwLWZiOjExMSB0cmFuc3BvcnQtY2NcclxuYT1mbXRwOjExMSBtaW5wdGltZT0xMDt1c2VpbmJhbmRmZWM9MVxyXG5hPXJ0cG1hcDo2MyByZWQvNDgwMDAvMlxyXG5hPWZtdHA6NjMgMTExLzExMVxyXG5hPXJ0cG1hcDoxMDMgSVNBQy8xNjAwMFxyXG5hPXJ0cG1hcDoxMDQgSVNBQy8zMjAwMFxyXG5hPXJ0cG1hcDo5IEc3MjIvODAwMFxyXG5hPXJ0cG1hcDowIFBDTVUvODAwMFxyXG5hPXJ0cG1hcDo4IFBDTUEvODAwMFxyXG5hPXJ0cG1hcDoxMDYgQ04vMzIwMDBcclxuYT1ydHBtYXA6MTA1IENOLzE2MDAwXHJcbmE9cnRwbWFwOjEzIENOLzgwMDBcclxuYT1ydHBtYXA6MTEwIHRlbGVwaG9uZS1ldmVudC80ODAwMFxyXG5hPXJ0cG1hcDoxMTIgdGVsZXBob25lLWV2ZW50LzMyMDAwXHJcbmE9cnRwbWFwOjExMyB0ZWxlcGhvbmUtZXZlbnQvMTYwMDBcclxuYT1ydHBtYXA6MTI2IHRlbGVwaG9uZS1ldmVudC84MDAwXHJcbmE9c3NyYzoyNTIxMDI1NTY1IGNuYW1lOlcyQlo0Rmk5OWhYd2RvVExcclxuYT1zc3JjOjI1MjEwMjU1NjUgbXNpZDpuRVJwb2JTNkoySWEydnk1NGhTMTlvWENSOFhXUkxXWEFIRmYgZjQxYzZmZjQtMDljYS00ZmQ5LThhNjktYzliZDdkNmY5MjczXHJcbm09dmlkZW8gNTk2NDMgVURQL1RMUy9SVFAvU0FWUEYgOTYgOTcgMTAyIDEyMiAxMjcgMTIxIDEyNSAxMDcgMTA4IDEwOSAxMjQgMTIwIDM5IDQwIDQ1IDQ2IDk4IDk5IDEwMCAxMDEgMTIzIDExOSAxMTQgMTE1IDExNlxyXG5jPUlOIElQNCAyMy44OC4xMDYuMjM2XHJcbmE9cnRjcDo5IElOIElQNCAwLjAuMC4wXHJcbmE9Y2FuZGlkYXRlOjM1OTY3Mzg4NjggMSB1ZHAgMjEyMjI2MDIyMyAxOTIuMTY4LjEuMTYgNTI1MTEgdHlwIGhvc3QgZ2VuZXJhdGlvbiAwIG5ldHdvcmstaWQgMSBuZXR3b3JrLWNvc3QgMTBcclxuYT1jYW5kaWRhdGU6Mjc3NDkwMDg4MSAxIHVkcCAyMTIyMTk0Njg3IDE3Mi4yNy4yNDEuMTg2IDU5NjQzIHR5cCBob3N0IGdlbmVyYXRpb24gMCBuZXR3b3JrLWlkIDIgbmV0d29yay1jb3N0IDUwXHJcbmE9Y2FuZGlkYXRlOjI1NjQ5NTU1ODggMSB0Y3AgMTUxODI4MDQ0NyAxOTIuMTY4LjEuMTYgOSB0eXAgaG9zdCB0Y3B0eXBlIGFjdGl2ZSBnZW5lcmF0aW9uIDAgbmV0d29yay1pZCAxIG5ldHdvcmstY29zdCAxMFxyXG5hPWNhbmRpZGF0ZTozOTU3NzQyNjg5IDEgdGNwIDE1MTgyMTQ5MTEgMTcyLjI3LjI0MS4xODYgOSB0eXAgaG9zdCB0Y3B0eXBlIGFjdGl2ZSBnZW5lcmF0aW9uIDAgbmV0d29yay1pZCAyIG5ldHdvcmstY29zdCA1MFxyXG5hPWNhbmRpZGF0ZToxNzc2ODUyNzM4IDEgdWRwIDE2ODU5ODcwNzEgMjMuODguMTA2LjIzNiA1OTY0MyB0eXAgc3JmbHggcmFkZHIgMTcyLjI3LjI0MS4xODYgcnBvcnQgNTk2NDMgZ2VuZXJhdGlvbiAwIG5ldHdvcmstaWQgMiBuZXR3b3JrLWNvc3QgNTBcclxuYT1pY2UtdWZyYWc6UVhrblxyXG5hPWljZS1wd2Q6cTBrMVUvamxqK2x4cDVESER0MEhUbVVvXHJcbmE9aWNlLW9wdGlvbnM6dHJpY2tsZVxyXG5hPWZpbmdlcnByaW50OnNoYS0yNTYgMUE6REM6Nzc6Mjc6Qzk6OUE6NDk6RDc6QUQ6RkY6NkY6NTk6NDU6NjM6MUQ6OTM6MkE6RDU6MDI6MDY6NEU6MTg6NDA6Njc6REE6NTA6RjY6RjY6M0U6MjM6RkE6REVcclxuYT1zZXR1cDphY3RwYXNzXHJcbmE9bWlkOjFcclxuYT1leHRtYXA6MTQgdXJuOmlldGY6cGFyYW1zOnJ0cC1oZHJleHQ6dG9mZnNldFxyXG5hPWV4dG1hcDoyIGh0dHA6Ly93d3cud2VicnRjLm9yZy9leHBlcmltZW50cy9ydHAtaGRyZXh0L2Ficy1zZW5kLXRpbWVcclxuYT1leHRtYXA6MTMgdXJuOjNncHA6dmlkZW8tb3JpZW50YXRpb25cclxuYT1leHRtYXA6MyBodHRwOi8vd3d3LmlldGYub3JnL2lkL2RyYWZ0LWhvbG1lci1ybWNhdC10cmFuc3BvcnQtd2lkZS1jYy1leHRlbnNpb25zLTAxXHJcbmE9ZXh0bWFwOjUgaHR0cDovL3d3dy53ZWJydGMub3JnL2V4cGVyaW1lbnRzL3J0cC1oZHJleHQvcGxheW91dC1kZWxheVxyXG5hPWV4dG1hcDo2IGh0dHA6Ly93d3cud2VicnRjLm9yZy9leHBlcmltZW50cy9ydHAtaGRyZXh0L3ZpZGVvLWNvbnRlbnQtdHlwZVxyXG5hPWV4dG1hcDo3IGh0dHA6Ly93d3cud2VicnRjLm9yZy9leHBlcmltZW50cy9ydHAtaGRyZXh0L3ZpZGVvLXRpbWluZ1xyXG5hPWV4dG1hcDo4IGh0dHA6Ly93d3cud2VicnRjLm9yZy9leHBlcmltZW50cy9ydHAtaGRyZXh0L2NvbG9yLXNwYWNlXHJcbmE9ZXh0bWFwOjQgdXJuOmlldGY6cGFyYW1zOnJ0cC1oZHJleHQ6c2RlczptaWRcclxuYT1leHRtYXA6MTAgdXJuOmlldGY6cGFyYW1zOnJ0cC1oZHJleHQ6c2RlczpydHAtc3RyZWFtLWlkXHJcbmE9ZXh0bWFwOjExIHVybjppZXRmOnBhcmFtczpydHAtaGRyZXh0OnNkZXM6cmVwYWlyZWQtcnRwLXN0cmVhbS1pZFxyXG5hPXNlbmRyZWN2XHJcbmE9bXNpZDpuRVJwb2JTNkoySWEydnk1NGhTMTlvWENSOFhXUkxXWEFIRmYgY2RmMzNiZWQtNTYzNS00NGNiLTk4ODktNDE1YTM3MzhkM2MyXHJcbmE9cnRjcC1tdXhcclxuYT1ydGNwLXJzaXplXHJcbmE9cnRwbWFwOjk2IFZQOC85MDAwMFxyXG5hPXJ0Y3AtZmI6OTYgZ29vZy1yZW1iXHJcbmE9cnRjcC1mYjo5NiB0cmFuc3BvcnQtY2NcclxuYT1ydGNwLWZiOjk2IGNjbSBmaXJcclxuYT1ydGNwLWZiOjk2IG5hY2tcclxuYT1ydGNwLWZiOjk2IG5hY2sgcGxpXHJcbmE9cnRwbWFwOjk3IHJ0eC85MDAwMFxyXG5hPWZtdHA6OTcgYXB0PTk2XHJcbmE9cnRwbWFwOjEwMiBIMjY0LzkwMDAwXHJcbmE9cnRjcC1mYjoxMDIgZ29vZy1yZW1iXHJcbmE9cnRjcC1mYjoxMDIgdHJhbnNwb3J0LWNjXHJcbmE9cnRjcC1mYjoxMDIgY2NtIGZpclxyXG5hPXJ0Y3AtZmI6MTAyIG5hY2tcclxuYT1ydGNwLWZiOjEwMiBuYWNrIHBsaVxyXG5hPWZtdHA6MTAyIGxldmVsLWFzeW1tZXRyeS1hbGxvd2VkPTE7cGFja2V0aXphdGlvbi1tb2RlPTE7cHJvZmlsZS1sZXZlbC1pZD00MjAwMWZcclxuYT1ydHBtYXA6MTIyIHJ0eC85MDAwMFxyXG5hPWZtdHA6MTIyIGFwdD0xMDJcclxuYT1ydHBtYXA6MTI3IEgyNjQvOTAwMDBcclxuYT1ydGNwLWZiOjEyNyBnb29nLXJlbWJcclxuYT1ydGNwLWZiOjEyNyB0cmFuc3BvcnQtY2NcclxuYT1ydGNwLWZiOjEyNyBjY20gZmlyXHJcbmE9cnRjcC1mYjoxMjcgbmFja1xyXG5hPXJ0Y3AtZmI6MTI3IG5hY2sgcGxpXHJcbmE9Zm10cDoxMjcgbGV2ZWwtYXN5bW1ldHJ5LWFsbG93ZWQ9MTtwYWNrZXRpemF0aW9uLW1vZGU9MDtwcm9maWxlLWxldmVsLWlkPTQyMDAxZlxyXG5hPXJ0cG1hcDoxMjEgcnR4LzkwMDAwXHJcbmE9Zm10cDoxMjEgYXB0PTEyN1xyXG5hPXJ0cG1hcDoxMjUgSDI2NC85MDAwMFxyXG5hPXJ0Y3AtZmI6MTI1IGdvb2ctcmVtYlxyXG5hPXJ0Y3AtZmI6MTI1IHRyYW5zcG9ydC1jY1xyXG5hPXJ0Y3AtZmI6MTI1IGNjbSBmaXJcclxuYT1ydGNwLWZiOjEyNSBuYWNrXHJcbmE9cnRjcC1mYjoxMjUgbmFjayBwbGlcclxuYT1mbXRwOjEyNSBsZXZlbC1hc3ltbWV0cnktYWxsb3dlZD0xO3BhY2tldGl6YXRpb24tbW9kZT0xO3Byb2ZpbGUtbGV2ZWwtaWQ9NDJlMDFmXHJcbmE9cnRwbWFwOjEwNyBydHgvOTAwMDBcclxuYT1mbXRwOjEwNyBhcHQ9MTI1XHJcbmE9cnRwbWFwOjEwOCBIMjY0LzkwMDAwXHJcbmE9cnRjcC1mYjoxMDggZ29vZy1yZW1iXHJcbmE9cnRjcC1mYjoxMDggdHJhbnNwb3J0LWNjXHJcbmE9cnRjcC1mYjoxMDggY2NtIGZpclxyXG5hPXJ0Y3AtZmI6MTA4IG5hY2tcclxuYT1ydGNwLWZiOjEwOCBuYWNrIHBsaVxyXG5hPWZtdHA6MTA4IGxldmVsLWFzeW1tZXRyeS1hbGxvd2VkPTE7cGFja2V0aXphdGlvbi1tb2RlPTA7cHJvZmlsZS1sZXZlbC1pZD00MmUwMWZcclxuYT1ydHBtYXA6MTA5IHJ0eC85MDAwMFxyXG5hPWZtdHA6MTA5IGFwdD0xMDhcclxuYT1ydHBtYXA6MTI0IEgyNjQvOTAwMDBcclxuYT1ydGNwLWZiOjEyNCBnb29nLXJlbWJcclxuYT1ydGNwLWZiOjEyNCB0cmFuc3BvcnQtY2NcclxuYT1ydGNwLWZiOjEyNCBjY20gZmlyXHJcbmE9cnRjcC1mYjoxMjQgbmFja1xyXG5hPXJ0Y3AtZmI6MTI0IG5hY2sgcGxpXHJcbmE9Zm10cDoxMjQgbGV2ZWwtYXN5bW1ldHJ5LWFsbG93ZWQ9MTtwYWNrZXRpemF0aW9uLW1vZGU9MTtwcm9maWxlLWxldmVsLWlkPTRkMDAxZlxyXG5hPXJ0cG1hcDoxMjAgcnR4LzkwMDAwXHJcbmE9Zm10cDoxMjAgYXB0PTEyNFxyXG5hPXJ0cG1hcDozOSBIMjY0LzkwMDAwXHJcbmE9cnRjcC1mYjozOSBnb29nLXJlbWJcclxuYT1ydGNwLWZiOjM5IHRyYW5zcG9ydC1jY1xyXG5hPXJ0Y3AtZmI6MzkgY2NtIGZpclxyXG5hPXJ0Y3AtZmI6MzkgbmFja1xyXG5hPXJ0Y3AtZmI6MzkgbmFjayBwbGlcclxuYT1mbXRwOjM5IGxldmVsLWFzeW1tZXRyeS1hbGxvd2VkPTE7cGFja2V0aXphdGlvbi1tb2RlPTA7cHJvZmlsZS1sZXZlbC1pZD00ZDAwMWZcclxuYT1ydHBtYXA6NDAgcnR4LzkwMDAwXHJcbmE9Zm10cDo0MCBhcHQ9MzlcclxuYT1ydHBtYXA6NDUgQVYxLzkwMDAwXHJcbmE9cnRjcC1mYjo0NSBnb29nLXJlbWJcclxuYT1ydGNwLWZiOjQ1IHRyYW5zcG9ydC1jY1xyXG5hPXJ0Y3AtZmI6NDUgY2NtIGZpclxyXG5hPXJ0Y3AtZmI6NDUgbmFja1xyXG5hPXJ0Y3AtZmI6NDUgbmFjayBwbGlcclxuYT1ydHBtYXA6NDYgcnR4LzkwMDAwXHJcbmE9Zm10cDo0NiBhcHQ9NDVcclxuYT1ydHBtYXA6OTggVlA5LzkwMDAwXHJcbmE9cnRjcC1mYjo5OCBnb29nLXJlbWJcclxuYT1ydGNwLWZiOjk4IHRyYW5zcG9ydC1jY1xyXG5hPXJ0Y3AtZmI6OTggY2NtIGZpclxyXG5hPXJ0Y3AtZmI6OTggbmFja1xyXG5hPXJ0Y3AtZmI6OTggbmFjayBwbGlcclxuYT1mbXRwOjk4IHByb2ZpbGUtaWQ9MFxyXG5hPXJ0cG1hcDo5OSBydHgvOTAwMDBcclxuYT1mbXRwOjk5IGFwdD05OFxyXG5hPXJ0cG1hcDoxMDAgVlA5LzkwMDAwXHJcbmE9cnRjcC1mYjoxMDAgZ29vZy1yZW1iXHJcbmE9cnRjcC1mYjoxMDAgdHJhbnNwb3J0LWNjXHJcbmE9cnRjcC1mYjoxMDAgY2NtIGZpclxyXG5hPXJ0Y3AtZmI6MTAwIG5hY2tcclxuYT1ydGNwLWZiOjEwMCBuYWNrIHBsaVxyXG5hPWZtdHA6MTAwIHByb2ZpbGUtaWQ9MlxyXG5hPXJ0cG1hcDoxMDEgcnR4LzkwMDAwXHJcbmE9Zm10cDoxMDEgYXB0PTEwMFxyXG5hPXJ0cG1hcDoxMjMgSDI2NC85MDAwMFxyXG5hPXJ0Y3AtZmI6MTIzIGdvb2ctcmVtYlxyXG5hPXJ0Y3AtZmI6MTIzIHRyYW5zcG9ydC1jY1xyXG5hPXJ0Y3AtZmI6MTIzIGNjbSBmaXJcclxuYT1ydGNwLWZiOjEyMyBuYWNrXHJcbmE9cnRjcC1mYjoxMjMgbmFjayBwbGlcclxuYT1mbXRwOjEyMyBsZXZlbC1hc3ltbWV0cnktYWxsb3dlZD0xO3BhY2tldGl6YXRpb24tbW9kZT0xO3Byb2ZpbGUtbGV2ZWwtaWQ9NjQwMDFmXHJcbmE9cnRwbWFwOjExOSBydHgvOTAwMDBcclxuYT1mbXRwOjExOSBhcHQ9MTIzXHJcbmE9cnRwbWFwOjExNCByZWQvOTAwMDBcclxuYT1ydHBtYXA6MTE1IHJ0eC85MDAwMFxyXG5hPWZtdHA6MTE1IGFwdD0xMTRcclxuYT1ydHBtYXA6MTE2IHVscGZlYy85MDAwMFxyXG5hPXNzcmMtZ3JvdXA6RklEIDIwNDMzODIwIDIxNjE4MDE1OFxyXG5hPXNzcmM6MjA0MzM4MjAgY25hbWU6VzJCWjRGaTk5aFh3ZG9UTFxyXG5hPXNzcmM6MjA0MzM4MjAgbXNpZDpuRVJwb2JTNkoySWEydnk1NGhTMTlvWENSOFhXUkxXWEFIRmYgY2RmMzNiZWQtNTYzNS00NGNiLTk4ODktNDE1YTM3MzhkM2MyXHJcbmE9c3NyYzoyMTYxODAxNTggY25hbWU6VzJCWjRGaTk5aFh3ZG9UTFxyXG5hPXNzcmM6MjE2MTgwMTU4IG1zaWQ6bkVScG9iUzZKMklhMnZ5NTRoUzE5b1hDUjhYV1JMV1hBSEZmIGNkZjMzYmVkLTU2MzUtNDRjYi05ODg5LTQxNWEzNzM4ZDNjMlxyXG4ifQ=="
	offer := webrtc.SessionDescription{}
	Decode(sdp, &offer)

	// Set the remote SessionDescription
	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
		panic(err)
	}

	// Create answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Sets the LocalDescription, and starts our UDP listeners
	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		panic(err)
	}

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

	// Output the answer in base64 so we can paste it in browser
	fmt.Println(Encode(*peerConnection.LocalDescription()))

	// Block forever
	select {}
}

// Allows compressing offer/answer to bypass terminal input limits.
const compress = false

// MustReadStdin blocks until input is received from stdin
func MustReadStdin() string {
	r := bufio.NewReader(os.Stdin)

	var in string
	for {
		var err error
		in, err = r.ReadString('\n')
		if err != io.EOF {
			if err != nil {
				panic(err)
			}
		}
		in = strings.TrimSpace(in)
		if len(in) > 0 {
			break
		}
	}

	return in
}

// Encode encodes the input in base64
// It can optionally zip the input before encoding
func Encode(obj interface{}) string {
	b, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}

	if compress {
		b = zip(b)
	}

	return base64.StdEncoding.EncodeToString(b)
}

// Decode decodes the input from base64
// It can optionally unzip the input after decoding
func Decode(in string, obj interface{}) {
	b, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		panic(err)
	}

	if compress {
		b = unzip(b)
	}

	err = json.Unmarshal(b, obj)
	if err != nil {
		panic(err)
	}
}

func zip(in []byte) []byte {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	_, err := gz.Write(in)
	if err != nil {
		panic(err)
	}
	err = gz.Flush()
	if err != nil {
		panic(err)
	}
	err = gz.Close()
	if err != nil {
		panic(err)
	}
	return b.Bytes()
}

func unzip(in []byte) []byte {
	var b bytes.Buffer
	_, err := b.Write(in)
	if err != nil {
		panic(err)
	}
	r, err := gzip.NewReader(&b)
	if err != nil {
		panic(err)
	}
	res, err := ioutil.ReadAll(r)
	if err != nil {
		panic(err)
	}
	return res
}
