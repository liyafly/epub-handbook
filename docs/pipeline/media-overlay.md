# 有声同步（Media Overlays）

> 状态：开发中（S1：契约前置与规则）。硬规则见 [SPEC-实现约束 §5.14](../final/SPEC-实现约束.md)。

把「EPUB + 按章分开的有声书音频」做成逐句高亮跟读的 EPUB3。原型在一本英文插图小说上跑通：whisper.cpp + `small.en` 模型，各章 97–99% 的书面词与识别词对上。书级证据留在本地。

## 两部分

| 部分 | 做什么 | 依赖 |
| --- | --- | --- |
| `epub book transcribe` | 每个音频文件跑一次 `whisper-cli`，把结果规范成时间轴 JSON，缓存到 `.epub/asr/` | `whisper-cli` + 模型文件，经 `internal/extern` |
| `epub run epub.media.overlay.build` | 输入 EPUB、音频目录、时间轴目录，写出带同步的 EPUB | 无，纯 Go |

时间轴格式：[`contracts/schemas/v2/word-timings.schema.json`](../../contracts/schemas/v2/word-timings.schema.json)，每个音频一个文件。其他识别器只要写出同样的格式就能用。

缓存键：音频 SHA + 模型 SHA + 引擎版本。

## `epub.media.overlay.build`

参数：`--audio-dir`、`--audio-map`（可选，XHTML → 音频）、`--timings-dir`、`--granularity sentence|paragraph`（默认 sentence）、`--narrator`。

1. **扫描**
   - 分句：断句只看 `<p>` 直属的文字，行内元素整个包进句子 span；脚注区、nav、figure 不切。章题 `h1` 取自身文字或图片 `alt`。
   - 对齐：词规范化（NFKC、小写、去撇号、数字转词），再做 Myers diff。句子起点取第一个对上的词，句子之间连续接上，空白超过 2 秒截断。没对上的句子插值。
   - 生成 SMIL。
   - figure 链接到转写弹注时，用注释文字参与对齐，SMIL 指向 figure。
2. **应用**：XHTML span edits；OPF edits（manifest、`media-overlay`、`media:duration`、`media:active-class`）；CSS 高亮规则；新增 SMIL 和音频 entry，音频不压缩。
3. **报告**：facts `epub.media.overlay.chapters` 记每章的词数、匹配率、起止时间和弱句数。

| finding | 级别 |
| --- | --- |
| 某章匹配率低于 90% | error |
| 音频没有对应章节 | error |
| 弱句（对上的词不到一半） | warn |
| 片头、片尾没覆盖 | info |

nav audit 增加 SMIL 检查：`text@src` 能解析；clip 单调递增、不超过音频时长；`media:duration` 的合计一致。不新增红线。

## 音频

- 支持 MP3（`audio/mpeg`）和 M4A/AAC（`audio/mp4`），原样放进包。时长由 Go 解析帧头或 `mvhd`。
- `whisper-cli` 能直接读 MP3、WAV、FLAC，读不了 AAC。第一版要求按章的 MP3。单个 m4b 以后再做，需要可选的 ffmpeg。
- 限额：现有 `zipfs.DefaultLimits` 是整包 512 MB、单个文件 256 MB。EPUB 规范本身没有上限，这是本项目的安全阀。带音频的书怎么放宽，在 S2 里定。

## 书级接入

```json
"readAlong": {"audio": "素材/audio", "model": "<模型路径>", "narrator": "…"},
"variants": {"readalong": {"mediaOverlay": true}}
```

- 跟读只做成变体，通用版不带音频。
- 构建时缺时间轴缓存：这个变体跳过并报 warn，其他输出照常。
- 音频原件放 `素材/audio/`，识别缓存放 `.epub/`。

## 跨平台

| 平台 | whisper.cpp |
| --- | --- |
| macOS | `brew install whisper-cpp` |
| Windows | 官方 Release 的 `whisper-bin-x64.zip`，`whisper-cli.exe` 放进 PATH |
| Linux | 发行版包或源码编译 |

模型（`ggml-base.en.bin` 约 148 MB、`ggml-small.en.bin` 约 488 MB）由用户下载。CLI 不联网。工具缺失时按 SPEC-Go 架构 §6.4 显式降级。

## 不做

- 免模型的 TTS + DTW 对齐。原型实测只有约 70% 的句子起点落在停顿附近（随机放约 29%），章末还会丢段。
- CLI 内下载模型或调用在线识别。

## 阶段

| 阶段 | 内容 |
| --- | --- |
| S1 | 时间轴 schema、本页、SPEC §5.14 |
| S2 | 契约、参数和能力本体，加 golden 和原型对照 |
| S3 | extern whisper-cli、`epub book transcribe` |
| S4 | nav audit SMIL 检查、`book.json` 接入 |
| S5 | how-to（双语）、skill（并入 `epub-package-ops`）、FAQ、手册和速查表、reader-matrix |
