# Readest 导入身份来源说明

查阅日期：2026-10-29。发布方/作者：Readest 项目及贡献者。核对 tag `v0.12.16`，对应本地制作记录中的应用版本；远端源码只能解释可能路径，不能证明已安装二进制的全部内部行为。

| 一手来源 | 本次使用的事实 |
| --- | --- |
| [utils/md5.ts](https://github.com/readest/readest/blob/v0.12.16/apps/readest-app/src/utils/md5.ts) | `partialMD5` 抽取文件若干区域计算摘要；另有整文件函数，二者用途不可混同 |
| [services/bookService.ts](https://github.com/readest/readest/blob/v0.12.16/apps/readest-app/src/services/bookService.ts) | 导入路径计算文件 hash，并结合现有书库与元数据匹配；只修改 OPF identifier 不能证明所有身份键均已隔离 |
| [LICENSE](https://github.com/readest/readest/blob/v0.12.16/LICENSE) | 上游源码采用 GNU AGPL v3；本仓未复制或移植其实现 |

本页仅保存链接与自行撰写的行为摘要。没有镜像源码、软件二进制或用户书库文件；完整来源与保留理由记在 [THIRD_PARTY.md](../THIRD_PARTY.md)。

本地案例记录了相同 partial-MD5 / 原生 URL、重新隔离后复现及最终输入确认，可复用方法见 [匿名复盘 §4](../docs/pipeline/reader-adaptation-lessons-2026-10.md#4-readest先排除旧内容再判断-css-是否有效)。具体书名、文件指纹、定位和私人证据链接不公开。更换测试 identifier、调整测试 ZIP 顺序和摘要差异都只是准备步骤，导入后仍要确认真实条目/载荷；其他 Readest 版本须重新核对，不能把本页当跨版本算法承诺。
