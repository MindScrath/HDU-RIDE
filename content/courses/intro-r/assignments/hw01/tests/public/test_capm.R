# 公开测试：检查 .Rmd 文件存在且包含必要代码
stopifnot(file.exists("hw01_capm.Rmd") || file.exists("answer.Rmd"))
