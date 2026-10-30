# 书级图片辅助工具

`epub-art` 是独立 Python/Pillow 项目，不进入 Go 发行包，也没有注册 Go capability。
它生成候选和报告，不直接改书；多书验证后若要接入只读能力，另按 Go SPEC §6.1 建契约和 golden。

```sh
uv tool install --editable tools-art/epub-art
epub-art audit illustration.png
epub-art shape illustration.png --float left --selector figure.outline --output shape.json --css shape.css
epub-art transparent original.jpg --output candidate.png
epub-art transparent line-art.png --method white-matte --protect-mask keep.png --output candidate.png
# 在本项目目录运行测试，避免扫描独立字体项目
uv run --directory tools-art/epub-art --frozen pytest -q
```

`shape` 将 alpha 二值化，逐行取正文侧边缘，扩张像素安全区，再做 Douglas–Peucker 简化。
简化后复核每段的向内误差并向外补偿，输出 content-box 百分比、输入 SHA、参数和 CSS。
左浮动描右边缘，右浮动描左边缘；其余侧闭合到盒边。默认 padding 1px、阈值 16、简化容差 1.5px，超过 40 点会提示人工检查，不以过度简化换取固定点数。
坐标必须匹配原图比例；figure 不放图注，保留矩形 fallback 的普通侧边 margin。生成坐标不是阅读器验收。

`transparent` 默认只移除与图像边缘连通的近白区域，保留封闭的白色物体内部，保护蒙版非零像素保持原样。
`white-matte` 仅适合已确认白色均为底色的线稿，按白底恢复公式生成 RGB/alpha；白色物体也可能消失，须显式选择并检查。
这是书级既有“边缘纸区”和“白底线稿恢复”两类算法的通用部分；书内专用蒙版、颜色判断和人工决策仍留书级，不自动重放到其他图。
默认不反色、不裁切、不缩放、不覆盖输入；同名输出须显式 `--force`，输入始终不可作为输出。

`audit` 报告尺寸、比例、透明像素、模式和 ICC 存在性。ICC 存在不证明为 sRGB；CMYK 等输入须先有意转换颜色空间，透明化拒绝猜转。
所有候选都要核对浅色、彩色、深色背景和线条/封闭白区，之后再做目标阅读器检查，不全书盲目批处理。
