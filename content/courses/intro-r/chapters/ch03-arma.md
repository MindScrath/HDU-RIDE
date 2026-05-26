# 第3章 ARMA 过程

在上一章中，我们认识了时间序列的基本统计特征，并学习了最基础的平稳过程——白噪声。然而，真实的金融数据极少是纯粹的白噪声，它们往往在不同时间点之间存在着复杂的记忆性和相关性。本章我们将利用白噪声作为“建筑砌块”，搭建出现代时间序列分析中最核心的模型家族：自回归移动平均模型（ARMA 模型）。

## 3.1 移动平均 (MA) 过程

移动平均过程（Moving Average, MA）是最直观的一种模型。它假设当前时刻的观测值，是由过去一段时间内的白噪声“冲击”线性组合而成的。

### 3.1.1 MA(q) 过程

从白噪声出发，用白噪声序列 $\{\varepsilon_t\}$ 构成 MA 过程。最简单的是一阶移动平均过程，记作 MA(1)：
$$y_t = \mu + \varepsilon_t + \theta_1 \varepsilon_{t-1}$$
其中 $\mu$ 是常数项（均值），$\varepsilon_t$ 是均值为 0，方差为 $\sigma^2$ 的白噪声。

一般地，包含 $q$ 个滞后白噪声项的模型称为 $q$ 阶移动平均过程，记作 MA(q)：
$$y_t = \mu + \varepsilon_t + \theta_1 \varepsilon_{t-1} + \theta_2 \varepsilon_{t-2} + \cdots + \theta_q \varepsilon_{t-q}$$

**统计特征考察**

**(1) 均值**
对 MA(q) 两边取期望，由于白噪声的期望 $E(\varepsilon_{t-i}) = 0$：
$$E(y_t) = \mu + 0 + 0 + \cdots + 0 = \mu$$
均值是一个与时间无关的常数。

**(2) 方差**
由于不同时刻的白噪声互相独立，它们之间的协方差期望为 0：
$$
\begin{aligned}
Var(y_t) &= E[(y_t - \mu)^2] \\
&= E[(\varepsilon_t + \theta_1 \varepsilon_{t-1} + \cdots + \theta_q \varepsilon_{t-q})^2] \\
&= Var(\varepsilon_t) + \theta_1^2 Var(\varepsilon_{t-1}) + \cdots + \theta_q^2 Var(\varepsilon_{t-q}) \\
&= (1 + \theta_1^2 + \cdots + \theta_q^2)\sigma^2
\end{aligned}
$$
方差也是一个与时间无关的常数。

**(3) 自协方差与自相关函数 (ACF)**
对于 MA(1) 过程：
当滞后 $k=1$ 时，自协方差为：
$$
\begin{aligned}
\gamma_1 &= Cov(y_t, y_{t-1}) \\
&= E[(\varepsilon_t + \theta_1 \varepsilon_{t-1})(\varepsilon_{t-1} + \theta_1 \varepsilon_{t-2})] \\
&= \theta_1 E(\varepsilon_{t-1}^2) = \theta_1 \sigma^2
\end{aligned}
$$
此时，自相关系数 $\rho_1$ 为：
$$\rho_1 = \frac{\gamma_1}{\gamma_0} = \frac{\theta_1 \sigma^2}{(1 + \theta_1^2)\sigma^2} = \frac{\theta_1}{1 + \theta_1^2}$$
当 $k \ge 2$ 时，由于 $y_t$ 和 $y_{t-k}$ 之间没有任何重叠的历史白噪声项，所以 $\gamma_k = 0$，$\rho_k = 0$。

推广到 MA(q) 过程，它的 ACF 具有一个极其鲜明的统计特点——**$q$ 阶截尾性**。即在滞后期数 $k \le q$ 时，自相关系数不为 0；一旦 $k > q$，自相关系数突然全部变为 0。
由于 MA(q) 的均值、方差是常数，且自协方差仅依赖于滞后期数 $k$，这直接证明了：**任何有限阶的 MA(q) 过程都是永远平稳的。**

### 3.1.2 一般线性模型

