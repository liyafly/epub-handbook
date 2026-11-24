package mediaoverlay

import (
	"errors"
	"fmt"
)

// MP3Duration 逐帧累计 MPEG 音频（Layer III）的时长，CBR 与 VBR 都适用。
// 跳过开头的 ID3v2 标签，遇到 ID3v1（"TAG"）或无法同步的尾部停止。
func MP3Duration(data []byte) (float64, error) {
	pos := 0
	if len(data) >= 10 && string(data[:3]) == "ID3" {
		size := int(data[6]&0x7f)<<21 | int(data[7]&0x7f)<<14 | int(data[8]&0x7f)<<7 | int(data[9]&0x7f)
		pos = 10 + size
		if data[5]&0x10 != 0 {
			pos += 10 // footer
		}
	}
	bitrates := [2][16]int{
		{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0}, // MPEG-1 Layer III
		{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0},     // MPEG-2/2.5 Layer III
	}
	rates := [3][3]int{{44100, 48000, 32000}, {22050, 24000, 16000}, {11025, 12000, 8000}}
	var seconds float64
	frames := 0
	for pos+4 <= len(data) {
		h := data[pos : pos+4]
		if h[0] != 0xff || h[1]&0xe0 != 0xe0 {
			if frames == 0 {
				pos++ // 首帧前的填充字节
				continue
			}
			break
		}
		version := (h[1] >> 3) & 3 // 0=2.5 1=reserved 2=MPEG-2 3=MPEG-1
		layer := (h[1] >> 1) & 3   // 1=Layer III
		brIdx := h[2] >> 4
		srIdx := (h[2] >> 2) & 3
		if version == 1 || layer != 1 || brIdx == 0 || brIdx == 15 || srIdx == 3 {
			if frames == 0 {
				pos++
				continue
			}
			break
		}
		padding := int(h[2]>>1) & 1
		var vi, spf, coef int
		switch version {
		case 3:
			vi, spf, coef = 0, 1152, 144
		case 2:
			vi, spf, coef = 1, 576, 72
		default:
			vi, spf, coef = 2, 576, 72
		}
		br := bitrates[min(vi, 1)][brIdx] * 1000
		sr := rates[vi][srIdx]
		length := coef*br/sr + padding
		if length < 4 {
			break
		}
		frames++
		seconds += float64(spf) / float64(sr)
		pos += length
	}
	if frames == 0 {
		return 0, errors.New("no MPEG Layer III frames found")
	}
	if frames < 10 {
		return 0, fmt.Errorf("only %d MPEG frames found; not an MP3 file?", frames)
	}
	return seconds, nil
}
