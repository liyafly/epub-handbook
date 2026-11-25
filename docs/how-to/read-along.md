# 做有声同步版（跟读高亮）

把一本书和它的有声书（每章一个 MP3）做成边听边高亮的 EPUB3（Media Overlays）。规则见 [SPEC §5.14](../final/SPEC-实现约束.md)，实现细节见 [media-overlay.md](../pipeline/media-overlay.md)。

## 准备

| 东西 | 说明 |
| --- | --- |
| whisper.cpp | macOS：`brew install whisper-cpp`；Windows：官方 Release 的 `whisper-bin-x64.zip`，把 `whisper-cli.exe` 放进 PATH；Linux：发行版包或源码编译。路径不在 PATH 时设 `EPUB_WHISPER_BIN`。 |
| 模型 | 从 `huggingface.co/ggerganov/whisper.cpp` 下载，别改文件名（要靠它认出模型尺寸做精确的词时间）。英文书用 `ggml-small.en.bin`（约 488 MB）或 `ggml-base.en.bin`（约 148 MB）；中文书用多语言的 `ggml-small.bin` 等。 |
| 音频 | 两种都行：每章一个 MP3 / M4A 的文件夹（文件名排序和章节顺序一致），或带章节标记的单个 m4b。原件放 `素材/`。 |
| FFmpeg | 只有 m4b、M4A 才要。macOS：`brew install ffmpeg`；Windows：`winget install ffmpeg`。不在 PATH 时设 `EPUB_FFMPEG_BIN`、`EPUB_FFPROBE_BIN`。 |

## 配置

`book.json`（旧布局写在制作区 `variants.json`，路径相对制作区）：

```json
"readAlong": {
  "audio": "素材/audio",
  "model": "~/models/ggml-small.en.bin",
  "language": "en",
  "narrator": "朗读者名"
},
"variants": {
  "readalong": {"mediaOverlay": true}
}
```

- `audio` 可以是文件夹，也可以直接写 m4b 文件，如 `"素材/book.m4b"`。
- `language` 省略时由 whisper 自动判断。`granularity` 可设 `paragraph`（按段高亮，默认按句）。
- 章节默认取 spine 里标了 `epub:type="chapter"`、且至少 50 个词的文档。数量和音频对不上时，写 `"audioMap": "素材/audio-map.json"`，内容是 `{"Text/ch1.xhtml": "001.mp3", …}`。
- 跟读只做成变体：通用版和 Kindle 版不带音频。

## 步骤

```sh
epub book transcribe          # m4b 先按章节标记切开；每个音频识别一次，结果缓存在 .epub/asr/，再跑会跳过
epub book build               # 通用版 + readalong 变体
```

- m4b 切开时只复制音频数据，不重新编码，几秒完成，放在 `.epub/audio/`。换了 m4b 会重新切。
- 识别时间：M1 上用 small.en，约 4 小时的书大概 15–25 分钟。
- 没跑过 transcribe 就 build：通用版照常出，readalong 变体跳过并提示。
- `epub book preview` 不加音频。要听效果，用 build 出的 `out/*-readalong.epub`。

## 看结果

构建输出里 `epub.media.overlay.build [readalong]` 一行就是结果，明细在 `.epub/media-overlay.readalong.json`：

| 提示 | 意思 | 怎么办 |
| --- | --- | --- |
| `low-match`（error） | 某章对上的词不到 90% | 多半音频顺序错了，或这章音频是别的书，检查文件名排序或写 `audioMap` |
| `weak-segments`（warn） | 个别句子几乎没对上，时间是按前后句推算的 | 通常是没念的章题、一两个词的喊叫，听一下即可 |
| `uncovered-edges`（info） | 片头或片尾有声音但不播放 | 正常：片头、片尾的制作信息不对应正文 |

## 阅读器

- Reeden：能播放并高亮（所有者口头反馈，待记录版本和截图）。
- Readest：待测。
- Apple Books：可重排的同步版没找到播放入口，不作目标。
- Kindle：不支持，不作目标。

实测后按 [阅读器闭环](../../AGENTS.md) 记到 `docs/final/reader-matrix.yaml`。

## 体积

音频原样放进包，四小时左右的 128 kbps 有声书约 230 MB。本项目对 EPUB 的上限是 2 GiB，单个文件 256 MiB。阅读器自己可能有更低的限制。
