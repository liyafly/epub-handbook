#!/usr/bin/env bash
# 有声同步端到端冒烟：真实 whisper-cli + ffmpeg/ffprobe，单个 m4b（带章节标记）→
# epub book transcribe → epub book build，检查 readalong 变体里有 SMIL、音频和句子 span。
#
# 用法：readalong-smoke.sh <epub 可执行文件> <工作目录> <ggml 模型> <语音 wav>
# 需要 PATH 上有 whisper-cli（或 EPUB_WHISPER_BIN）、ffmpeg、ffprobe 和 Python（PYTHON 可指定）。
# 语音用 whisper.cpp 自带的 samples/jfk.wav（公有领域），书的正文就是它的原话。
set -euo pipefail

epub=$1 work=$2 model=$3 wav=$4
book="$work/read along"
rm -rf "$work" && mkdir -p "$work/whisper-models"
cp "$model" "$work/whisper-models/"
"$epub" book new "$book"

cat > "$book/epub/OEBPS/Text/01-chapter.xhtml" <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="en" lang="en">
<head>
  <title>Chapter One</title>
  <link rel="stylesheet" type="text/css" href="../Styles/base.css"/>
  <link rel="stylesheet" type="text/css" href="../Styles/design.css"/>
  <link rel="stylesheet" type="text/css" href="../Styles/reader.css"/>
</head>
<body>
  <section epub:type="chapter">
    <h1>And so, my fellow Americans</h1>
    <p>Ask not what your country can do for you. Ask what you can do for your country.</p>
  </section>
</body>
</html>
EOF

# 单个 m4b：AAC + 一个章节标记（ffmetadata），由 transcribe 按标记切分。
mkdir -p "$book/素材"
printf ';FFMETADATA1\n[CHAPTER]\nTIMEBASE=1/1000\nSTART=0\nEND=11000\ntitle=001\n' > "$work/chapters.txt"
ffmpeg -nostdin -v error -y -i "$wav" -i "$work/chapters.txt" -map 0:a -map_chapters 1 -c:a aac -b:a 64k -f mp4 "$book/素材/book.m4b"
ffprobe -v error -show_chapters -of csv "$book/素材/book.m4b"

cat > "$book/book.json" <<EOF
{
  "version": 1,
  "title": "read along",
  "output": "book.epub",
  "fonts": [],
  "readAlong": {
    "audio": "素材/book.m4b",
    "model": "$(basename "$model")",
    "language": "en",
    "audioMap": "素材/audio-map.json"
  },
  "variants": {"readalong": {"mediaOverlay": true}}
}
EOF
printf '{"Text/01-chapter.xhtml": "001.m4a"}\n' > "$book/素材/audio-map.json"

"$epub" book transcribe "$book"
"$epub" book build "$book" | tee "$work/build.log"
grep -q "PASS epub.media.overlay.build \[readalong\]" "$work/build.log"

out="$book/out/book-readalong.epub"
test -f "$out"
# 用 Python 读 zip：Windows 的 Git Bash 不一定带 unzip。
"${PYTHON:-python3}" - "$out" <<'PY'
import sys, zipfile
z = zipfile.ZipFile(sys.argv[1])
names = set(z.namelist())
assert "OEBPS/Smil/01-chapter.smil" in names and "OEBPS/Audio/01-chapter.m4a" in names, sorted(names)
assert 'id="mo-01-chapter-s' in z.read("OEBPS/Text/01-chapter.xhtml").decode()
pars = z.read("OEBPS/Smil/01-chapter.smil").decode().count("<par ")
print("pars:", pars)
assert pars >= 3, pars
PY
# 词时间走 DTW，缓存里应记 wordTiming=dtw。
grep -q '"wordTiming":"dtw"' "$book"/.epub/asr/*.json
echo "read-along smoke: OK"
