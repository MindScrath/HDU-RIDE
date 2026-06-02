FROM rocker/rstudio:4.6.0

# 覆盖默认的 p3m.dev 仓库（国内可能无法访问），改用 cloud.r-project.org
# cloud.r-project.org 会自动重定向到最近的 CRAN 镜像
RUN sed -i 's|https://[^"]*p3m\.dev[^"]*|https://cloud.r-project.org|g' /etc/R/Rprofile.site && \
    install2.r --error -r https://cloud.r-project.org tidyverse rmarkdown renv