如果我们将 MA 过程的滞后阶数推广到无穷大，就得到了无穷阶移动平均过程 MA($\infty$)：
$$y_t = \mu + \varepsilon_t + \psi_1 \varepsilon_{t-1} + \psi_2 \varepsilon_{t-2} + \cdots = \mu + \sum_{i=0}^\infty \psi_i \varepsilon_{t-i}$$
（其中 $\psi_0 = 1$）

这样的过程还是平稳的吗？我们来考察它的方差：
$$Var(y_t) = \sum_{i=0}^\infty \psi_i^2 \sigma^2 = \sigma^2 \sum_{i=0}^\infty \psi_i^2$$
要使得方差是一个有限的常数（弱平稳的必要条件），必须满足**系数平方和有限**的数学条件：
$$\sum_{i=0}^\infty \psi_i^2 < \infty$$
满足这一系数收敛条件的 MA($\infty$) 过程，被称为**一般线性模型**（General Linear Model）。这告诉我们一个重要结论：只要系数序列的平方和收敛，无穷阶移动平均过程也是平稳的。

## 3.2 自回归 (AR) 过程

时间序列数据的一个强烈特点是“惯性”或自相关性。一个直接的建模思路就是考虑用序列自身的历史值来解释当前的表现，这就是自回归模型（Autoregressive Model, 简称 AR 模型）的由来。

### 3.2.1 AR(1) 的定义与迭代计算

最简单的是一阶自回归过程，记作 AR(1)，表示当期的随机变量是常数项、自身的滞后一期项和当期白噪声的线性组合：
$$y_t = c + \phi y_{t-1} + \varepsilon_t$$
其中, $c$ 是截距项，$\varepsilon_t$ 是白噪声。

我们回到时间 $t-1$ 期时，方程可以写成：
$$y_{t-1} = c + \phi y_{t-2} + \varepsilon_{t-1}$$
将上式代入 $y_t$ 的方程中，可以得到：
$$
\begin{aligned}
y_t &= c + \phi(c + \phi y_{t-2} + \varepsilon_{t-1}) + \varepsilon_t \\
&= c + \phi c + \phi^2 y_{t-2} + \varepsilon_t + \phi \varepsilon_{t-1}
\end{aligned}
$$
按照这个思路，对 $y_{t-2}, y_{t-3}, \cdots, y_{t-n}$ 继续做不断的迭代替换，可以得到:
$$y_t = (1 + \phi + \phi^2 + \cdots + \phi^n)c + \phi^{n+1} y_{t-n-1} + \varepsilon_t + \phi \varepsilon_{t-1} + \phi^2 \varepsilon_{t-2} + \cdots + \phi^n \varepsilon_{t-n}$$

要使这个演化过程具有统计上的意义（收敛），当 $n \to \infty$ 时，系统必须不至于发散。容易看出，$c$ 的系数是一个等比数列累计求和，其收敛的条件为 $|\phi| < 1$。
在这时会有 $\lim_{n \to \infty} \phi^{n+1} = 0$，且 $c$ 的系数收敛于 $\frac{1}{1 - \phi}$。因此当 $n$ 趋于无穷大时，方程可以写成：
$$y_t = \frac{c}{1 - \phi} + \varepsilon_t + \phi \varepsilon_{t-1} + \phi^2 \varepsilon_{t-2} + \cdots$$

这实质上是将 AR(1) 过程转换成了一个无穷阶移动平均过程 MA($\infty$)。同时，其白噪声系数的平方和满足 $1 + \phi^2 + \phi^4 + \cdots = \frac{1}{1 - \phi^2} < \infty$。
这完全符合我们在上一节讨论的“一般线性模型”的平稳性条件！因此，这个过程是平稳的。
我们由此得出了时间序列中最核心的结论之一：**AR(1) 过程的平稳性条件是其系数 $|\phi| < 1$。**

### 3.2.2 滞后算子与脉冲响应函数

