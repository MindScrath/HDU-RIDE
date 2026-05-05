---
title: 数据框与 tibble
---

# 数据框与 tibble

数据框是 R 中最常用的表格数据结构。请在 RStudio 中运行：

```r
df <- data.frame(
  symbol = c("A", "B", "C"),
  ret = c(0.012, -0.004, 0.018)
)
summary(df$ret)
```

## 课堂任务

读取 CSV，检查列名、缺失值和基本统计量。
