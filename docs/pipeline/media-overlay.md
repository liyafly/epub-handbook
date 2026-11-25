# 有声同步（Media Overlays）

> 状态：已实现（2026-11-25），阅读器实测待记录。用法见 [做有声同步版](../how-to/read-along.md)，硬规则见 [SPEC-实现约束 §5.14](../final/SPEC-实现约束.md)。本页是维护者参考。

把「EPUB + 按章分开的有声书音频」做成逐句高亮跟读的 EPUB3。原型在一本英文插图小说上跑通：whisper.cpp + `small.en` 模型，各章 97–99% 的书面词与识别词对上。书级证据留在本地。

## 两部分

| 部分 | 做什么 | 依赖 |
| --- | --- | --- |
| `epub book transcribe` | 每个音频文件跑一次 `whisper-cli`，把结果规范成时间轴 JSON，缓存到 `.epub/asr/` | `whisper-cli` + 模型文件，经 `internal/extern` |
| `epub run epub.media.overlay.build` | 输入 EPUB、音频目录、时间轴目录，写出带同步的 EPUB | 无，纯 Go |

时间轴格式：[`contracts/schemas/v2/word-timings.schema.json`](../../contracts/schemas/v2/word-timings.schema.json)，每个音频一个文件。其他识别器只要写出同样的格式就能用。

缓存键：音频 SHA + 模型 SHA + 引擎版本。

## `epub.media.overlay.build`

参数（`key=value`）：`audio_dir`、`timings_dir`、`audio_map`（可选，XHTML → 音频）、`granularity=sentence|paragraph`（默认 sentence）、`narrator`。

```sh
epub run epub.media.overlay.build --input book.epub --output book-readalong.epub --json \
  audio_dir=素材/audio timings_dir=.epub/asr narrator="…"
```

- 默认配对：spine 里标了 `epub:type="chapter"`、且至少 50 个词的文档，按顺序对应 `audio_dir` 里按文件名排序的 `.mp3` / `.m4a`。数量不一致就报错，改用 `audio_map`。
- 时间轴按音频 SHA-256 匹配，与 JSON 文件名无关。
- 新 entry：`<OPF 目录>/Audio/<章节文件名>.<扩展名>`、`<OPF 目录>/Smil/<章节文件名>.smil`；高亮规则追加到第一章最后一个样式表。

1. **扫描**
   - 分句：断句只看 `<p>` 直属的文字，行内元素整个包进句子 span；脚注区、nav、figure 不切。章题 `h1` 取自身文字或图片 `alt`。
   - 对齐：词规范化（NFKC、小写、去撇号、数字转词），再做 Myers diff。句子起点取第一个对上的词，句子之间连续接上，空白超过 2 秒截断。没对上的句子插值。
   - 生成 SMIL。
   - figure 链接到转写弹注时，用注释文字参与对齐，SMIL 指向 figure。
2. **应用**：XHTML span edits；OPF edits（manifest、`media-overlay`、`media:duration`、`media:active-class`）；CSS 高亮规则；新增 SMIL 和音频 entry，音频不压缩。
3. **报告**：facts `epub.media.overlay.build.chapters` 记每章的词数、匹配率、起止时间和弱句数。

| finding | 级别 |
| --- | --- |
| 某章匹配率低于 90% | error |
| 音频没有对应章节 | error |
| 弱句（对上的词不到一半） | warn |
| 片头、片尾没覆盖 | info |

nav audit 增加 SMIL 检查：`text@src` 能解析；clip 单调递增、不超过音频时长；`media:duration` 的合计一致。不新增红线。

## 音频

- 支持 MP3（`audio/mpeg`）和 M4A/AAC（`audio/mp4`），原样放进包。时长写在时间轴里：MP3 由 Go 逐帧计算，M4A 取 `ffprobe` 报的容器时长。
- 单个 m4b（或带章节的 m4a / mp4）：`transcribe` 用 `ffprobe -show_chapters` 读章节标记，`ffmpeg -c copy` 按章切成 `<状态目录>/audio/<m4b SHA 前 16 位>/NNN.m4a`。只复制 AAC 数据，不重新编码；先写临时目录，全部成功再改名到位。构建从这个目录取音频，没切过就当作缺时间轴。
- `whisper-cli` 能直接读 MP3、WAV、FLAC，读不了 AAC：M4A 先由 ffmpeg 解码成 16 kHz 单声道 WAV 再识别，时间轴不变，WAV 用完即删。FFmpeg 只在有 m4b / M4A 时才要求。
- 限额：整包 2 GiB、单个文件 256 MiB、解压总量 4 GiB（`zipfs.DefaultLimits`）。EPUB 规范本身没有上限，这是本项目的安全阀。新加的音频由 `book.AddFile` 登记磁盘路径，写包时直接流式写入并存储不压缩，不占 512 MiB 内存缓存。

## 书级接入

```json
"readAlong": {"audio": "素材/audio", "model": "<模型路径>", "narrator": "…"},
"variants": {"readalong": {"mediaOverlay": true}}
```

- 跟读只做成变体，通用版不带音频。构建在字体子集之后对变体候选跑能力，之后照常做导航审计和红线。
- 构建时缺时间轴缓存：这个变体跳过并报 WARN，其他输出照常；`--variant` 只建它时失败。
- 音频原件放 `素材/audio/`，识别缓存放状态目录 `asr/`（新布局 `.epub/asr/`）。
- `transcribe` 用 Go 逐帧算 MP3 时长写进时间轴，whisper.cpp 的 token 以空格开头的开新词，`[_BEG_]` 之类的特殊 token 丢掉。
- 词时间用 whisper.cpp 的 DTW 对齐（`--dtw <模型尺寸> -nfa`，取 `t_dtw`）：默认的逐 token 估计常把一整段的词都压到段首，句子高亮会在段里停住几秒。模型尺寸从文件名认（`ggml-small.en.bin` → `small.en`），认不出就退回 token 时间并 WARN。缓存记 `engine.wordTiming`，算法变了 `transcribe` 会重做。
- 插图段（如朗读整页手稿时指向 `figure`）的高亮用描边：图片会盖住背景色。
- 中日文按字对齐（汉字、假名逐字成词）；要用多语言模型。

## 跨平台

| 平台 | whisper.cpp |
| --- | --- |
| macOS | `brew install whisper-cpp` |
| Windows | 官方 Release 的 `whisper-bin-x64.zip`，`whisper-cli.exe` 放进 PATH |
| Linux | 发行版包或源码编译 |

m4b / M4A 另需 FFmpeg：macOS `brew install ffmpeg`，Windows `winget install ffmpeg`，Linux 发行版包。
模型（`ggml-base.en.bin` 约 148 MB、`ggml-small.en.bin` 约 488 MB）由用户下载。CLI 不联网。工具缺失时按 SPEC-Go 架构 §6.4 显式降级。

## 不做

- 免模型的 TTS + DTW 对齐。原型实测只有约 70% 的句子起点落在停顿附近（随机放约 29%），章末还会丢段。
- CLI 内下载模型或调用在线识别。

## 阶段

单个 m4b 于 2026-11-25 补上。S1–S5 已于 2026-11-24/25 完成：时间轴 schema 与规则；能力本体（含 2 GiB 限额与 `book.AddFile`）；`epub book transcribe`；nav audit 的 SMIL 检查与 `book.json` 接入；how-to、FAQ、skill、手册与速查表。

## 待办

- Reeden、Readest 实测：记版本、截图，写进 `reader-matrix.yaml`。
- nav audit 不核对 clip 是否超过音频长度（能力写 SMIL 时已按时间轴里的时长封顶）。
