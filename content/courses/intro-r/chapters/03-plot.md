---
title: 可视化基础
---

# 可视化基础

本节使用 `ggplot2` 绘制收益率分布与分组箱线图。

```r
library(ggplot2)
ggplot(mtcars, aes(factor(cyl), mpg)) +
  geom_boxplot()
```
