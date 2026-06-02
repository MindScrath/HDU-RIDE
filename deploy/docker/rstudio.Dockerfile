FROM rocker/rstudio:4.6.0

# 先安装编译 R 包所需的系统依赖
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

# 用 -r 显式指定 CRAN 镜像，绕过默认的 p3m.dev（国内可能无法访问）
RUN install2.r --error -r https://cloud.r-project.org tidyverse rmarkdown renv
