FROM rocker/rstudio:4.6.0
RUN install2.r --error tidyverse rmarkdown renv
