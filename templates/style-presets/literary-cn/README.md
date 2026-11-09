# literary-cn

适合小说、散文、书信集。正文默认自由模式（需要锁定时取消 `fonts.css` 中
直接 `body` 规则的注释，普通 `p` 继承），章首留白舒展，题记和引文要换字形时，打包楷体后打开 `.font-kt` 再挂上，
信件场景的 `.note-box` 使用低对比细边框。`.chapter-head-art` 提供章首图槽位，
但不包含位图素材。

书信、公文：抬头用 `.letter-salutation`（顶格），文件抬头用 `.doc-heading`（居中），
要用楷体就打包楷体、打开 `.font-kt`，再加到 `.letter` 容器上。正文里的人物旁白、舞台说明这类括注，
用 `span.aside-inline` 缩小字号，不锁颜色。
