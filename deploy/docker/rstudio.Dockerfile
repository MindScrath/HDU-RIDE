FROM rocker/rstudio:4.6.0

# 替换为阿里云 apt 源（Ubuntu Noble 24.04 使用 deb822 格式）
RUN sed -i 's|http://archive.ubuntu.com/ubuntu/|http://mirrors.aliyun.com/ubuntu/|g' /etc/apt/sources.list.d/ubuntu.sources && \
    sed -i 's|http://security.ubuntu.com/ubuntu/|http://mirrors.aliyun.com/ubuntu/|g' /etc/apt/sources.list.d/ubuntu.sources && \
    sed -i 's|http://ports.ubuntu.com/ubuntu-ports/|http://mirrors.aliyun.com/ubuntu-ports/|g' /etc/apt/sources.list.d/ubuntu.sources

# 安装编译 R 包所需的系统依赖
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    libcurl4-openssl-dev \
    libssl-dev \
    libxml2-dev \
    libgit2-dev \
    libharfbuzz-dev \
    libfribidi-dev \
    libfreetype6-dev \
    libpng-dev \
    libtiff5-dev \
    libjpeg-dev \
    libfontconfig1-dev \
    && rm -rf /var/lib/apt/lists/*

# 用清华 CRAN 镜像替代默认的 p3m.dev（国内无法访问）
RUN install2.r --error -r https://mirrors.tuna.tsinghua.edu.cn/CRAN/ tidyverse rmarkdown renv
