package codec

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"encoding/binary"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
)

type HermesDecoder struct {
	MAGIC      []byte
	CRC_LENGTH int
}

// http://git.dev.sh.ctripcorp.com/hermes/hermes/blob/85593a3d/hermes-core/src/main/java/com/ctrip/hermes/core/message/codec/internal/MessageCodecBinaryV1Handler.java#L185
func getHeaderProperties(header []byte, thekey string) []byte {
	var (
		offset    int = 0
		length    int
		firstByte int8
	)

	//codec.writeString(msg.getKey());
	firstByte = int8(header[offset])
	if firstByte != -1 {
		length = int(binary.BigEndian.Uint32(header[offset:]))
		offset += 4
		offset += length
	} else {
		offset += 1
	}

	offset += 8 // skip bornTime
	offset += 4 // skip remaining retries

	//codec.writeString(msg.getBodyCodecType());
	firstByte = int8(header[offset])
	if firstByte != -1 {
		length = int(binary.BigEndian.Uint32(header[offset:]))
		offset += 4
		offset += length
	} else {
		offset += 1
	}

	// writeProperties(msg.getDurableProperties(), buf, codec);
	firstByte = int8(header[offset])
	if firstByte == -1 {
		return nil
	}

	//properties_bytes_length := int(binary.BigEndian.Uint32(header[offset:]))
	offset += 4

	properties_count := int(binary.BigEndian.Uint32(header[offset:]))
	offset += 4
	for i := 0; i < properties_count; i++ {
		length = int(binary.BigEndian.Uint32(header[i+offset:]))
		i += 4
		key := string(header[i+offset : i+offset+length])
		i += length

		length = int(binary.BigEndian.Uint32(header[offset:]))
		i += 4
		value := header[i+offset : i+offset+length-1]
		i += length

		if key == thekey {
			return value
		}
	}
	return nil
}

func getCodecType(header []byte) string {
	var (
		offset    int = 0
		length    int
		firstByte int8
	)

	firstByte = int8(header[offset])
	if firstByte != -1 {
		length = int(binary.BigEndian.Uint32(header[offset:]))
		offset += 4
		offset += length
	} else {
		offset += 1
	}
	offset += 8 // skip bornTime
	offset += 4 // skip remaining retries

	firstByte = int8(header[offset])
	if firstByte != -1 {
		length = int(binary.BigEndian.Uint32(header[offset:]))
		offset += 4
		return string(header[offset : offset+length])
	}

	return ""
}

func (hd *HermesDecoder) Decode(value []byte) map[string]interface{} {
	offset := 0
	offset += 4

	//version := int(value[offset]) // version
	offset++

	//binary.BigEndian.Uint32(value[offset:]) // totalLength
	offset += 4

	headerLength := int(binary.BigEndian.Uint32(value[offset:]))
	offset += 4

	//bodyLength := binary.BigEndian.Uint32(value[offset:])
	offset += 4

	timestamp, parse_timestamp_err := strconv.ParseInt(string(getHeaderProperties(value[offset:offset+headerLength], "APP.@timestamp")), 10, 64)

	if parse_timestamp_err == nil {
		value = value[offset+headerLength : len(value)-hd.CRC_LENGTH]
		return map[string]interface{}{
			"@timestamp": timestamp,
			"_source":    value,
		}
	}

	glog.V(10).Infof("could not get timestamp: %s", parse_timestamp_err)

	codecType := getCodecType(value[offset : offset+headerLength])
	codeAndCompress := strings.SplitN(codecType, ",", 2)

	value = value[offset+headerLength : len(value)-hd.CRC_LENGTH]

	if len(codeAndCompress) == 2 {
		if codeAndCompress[1] == "gzip" {
			reader, err := gzip.NewReader(bytes.NewReader(value))
			if err != nil {
				glog.Errorf("gzip decode hermes message error:%s", err)
				return nil
			}
			if value, err = ioutil.ReadAll(reader); err != nil && err != io.EOF {
				glog.Errorf("gzip decode hermes message error:%s", err)
				return nil
			}
		} else if strings.Contains(codeAndCompress[1], "deflater") {
			reader := flate.NewReader(bytes.NewReader(value))
			var err error
			if value, err = ioutil.ReadAll(reader); err != nil && err != io.EOF {
				glog.Errorf("gzip decode hermes message error:%s", err)
				return nil
			}
		} else {
			glog.Fatalf("%s unknown codec type", codecType)
		}
	}

	rst := map[string]interface{}{"@timestamp": time.Now()}
	d := json.NewDecoder(bytes.NewReader(value))
	d.UseNumber()
	err := d.Decode(&rst)
	if err != nil || d.More() {
		rst["@timestamp"] = time.Now()
		rst["message"] = string(value)
		return rst
	}

	return map[string]interface{}{
		"@timestamp": rst["@timestamp"],
		"_source":    value,
	}
}
