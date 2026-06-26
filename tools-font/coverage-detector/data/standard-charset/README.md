# 标准字表数据（可选）

默认的「标准字区」判定由 GB2312 / GBK 编解码器在运行时生成，**不需要任何文件**。
本目录仅用于**可选**的自备字表覆盖（如《通用规范汉字表》8105 字），命中者会被当作「至少 GBK 级、非生僻」。

## 文件格式

- UTF-8 纯文本，扩展名 `.txt`。
- 文件中**每一个非空白字符**都算作成员（按 Unicode 码位）。
- 以 `#` 开头的行视为注释，整行跳过。字符可任意分行/空格排布。

## 用法

    # 默认（无文件，用 GB2312/GBK）
    uv run python -m src.cli book.epub -o report.json
    # 可选：叠加自备字表
    uv run python -m src.cli book.epub --standard-table data/standard-charset/tongyong-2013.txt -o report.json