为了简化上述极其繁琐的迭代推导，计量经济学家引入了**滞后算子（Lag Operator）**，通常记为 $B$。该算子作用于序列，就相当于将序列在时间轴上向后移动一期：
$$B y_t = y_{t-1}, \quad B^2 y_t = y_{t-2}$$
用滞后算子重新表达 AR(1) 模型：
$$y_t = c + \phi B y_t + \varepsilon_t \implies (1 - \phi B)y_t = c + \varepsilon_t$$
考虑将算子多项式 $(1 - \phi B)$ 移到右侧（相当于求逆）：
$$y_t = \frac{c}{1 - \phi B} + \frac{\varepsilon_t}{1 - \phi B}$$
这里我们需要对 $\frac{1}{1 - \phi B}$ 做泰勒展开：
$$\frac{1}{1 - \phi B} = 1 + \phi B + \phi^2 B^2 + \phi^3 B^3 + \cdots$$
代入原式，我们得到了和之前迭代法一模一样的 MA($\infty$) 展开式：
$$y_t = \frac{c}{1 - \phi} + (1 + \phi B + \phi^2 B^2 + \cdots)\varepsilon_t = \frac{c}{1 - \phi} + \varepsilon_t + \phi \varepsilon_{t-1} + \phi^2 \varepsilon_{t-2} + \cdots$$

在这里，白噪声前的系数序列 $\{1, \phi, \phi^2, \phi^3, \cdots\}$ 被称为**脉冲响应函数（Impulse Response Function, IRF）**。它生动地衡量了一个单位的外部白噪声“冲击” $\varepsilon_t$，对未来各期 $y_{t+k}$ 产生的影响大小。
对于平稳的 AR(1)（因为 $|\phi| < 1$），这种冲击的影响会随着时间的推移呈指数级衰减，直至归零。这在经济学上被称为“市场记忆逐渐消退”。

### 3.2.3 平稳 AR(1) 的统计特征 (Yule-Walker 方程)

在 $|\phi| < 1$ 的平稳条件下，我们来计算 AR(1) 的关键统计特征。
对方程 $y_t = c + \phi y_{t-1} + \varepsilon_t$ 两端同取期望：
$$E(y_t) = c + \phi E(y_{t-1}) \implies \mu = c + \phi \mu \implies \mu = \frac{c}{1 - \phi}$$

为了方便计算方差和自协方差，我们通常先将原序列**中心化**，即减去均值令 $\tilde{y}_t = y_t - \mu$：
$$\tilde{y}_t = \phi \tilde{y}_{t-1} + \varepsilon_t$$
对该式两端同乘以历史值 $\tilde{y}_{t-k}$ 并取期望（其中 $k=0, 1, 2, \dots$），可以得到大名鼎鼎的 **Yule-Walker 方程**体系。

当 $k=0$ 时：
$$E(\tilde{y}_t \tilde{y}_t) = \phi E(\tilde{y}_{t-1} \tilde{y}_t) + E(\varepsilon_t \tilde{y}_t)$$
由于 $\tilde{y}_t$ 中完全包含了当前的 $\varepsilon_t$（两者同向变动），所以 $E(\varepsilon_t \tilde{y}_t) = \sigma^2$。同时根据平稳性的方差恒定要求，$E(\tilde{y}_t^2) = E(\tilde{y}_{t-1}^2) = \gamma_0$：
$$\gamma_0 = \phi \gamma_1 + \sigma^2$$

当 $k=1$ 时：
$$E(\tilde{y}_t \tilde{y}_{t-1}) = \phi E(\tilde{y}_{t-1} \tilde{y}_{t-1}) + E(\varepsilon_t \tilde{y}_{t-1}) \implies \gamma_1 = \phi \gamma_0 + 0$$
将 $\gamma_1$ 代入上面 $k=0$ 时的公式，求得 AR(1) 模型的**方差**为：
$$\gamma_0 = \frac{\sigma^2}{1 - \phi^2}$$

当 $k \ge 1$ 时，一般的推导可以得到 n 阶的 Yule-Walker 方程：
$$\gamma_k = \phi \gamma_{k-1}$$
将两边同除以方差 $\gamma_0$，我们就得到了自相关函数 (ACF) 的递推公式：
$$\rho_k = \phi \rho_{k-1} \implies \rho_k = \phi^k$$
可以看出，平稳 AR(1) 过程的自相关函数随着滞后期数 $k$ 呈现出指数级的持续衰减（即没有突然截断），这种特征被称为**拖尾性**。

