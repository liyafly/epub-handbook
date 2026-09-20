---
name: epub-structure-normalizer
description: 把 EPUB 资源目录先归类，再按 OPF manifest id 还原混淆文件名并同步引用。用于目录混乱或需要稳定 diff；写新候选，需审查路径映射，不是 EPUB3 迁移或 DRM 解密。
---

# EPUB 结构规范化

## 何时用

只处理内部资源路径：先 format 保留文件名并归类目录，再 deobfuscate 按 manifest id 改名。正文和字体字节不变；包迁移另走 migrator。预检与写入保护见根 AGENTS。

## 调什么

```sh
epub run epub.structure.normalize --input "before.epub" --output "normalized.epub" --dry-run --json
# 人工审查两阶段映射后实跑；报告目录须已存在
epub run epub.structure.normalize --input "before.epub" --output "normalized.epub" --json > normalize-envelope.json
epub redline --check all --path-map normalize-envelope.json "before.epub" "normalized.epub"
```

排障才用 `mode=format|deobfuscate|inspect`。`mode=inspect` 仍属单输出调用，非 dry-run 会写未修改副本，不称“无输出只读”。明确授权且确认标准字体混淆时，运行加 `allow_font_obfuscation=true`，红线加 `--allow-font-obfuscation`；不能用于未知加密。

## 返回怎么读

前缀 `epub.structure.normalize.`：`mappings[]`（from/to）、`warnings[]`、`movedResources`、`renamedResources`、`rewrittenFiles`、`fontObfuscationResources`、`removedStaleEncryptionResources`；默认双阶段另有 `stages[]`。保存完整信封作为 path-map，不手抄映射。公共语义见 [索引](../README.md)。

## 依据返回怎么判断

- dry-run 在内存完成两个阶段并检查真实候选，mappings 与候选一致；不写文件。审查映射/冲突/警告及红线后实跑，产物再用同次实跑信封作 path-map 复核。预览出现红线 error 不能按“尚未应用”忽略。
- `markup scan stopped at byte offset N` 表示后续引用未改写：先修源，不能直接接受候选。其他断链逐项检查。
- 缺失字体 URL 不猜文件、不删声明，保留 `local()` fallback；非字体资源断链须修复。stale encryption 只移除目标已不存在的引用。
- 真实未知加密停止；保守重写失败不一概推断是 DRM，读取具体 events/findings。实跑红线失败保留候选供 diff，不覆盖原件。通过后人工检查路径/链接，再判断是否需要 EPUB3 迁移。
