package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"
)

const maxMockVideoDurationSeconds = 15

type mockVideoRenderer struct {
	mu    sync.Mutex
	cache map[int][]byte
}

func newMockVideoRenderer(oneSecondVideo []byte) *mockVideoRenderer {
	return &mockVideoRenderer{cache: map[int][]byte{1: oneSecondVideo}}
}

func (r *mockVideoRenderer) bytesForDuration(seconds int) []byte {
	if seconds < 1 {
		seconds = 1
	}
	if seconds > maxMockVideoDurationSeconds {
		seconds = maxMockVideoDurationSeconds
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if video, ok := r.cache[seconds]; ok {
		return video
	}
	video, err := repeatMockMP4(r.cache[1], seconds)
	if err != nil {
		panic(fmt.Sprintf("extend embedded mock video to %ds: %v", seconds, err))
	}
	r.cache[seconds] = video
	return video
}

var mp4ContainerTypes = map[string]bool{
	"moov": true, "trak": true, "edts": true, "mdia": true,
	"minf": true, "dinf": true, "stbl": true,
}

func repeatMockMP4(source []byte, seconds int) ([]byte, error) {
	return rewriteMP4Boxes(source, seconds)
}

func rewriteMP4Boxes(data []byte, seconds int) ([]byte, error) {
	var output bytes.Buffer
	for offset := 0; offset < len(data); {
		if len(data)-offset < 8 {
			return nil, fmt.Errorf("truncated MP4 box header")
		}
		headerSize := 8
		size := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		if size == 1 {
			if len(data)-offset < 16 {
				return nil, fmt.Errorf("truncated extended MP4 box header")
			}
			headerSize = 16
			extendedSize := binary.BigEndian.Uint64(data[offset+8 : offset+16])
			if extendedSize > uint64(len(data)-offset) {
				return nil, fmt.Errorf("invalid extended MP4 box size %d", extendedSize)
			}
			size = int(extendedSize)
		}
		if size < headerSize || offset+size > len(data) {
			return nil, fmt.Errorf("invalid MP4 box size %d", size)
		}
		boxType := string(data[offset+4 : offset+8])
		payload := append([]byte(nil), data[offset+headerSize:offset+size]...)
		var err error
		if mp4ContainerTypes[boxType] {
			payload, err = rewriteMP4Boxes(payload, seconds)
		} else {
			payload, err = rewriteMockMP4Payload(boxType, payload, seconds)
		}
		if err != nil {
			return nil, fmt.Errorf("box %s: %w", boxType, err)
		}
		if len(payload)+8 > int(^uint32(0)) {
			return nil, fmt.Errorf("MP4 box %s exceeds 32-bit size", boxType)
		}
		_ = binary.Write(&output, binary.BigEndian, uint32(len(payload)+8))
		output.WriteString(boxType)
		output.Write(payload)
		offset += size
	}
	return output.Bytes(), nil
}

func rewriteMockMP4Payload(boxType string, payload []byte, seconds int) ([]byte, error) {
	switch boxType {
	case "mdat":
		return bytes.Repeat(payload, seconds), nil
	case "mvhd", "mdhd":
		if len(payload) < 20 || payload[0] != 0 {
			return nil, fmt.Errorf("unsupported %s box", boxType)
		}
		binary.BigEndian.PutUint32(payload[16:20], binary.BigEndian.Uint32(payload[16:20])*uint32(seconds))
	case "tkhd":
		if len(payload) < 24 || payload[0] != 0 {
			return nil, fmt.Errorf("unsupported tkhd box")
		}
		binary.BigEndian.PutUint32(payload[20:24], binary.BigEndian.Uint32(payload[20:24])*uint32(seconds))
	case "elst":
		if len(payload) < 20 || payload[0] != 0 || binary.BigEndian.Uint32(payload[4:8]) != 1 {
			return nil, fmt.Errorf("unsupported elst box")
		}
		binary.BigEndian.PutUint32(payload[8:12], binary.BigEndian.Uint32(payload[8:12])*uint32(seconds))
	case "stts":
		if len(payload) < 16 || binary.BigEndian.Uint32(payload[4:8]) != 1 {
			return nil, fmt.Errorf("unsupported stts box")
		}
		binary.BigEndian.PutUint32(payload[8:12], binary.BigEndian.Uint32(payload[8:12])*uint32(seconds))
	case "stsc":
		if len(payload) < 20 || binary.BigEndian.Uint32(payload[4:8]) != 1 {
			return nil, fmt.Errorf("unsupported stsc box")
		}
		binary.BigEndian.PutUint32(payload[12:16], binary.BigEndian.Uint32(payload[12:16])*uint32(seconds))
	case "stsz":
		if len(payload) < 12 {
			return nil, fmt.Errorf("invalid stsz box")
		}
		sampleCount := int(binary.BigEndian.Uint32(payload[8:12]))
		if binary.BigEndian.Uint32(payload[4:8]) != 0 || len(payload) != 12+sampleCount*4 {
			return nil, fmt.Errorf("unsupported stsz box")
		}
		entries := bytes.Repeat(payload[12:], seconds)
		payload = append(payload[:12], entries...)
		binary.BigEndian.PutUint32(payload[8:12], uint32(sampleCount*seconds))
	case "stss":
		if len(payload) < 8 {
			return nil, fmt.Errorf("invalid stss box")
		}
		entryCount := int(binary.BigEndian.Uint32(payload[4:8]))
		if len(payload) != 8+entryCount*4 {
			return nil, fmt.Errorf("unsupported stss box")
		}
		original := append([]byte(nil), payload[8:]...)
		entries := make([]byte, entryCount*seconds*4)
		const samplesPerSecond = 24
		for second := 0; second < seconds; second++ {
			for index := 0; index < entryCount; index++ {
				sample := binary.BigEndian.Uint32(original[index*4:]) + uint32(second*samplesPerSecond)
				binary.BigEndian.PutUint32(entries[(second*entryCount+index)*4:], sample)
			}
		}
		payload = append(payload[:8], entries...)
		binary.BigEndian.PutUint32(payload[4:8], uint32(entryCount*seconds))
	}
	return payload, nil
}
