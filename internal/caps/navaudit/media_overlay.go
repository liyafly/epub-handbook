package navaudit

import (
	"context"
	"fmt"
	"math"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/liyafly/epub-handbook/internal/scan/opf"
)

// checkMediaOverlays 检查 EPUB3 Media Overlays（SPEC-实现约束 §5.14）：
// media-overlay 指向 SMIL item；SMIL 的 text@src 能解析到文档里的 id；audio@src
// 在包内；clipBegin < clipEnd 且单调；media:duration 与 SMIL 合计一致。
// 音频本身的长度不在这里核对（由写出 SMIL 的能力按时间轴封顶）。
func (ins *inspector) checkMediaOverlays(ctx context.Context, pkg *opf.Package) {
	raw, err := ins.b.CurrentContext(ctx, ins.opfPath)
	if err != nil {
		return
	}
	tree, err := opf.ScanSpanTreeContext(ctx, raw)
	if err != nil {
		return
	}
	byID := make(map[string]opf.ManifestItem, len(pkg.Manifest))
	for _, item := range pkg.Manifest {
		byID[item.ID] = item
	}
	durations := map[string]string{}
	total := ""
	activeClass := false
	for _, m := range pkg.Metas {
		switch m.Property {
		case "media:duration":
			if m.Refines == "" {
				total = m.Text
			} else {
				durations[strings.TrimPrefix(m.Refines, "#")] = m.Text
			}
		case "media:active-class":
			activeClass = true
		}
	}
	idCache := map[string]map[string]bool{}
	var sum float64
	count := 0
	for _, n := range opf.ManifestNodes(tree) {
		if ctx.Err() != nil {
			return
		}
		smilID, ok := n.AttrByLocal("", "media-overlay")
		if !ok {
			continue
		}
		owner, _ := n.AttrByLocal("", "href")
		ownerPath, _ := opf.ResolveHref(ins.opfPath, owner)
		smil, ok := byID[smilID]
		if !ok || smil.MediaType != "application/smil+xml" {
			ins.addFinding("error", "media-overlay does not name a manifest item of type application/smil+xml: "+smilID, ownerPath, "media-overlay-missing-smil")
			continue
		}
		count++
		dur, ok := ins.checkSMIL(ctx, smil.ArchivePath, ownerPath, idCache)
		if !ok {
			continue
		}
		sum += dur
		declared, has := durations[smilID]
		if !has {
			ins.addFinding("warn", "SMIL has no media:duration (refines=\"#"+smilID+"\")", smil.ArchivePath, "media-overlay-duration")
			continue
		}
		if v, err := parseClock(declared); err != nil || math.Abs(v-dur) > 1 {
			ins.addFinding("warn", fmt.Sprintf("media:duration %s differs from the SMIL clips (%.3fs)", declared, dur), smil.ArchivePath, "media-overlay-duration")
		}
	}
	if count == 0 {
		return
	}
	if total == "" {
		ins.addFinding("warn", "book has media overlays but no total media:duration", ins.opfPath, "media-overlay-duration")
	} else if v, err := parseClock(total); err != nil || math.Abs(v-sum) > 1 {
		ins.addFinding("warn", fmt.Sprintf("total media:duration %s differs from the sum of SMIL clips (%.3fs)", total, sum), ins.opfPath, "media-overlay-duration")
	}
	if !activeClass {
		ins.addFinding("info", "media overlays without media:active-class; readers use their own highlight", ins.opfPath, "media-overlay-active-class")
	}
}

var xmlIDRe = regexp.MustCompile(`\sid\s*=\s*["']([^"']+)["']`)

