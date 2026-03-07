//go:build openh264
// +build openh264

package h264

/*
#cgo CFLAGS: -I/opt/homebrew/include -I/usr/local/include
#cgo LDFLAGS: -L/opt/homebrew/lib -L/usr/local/lib -lopenh264
#include <wels/codec_api.h>
#include <stdlib.h>
#include <string.h>

ISVCDecoder* create_decoder() {
    ISVCDecoder *decoder = NULL;
    int ret = WelsCreateDecoder(&decoder);
    if (ret != 0 || decoder == NULL) {
        return NULL;
    }

    SDecodingParam sDecParam;
    memset(&sDecParam, 0, sizeof(sDecParam));
    sDecParam.sVideoProperty.eVideoBsType = VIDEO_BITSTREAM_AVC;
    sDecParam.eEcActiveIdc = ERROR_CON_FRAME_COPY;
    sDecParam.bParseOnly = 0;

    ret = (*decoder)->Initialize(decoder, &sDecParam);
    if (ret != 0) {
        WelsDestroyDecoder(decoder);
        return NULL;
    }

    return decoder;
}

int decode_frame(ISVCDecoder *decoder, unsigned char *data, int len,
                 unsigned char **yuv_data, int *width, int *height) {
    unsigned char *pData[3] = {NULL};
    SBufferInfo sDstBufInfo;
    memset(&sDstBufInfo, 0, sizeof(sDstBufInfo));

    sDstBufInfo.uiInBsTimeStamp = 0;
    int ret = (*decoder)->DecodeFrameNoDelay(decoder, data, len, pData, &sDstBufInfo);

    if (ret != 0) {
        return -1;
    }

    if (sDstBufInfo.iBufferStatus == 1) {
        *width = sDstBufInfo.UsrData.sSystemBuffer.iWidth;
        *height = sDstBufInfo.UsrData.sSystemBuffer.iHeight;

        int ySize = (*width) * (*height);
        int uvSize = ySize / 4;
        int totalSize = ySize + 2 * uvSize;

        *yuv_data = (unsigned char*)malloc(totalSize);
        if (*yuv_data == NULL) {
            return -2;
        }

        int yStride = sDstBufInfo.UsrData.sSystemBuffer.iStride[0];
        int uvStride = sDstBufInfo.UsrData.sSystemBuffer.iStride[1];

        unsigned char *dst = *yuv_data;

        for (int i = 0; i < *height; i++) {
            memcpy(dst, pData[0] + i * yStride, *width);
            dst += *width;
        }

        for (int i = 0; i < (*height / 2); i++) {
            memcpy(dst, pData[1] + i * uvStride, *width / 2);
            dst += (*width / 2);
        }

        for (int i = 0; i < (*height / 2); i++) {
            memcpy(dst, pData[2] + i * uvStride, *width / 2);
            dst += (*width / 2);
        }

        return totalSize;
    }

    return 0;
}

void destroy_decoder(ISVCDecoder *decoder) {
    if (decoder != NULL) {
        (*decoder)->Uninitialize(decoder);
        WelsDestroyDecoder(decoder);
    }
}
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// OpenH264Decoder wraps OpenH264 decoder
type OpenH264Decoder struct {
	dec *C.ISVCDecoder
}

func NewOpenH264Decoder() (*OpenH264Decoder, error) {
	d := C.create_decoder()
	if d == nil {
		return nil, fmt.Errorf("failed to create OpenH264 decoder")
	}
	return &OpenH264Decoder{dec: d}, nil
}

// Decode takes an access unit represented as a slice of NALUs (each []byte),
// builds Annex-B data and decodes to YUV420 (I420) bytes.
func (d *OpenH264Decoder) Decode(nalus [][]byte) ([]byte, int, int, error) {
	if d.dec == nil {
		return nil, 0, 0, fmt.Errorf("decoder not initialized")
	}
	if len(nalus) == 0 {
		return nil, 0, 0, fmt.Errorf("empty access unit")
	}

	totalLen := 0
	for _, n := range nalus {
		totalLen += 4 + len(n)
	}
	data := make([]byte, totalLen)
	off := 0
	for _, n := range nalus {
		data[off] = 0x00
		data[off+1] = 0x00
		data[off+2] = 0x00
		data[off+3] = 0x01
		off += 4
		copy(data[off:], n)
		off += len(n)
	}

	var yuvData *C.uchar
	var w, h C.int
	ret := C.decode_frame(d.dec, (*C.uchar)(unsafe.Pointer(&data[0])), C.int(len(data)), &yuvData, &w, &h)
	if ret < 0 {
		return nil, 0, 0, fmt.Errorf("decode error: %d", ret)
	}
	if ret == 0 {
		return nil, 0, 0, nil
	}

	size := int(ret)
	goBytes := C.GoBytes(unsafe.Pointer(yuvData), C.int(size))
	C.free(unsafe.Pointer(yuvData))
	return goBytes, int(w), int(h), nil
}

func (d *OpenH264Decoder) Close() {
	if d.dec != nil {
		C.destroy_decoder(d.dec)
		d.dec = nil
	}
}
