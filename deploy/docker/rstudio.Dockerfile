FROM rocker/tidyverse:4.6.0

# 替换为阿里云 apt 源（Ubuntu Noble 24.04 deb822 格式）
RUN sed -i 's|http://archive.ubuntu.com/ubuntu/|http://mirrors.aliyun.com/ubuntu/|g' /etc/apt/sources.list.d/ubuntu.sources && \
    sed -i 's|http://security.ubuntu.com/ubuntu/|http://mirrors.aliyun.com/ubuntu/|g' /etc/apt/sources.list.d/ubuntu.sources && \
    sed -i 's|http://ports.ubuntu.com/ubuntu-ports/|http://mirrors.aliyun.com/ubuntu-ports/|g' /etc/apt/sources.list.d/ubuntu.sources

# renv 是轻量 R 包，从清华 CRAN 源码安装即可
RUN install2.r --error -r https://mirrors.tuna.tsinghua.edu.cn/CRAN/ renv
