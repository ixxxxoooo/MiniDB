package app

import "minidb/internal/model"

// Error 是带错误码的错误，定义在 model 包；界面按错误码取当前语言的文案。
type Error = model.Error