func (ins *inspector) checkSMIL(ctx context.Context, smilPath, ownerPath string, idCache map[string]map[string]bool) (float64, bool) {
	data, err := ins.b.CurrentContext(ctx, smilPath)
	if err != nil {
		ins.addFinding("error", "SMIL listed in the manifest is missing from the package", smilPath, "media-overlay-missing-smil")
		return 0, false
	}
	tree, err := opf.ScanSpanTreeContext(ctx, data)
	if err != nil {
		ins.addFinding("error", "SMIL could not be parsed: "+err.Error(), smilPath, "media-overlay-smil-parse")
		return 0, false
	}
	dir := path.Dir(smilPath)
	var total, prevBegin float64
	prevAudio := ""
	bad := 0
	report := func(level, msg string) {
		if bad < 5 {
			ins.addFinding(level, msg, smilPath, "media-overlay-clip")
		}
		bad++
	}
	for _, par := range tree.Walk() {
		if par.Name.Local != "par" {
			continue
		}
		var text, audio = par.ChildByAnyNS("text"), par.ChildByAnyNS("audio")
		if text == nil {
			report("error", "par without <text>")
			continue
		}
		src, _ := text.AttrByLocal("", "src")
		file, frag, _ := strings.Cut(src, "#")
		target := path.Join(dir, file)
		if decoded, err := urlPathUnescape(target); err == nil {
			target = decoded
		}
		if target != ownerPath {
			report("warn", "text@src points outside the document that owns this SMIL: "+src)
		}
		ids, ok := idCache[target]
		if !ok {
			doc, err := ins.b.CurrentContext(ctx, target)
			if err == nil {
				ids = map[string]bool{}
				for _, m := range xmlIDRe.FindAllSubmatch(doc, -1) {
					ids[string(m[1])] = true
				}
			}
			idCache[target] = ids
		}
		if ids == nil || (frag != "" && !ids[frag]) {
			report("error", "text@src does not resolve: "+src)
		}
		if audio == nil {
			continue
		}
		asrc, _ := audio.AttrByLocal("", "src")
		apath := path.Join(dir, asrc)
		if decoded, err := urlPathUnescape(apath); err == nil {
			apath = decoded
		}
		if !hasEntry(ins.b, apath) {
			report("error", "audio@src is missing from the package: "+asrc)
		}
		bs, _ := audio.AttrByLocal("", "clipBegin")
		es, _ := audio.AttrByLocal("", "clipEnd")
		b, errB := parseClock(bs)
		e, errE := parseClock(es)
		if errB != nil || errE != nil || e <= b {
			report("error", fmt.Sprintf("clip %q–%q is not a positive time range", bs, es))
			continue
		}
		if apath == prevAudio && b < prevBegin {
			report("warn", fmt.Sprintf("clipBegin %s goes backwards in the same audio", bs))
		}
		prevAudio, prevBegin = apath, b
		total += e - b
	}
	if bad > 5 {
		ins.addFinding("warn", fmt.Sprintf("%d more SMIL clip problems not listed", bad-5), smilPath, "media-overlay-clip")
	}
	return total, true
}

// parseClock 解析 SMIL 时钟值：12.5s、300ms、1.5min、0.2h、hh:mm:ss.f、mm:ss.f 或纯秒数。
func parseClock(v string) (float64, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, fmt.Errorf("empty clock value")
	}
	for _, unit := range []struct {
		suffix string
		scale  float64
	}{{"ms", 0.001}, {"min", 60}, {"h", 3600}, {"s", 1}} {
		if num, ok := strings.CutSuffix(v, unit.suffix); ok {
			f, err := strconv.ParseFloat(num, 64)
			return f * unit.scale, err
		}
	}
	parts := strings.Split(v, ":")
	if len(parts) > 3 {
		return 0, fmt.Errorf("bad clock value %q", v)
	}
	var total float64
	for _, p := range parts {
		f, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return 0, fmt.Errorf("bad clock value %q", v)
		}
		total = total*60 + f
	}
	return total, nil
}

func urlPathUnescape(p string) (string, error) {
	if !strings.Contains(p, "%") {
		return p, nil
	}
	var b strings.Builder
	for i := 0; i < len(p); i++ {
		if p[i] == '%' && i+2 < len(p) {
			n, err := strconv.ParseUint(p[i+1:i+3], 16, 8)
			if err != nil {
				return p, err
			}
			b.WriteByte(byte(n))
			i += 2
			continue
		}
		b.WriteByte(p[i])
	}
	return b.String(), nil
}