### 3.2.4 爆炸增长的 AR(1) 与无效差分 (含R代码演示)

如果 $|\phi| \ge 1$，系统会发生什么？
设想 $\phi = 1.05$，此时方程为 $y_t = 1.05 y_{t-1} + \varepsilon_t$。根据脉冲响应的逻辑，其系数表现为 $1.05, 1.05^2, 1.05^3 \dots$。一个微小的初始市场冲击，其影响不但不会消退，反而会像滚雪球一样越滚越大！这就是在数学上发散的**爆炸式增长 AR(1) 模型**。
这种序列极端非平稳。更可怕的是，它不能像我们在第二章讲过的随机游走（$\phi = 1$）那样，通过简单的差分处理就转化为平稳序列。

我们用 R 语言来进行具体的模拟和差分尝试：
```R
library(tidyverse)
library(tsibble)
library(cowplot)

set.seed(42)
n <- 100
# 模拟爆炸式 AR(1) (设定 phi = 1.05)
# 由于指数增长极快，数值容易溢出，这里模拟样本量 n 设置为较小的 100
y <- numeric(n)
eps <- rnorm(n)
y[1] <- eps[1]
for(t in 2:n){
  y[t] <- 1.05 * y[t-1] + eps[t] # 迭代生成序列
}

# 组合数据并进行差分
df_exp <- tibble(
  time = 1:n,
  Y = y,
  diff_1 = Y - lag(Y),
  diff_2 = diff_1 - lag(diff_1)
)

# 使用 ggplot2 作图比较
p1 <- ggplot(df_exp, aes(x = time, y = Y)) + 
  geom_line(color="darkred") + 
  labs(title="爆炸式 AR(1): ϕ=1.05", y="Y") + theme_bw()
  
p2 <- ggplot(df_exp, aes(x = time, y = diff_1)) + 
  geom_line(color="steelblue") + 
  labs(title="一阶差分后", y="ΔY") + theme_bw()
  
p3 <- ggplot(df_exp, aes(x = time, y = diff_2)) + 
  geom_line(color="darkgreen") + 
  labs(title="二阶差分后", y="Δ²Y") + theme_bw()

plot_grid(p1, p2, p3, nrow=3)
```
> [!WARNING]
> 运行上述代码后，你会清晰地观察到：无论是原序列，还是一阶差分、二阶差分后的序列，它们统统都表现出爆炸式的指数增长特征。
> 这说明：**对于 $|\phi| > 1$ 的发散序列，传统的差分方法是彻底无效的。** 在实际金融研究中，如果遇到这种宏观经济或金融泡沫数据，我们通常需要先对其取对数（Logarithm），将其指数增长转化为线性增长，然后再进行差分处理。

### 3.2.5 AR(p) 过程

将模型的历史记忆滞后阶数扩展到 $p$ 阶，我们便得到了一般的 AR(p) 模型：
$$y_t = c + \phi_1 y_{t-1} + \phi_2 y_{t-2} + \cdots + \phi_p y_{t-p} + \varepsilon_t$$
使用滞后算子 $B$ 可以优雅地表示为：
$$(1 - \phi_1 B - \phi_2 B^2 - \cdots - \phi_p B^p) y_t = c + \varepsilon_t$$
我们令 $\Phi(B) = 1 - \phi_1 B - \phi_2 B^2 - \cdots - \phi_p B^p$，这个部分被称为**AR 多项式**。此时方程等价于 $\Phi(B)y_t = c + \varepsilon_t$。

**平稳性的判断方法**
要判断一个复杂高阶的 AR(p) 模型是否平稳，我们需要求解其对应的**特征方程**：
$$\lambda^p - \phi_1 \lambda^{p-1} - \phi_2 \lambda^{p-2} - \cdots - \phi_p = 0$$
如果该特征方程的所有根（包括可能存在的复根）的绝对值**都严格小于 1**（即在几何上，所有的根都落在复平面的单位圆内），则该 AR(p) 过程是平稳的。等价地，如果我们考察多项式方程 $\Phi(z)=0$，其所有的根必须落在单位圆外。这两种判断方法在代数上是殊途同归的。

