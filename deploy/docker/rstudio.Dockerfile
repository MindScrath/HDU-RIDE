FROM rocker/rstudio:4.6.0

# 用 -r 显式指定 CRAN 镜像，绕过默认的 p3m.dev（国内可能无法访问）
# cloud.r-project.org 会自动重定向到最近的可用镜像
RUN install2.r --error -r https://cloud.r-project.org tidyverse rmarkdown renv
