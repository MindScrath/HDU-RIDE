---
title: CAPM 模型与实证检验
---

# CAPM 模型与实证检验

## 什么是 CAPM

资本资产定价模型（Capital Asset Pricing Model, CAPM）描述了资产的预期收益与其系统性风险之间的关系：

```
Ri - Rf = α + β(Rm - Rf) + ε
```

- Ri：个股收益率
- Rf：无风险利率
- Rm：市场组合收益率
- β：个股对市场波动的敏感度
- α：超额收益（理论上应不显著异于零）

检验 CAPM 的核心就是跑一次线性回归，看看 α 是否显著、β 是多少。

## 获取 A 股数据

使用 `cnstockR` 包获取股票和指数日线数据。安装：

```r
install.packages("remotes")
remotes::install_github("ApolloMonasa/cnstockR")
```

获取贵州茅台（600519）和沪深300指数（000300）的数据：

```r
library(cnstockR)

stock <- cn_get_daily("600519", start = "2024-01-01", end = "2024-12-31", adjust = 1)
index <- cn_get_daily("000300", start = "2024-01-01", end = "2024-12-31", adjust = 1)

head(stock)
head(index)
```

## 计算收益率

使用对数收益率（连续复利）：

```r
stock$ret <- c(NA, diff(log(stock$close)))
index$ret <- c(NA, diff(log(index$close)))

# 对齐日期，合并为一个数据框
df <- merge(stock[, c("date", "ret")],
            index[, c("date", "ret")],
            by = "date", suffixes = c("_stock", "_index"))
df <- na.omit(df)
```

## 回归检验

```r
model <- lm(ret_stock ~ ret_index, data = df)
summary(model)
```

关注 `summary` 输出中的：
- **截距项（Intercept）**：即 α，若 p 值大于 0.05，则 α 不显著，符合 CAPM 预期
- **ret_index 系数**：即 β，衡量个股的系统性风险
- **R-squared**：市场收益对个股收益的解释力度

## 绘制回归图

```r
plot(df$ret_index, df$ret_stock,
     xlab = "市场收益率", ylab = "个股收益率",
     main = "CAPM 回归：600519 vs 沪深300",
     pch = 16, col = rgb(0.2, 0.4, 0.6, 0.5))
abline(model, col = "red", lwd = 2)
legend("topleft", legend = paste0("beta = ", round(coef(model)[2], 4),
       "\nR² = ", round(summary(model)$r.squared, 4)),
       bty = "n")

# 残差诊断
par(mfrow = c(2, 2))
plot(model)
par(mfrow = c(1, 1))
```

## 关键点

- 使用**对数收益率**，不要用简单收益率（价格差直接相除）
- 换一只股票、换一个时间段，β 会不同，这就是 CAPM 的"稳定性"问题
- 如果 α 显著不为零，说明该股票存在超出 CAPM 解释范围的超额收益