同样地，平稳的 AR(p) 也能导出高维的 Yule-Walker 方程组。其自相关函数（ACF）表现为**拖尾性**，其形态通常是一系列指数衰减和正弦波振荡的复杂组合。

## 3.3 MA 过程的可逆性 (Invertibility)

讲完了 AR 过程，我们再回过头来考察一下移动平均 MA 过程。请思考下面两个没有常数项的极简 MA(1) 过程：
- **模型 A**：$y_t = \varepsilon_t + 0.5 \varepsilon_{t-1}$
- **模型 B**：$y_t = \varepsilon_t + 2 \varepsilon_{t-1}$

根据我们在 3.1 节推导的公式 $\rho_1 = \frac{\theta_1}{1 + \theta_1^2}$：
对于模型 A，$\rho_1 = \frac{0.5}{1 + 0.5^2} = 0.4$。
对于模型 B，$\rho_1 = \frac{2}{1 + 2^2} = 0.4$。

令人困惑的现象出现了：**两个系数完全不同的 MA 模型，居然算出了完全相同的自相关函数！** 
在实际的数据分析中，模型估计程序是靠“看”自相关函数来反推参数的。如果不同的模型对应同样的 ACF，计算机就无法判断数据到底是模型 A 生成的还是模型 B 生成的（在计量经济学中，这被称为参数“无法识别”）。

为了打破这个僵局，我们需要人为施加一个过滤条件，这个条件叫做**可逆性（Invertibility）**。
所谓可逆性，就是要求我们将 MA 模型“逆转”表示为一个合理的、系数收敛的 AR 模型。
尝试用滞后算子将 MA(1) 模型 $y_t = (1 + \theta_1 B)\varepsilon_t$ 重新反向表述，即把 $y_t$ 放在分母：
$$\varepsilon_t = \frac{1}{1 + \theta_1 B} y_t$$
对上述分式进行泰勒展开的数学前提，是其展开的无穷级数必须收敛，这强制要求系数满足 $|\theta_1| < 1$。
- 模型 A 的 $|\theta_1| = 0.5 < 1$，级数收敛，它是**可逆的**。
- 模型 B 的 $|\theta_1| = 2 > 1$，级数发散，它是**不可逆的**。

一般地，对于更复杂的 MA(q) 过程 $(1 + \theta_1 B + \cdots + \theta_q B^q)\varepsilon_t = \Theta(B)\varepsilon_t$，其可逆条件是 **MA 多项式方程 $\Theta(x) = 0$ 的所有根的绝对值都大于 1**。
好消息是，数学家已经证明：对于任何一个不可逆的 MA 过程，必定存在唯一一个具有相同 ACF 的可逆 MA 过程与之对应。因此，在统计软件（如 R）的底层运算中，总是默认选用那个**满足可逆性条件**的模型。

## 3.4 偏自相关函数 (PACF)

在上一节我们知道，AR(p) 过程的自相关函数（ACF）是拖尾的（永远不会突然变为0）。这就导致我们无法像 MA(q) 那样通过肉眼观察 ACF 突然截断的位置来直观判断模型的阶数 $p$。为了精准地定阶 AR 模型，我们需要引入时间序列分析的另一件重型武器：**偏自相关函数（Partial Autocorrelation Function, PACF）**。

**(1) PACF 的直观定义**
对于一个平稳的时间序列，滞后 $k$ 阶的自相关系数 $\rho_k$ 衡量了 $y_t$ 和 $y_{t-k}$ 之间的所有相关性。但是，这种相关性其实是“不纯粹的”，因为它夹杂了无数中间变量（即 $y_{t-1}, y_{t-2}, \dots, y_{t-k+1}$）在时间链条上慢慢传递过来的间接影响。
偏自相关函数，顾名思义，就是在统计学上运用偏回归技术，**剔除了所有中间变量的线性干扰后，$y_t$ 和 $y_{t-k}$ 之间剩余的、纯粹的直接相关性。**

