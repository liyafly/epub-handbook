# 第三方 EPUB 样本（预留）

本目录预留给未来的真实公版或明确授权 EPUB 样本。当前 Stage 4 主 demo 已改为 [samples/demo-books/](../demo-books/) 下的自造 EPUB，避免把繁体、版式较单薄的公版书作为首轮展示材料。

## 收录规则

- 只接受公版书或明确许可（CC0 / CC-BY / GFDL / Project Gutenberg）。
- 每本书一个子目录：`samples/third-party/<slug>/`。
- 子目录必须有：
  - `LICENSE.txt`：写明公版状态 + 原始许可。
  - `metadata.yaml`：来源 URL、抓取日期、SHA-256。
  - `fetch.sh`：下载 epub 并校验哈希。
  - `notes.md`：清洗过程记录、diff 关键发现。
- 实体 `.epub` 文件不入 git。

## 现有样本

暂无。新增第三方样本前，先确认来源、许可、语言版本和版式价值。

## 复现样本

```sh
cd samples/third-party/<slug>/
bash fetch.sh
```

## 同步 THIRD_PARTY.md

每新增一本样本，必须同步更新仓库根的 [THIRD_PARTY.md](../../THIRD_PARTY.md)。
