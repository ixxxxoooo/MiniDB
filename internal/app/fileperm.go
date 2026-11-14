package app

import "os"

// createPrivateFile 创建（或截断）只有当前用户可读写的文件。
// 导出、备份文件里是业务数据或连接口令，不能依赖 umask。
// 文件已存在时 OpenFile 不会改权限，所以创建后再 Chmod 一次。
func createPrivateFile(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// writePrivateFile 把 data 写入只有当前用户可读写的文件。
func writePrivateFile(path string, data []byte) error {
	f, err := createPrivateFile(path)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