**(2) AR 和 MA 过程的 PACF 核心特点**
- **AR(p) 过程**：这正是 PACF 的主场。因为 AR(p) 过程的当期值 $y_t$ 本质上是由且仅由过去 $p$ 期的值所决定的。因此，当我们把前 $p$ 期的所有间接影响都严格剔除后，更早的历史（例如滞后 $p+1$ 期）对当期不再有任何纯粹的新解释力。所以，AR(p) 过程的 PACF 图在 $p$ 阶之后会突然干净利落地变为 0。这就叫 **PACF 的 $p$ 阶截尾性**。
- **MA(q) 过程**：还记得可逆性吗？由于任何可逆的 MA(q) 过程都可以反向转换为无限阶的 AR($\infty$) 过程，因此 MA 过程的 PACF 永远不会在某一阶之后彻底变为 0，表现为如同波浪般的**拖尾性**。

## 3.5 终极合体：ARMA 过程

在现实金融市场中，单纯的 AR 或单纯的 MA 往往不够用。如果我们把两者的优势结合起来：一部分波动用自身的历史价格解释（AR），另一部分波动用历史上无法解释的新闻/冲击（即白噪声）解释（MA），我们就得到了金融分析中大杀四方的 **ARMA(p,q) 过程**：
$$y_t = c + \phi_1 y_{t-1} + \cdots + \phi_p y_{t-p} + \varepsilon_t + \theta_1 \varepsilon_{t-1} + \cdots + \theta_q \varepsilon_{t-q}$$
用滞后算子的形式，方程可以表示得极其紧凑而优美：
$$\Phi(B) y_t = c + \Theta(B) \varepsilon_t$$

**(1) 平稳性与可逆性判断的分离**
面对如此庞大的模型，它的统计条件其实是可以完全解耦分析的：
- **平稳性**完全且只由自回归部分决定。只要其 AR 多项式 $\Phi(B)$ 满足平稳条件，整个 ARMA 就是平稳的。
- **可逆性**完全且只由移动平均部分决定。只要其 MA 多项式 $\Theta(B)$ 满足可逆条件，整个 ARMA 就是可逆的。

**(2) ARMA(1,1) 的 Wold 表示**
考虑最经典也是应用最广的平稳且可逆的 ARMA(1,1) 模型：
$$(1 - \phi_1 B)y_t = c + (1 + \theta_1 B)\varepsilon_t$$
利用算子的代数规则，我们可以将其等式左边移到右边，完全展开为一个无穷阶的移动平均模型：
$$y_t = \frac{c}{1 - \phi_1 B} + \frac{1 + \theta_1 B}{1 - \phi_1 B} \varepsilon_t = \mu + \Psi(B) \varepsilon_t$$
这种将任何平稳过程表示为一个常数项加上无穷多个过去白噪声线性组合的极简形式，被称为 **Wold 表示形式**。这在数学上是由著名的 **Wold 分解定理** 提供严密保障的。其中，新生成的多项式 $\Psi(B)$ 的系数序列，就是整个复杂经济系统的**脉冲响应函数**。

---

### 💡 本章核心武器库总结表

我们在下一章面临实证分析时，最核心的步骤就是根据数据画出的 ACF 和 PACF 图来为模型定阶（识别 $p$ 和 $q$）。这是无数计量经济学考试的必考点，也是实战量化的基本功。请务必熟记下表：

| 时间序列模型 | 自相关函数 (ACF) 表现 | 偏自相关函数 (PACF) 表现 |
| :--- | :--- | :--- |
| **纯 MA(q) 过程** | **$q$ 阶截尾** (在 lag > q 时突然截断为 0) | 拖尾 (指数衰减或震荡，不归零) |
| **纯 AR(p) 过程** | 拖尾 (指数衰减或震荡，不归零) | **$p$ 阶截尾** (在 lag > p 时突然截断为 0) |
| **混合 ARMA(p,q) 过程** | 拖尾 | 拖尾 |

有了这份“看图识模型”的指南针，在接下来的第四章中，我们将进入真实的金融数据实战，完整体验从平稳性检验到建立模型并进行价格预测的全流程。
