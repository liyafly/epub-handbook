# 轮廓环绕

## 效果

连续正文沿透明插图的显式 polygon 排列。基础宽度先保证矩形也可读；不要靠阅读器解析图片 alpha 作为主路径。规则来源：[SPEC §5.6.2](../../../docs/final/SPEC-实现约束.md#562-异形环绕渐进增强)。

## 标记结构

见 [sample.xhtml](sample.xhtml)：`figure.wrap.wrap-left.outline-l` 只含一张图片，紧接连续正文；不把正文塞进左右栏容器。图注不能进入轮廓 figure，带图注的图保留普通块图或矩形浮动。当前坐标只对应随附合成 L 图；换图须重新算。

## 增强层

[recipe.css](recipe.css) 四层依次为块图、矩形 float、显式 `content-box` polygon、窄屏护栏。普通侧 margin 保住矩形间距；`shape-margin` 处理轮廓间距。不得加改变参考盒的 padding/border，不用 `@supports` 改宽度。右侧需同时镜像图片和 x 坐标；可用 [epub-art](../../../tools-art/README.md) 生成候选，再人工检查。

## 降级行为

忽略 shape 时保持矩形 float；窄于 20em 时回块图。忽略 float 时仍是普通图片后跟正文。20em 只是候选阈值，大字号和短段落要独立复测，不能把历史普通字号通过转签给护栏。

## 阅读器结果

历史 v3 观察继续保留。当前配方在 Readest 0.12.16 桌面配置通过；Kindle Previewer 4.0.3 Song 8 退回矩形，记 warn。Apple Books 9.0 当前样本首个展开页的凹口绕排与图后整行恢复可见，续页未覆盖。Readest 改为单栏、最大列宽280px后仍沿 polygon 绕排：分页栏宽不等于窗口宽，20em媒体查询未触发，L3不能标为通过。目标阅读器窄栏无法触发护栏时，应逐书评估普通块图。详见 [补验日志](../reader-evidence/observations-20261030-backing.json)、[早期观察](../reader-evidence/observations-20261030.json) 和 [阅读器矩阵](../../../docs/final/reader-matrix.yaml)。不把合成样本范围外推到其他书或实体设备。
