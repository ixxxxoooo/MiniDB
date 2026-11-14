package database

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"time"

	mysql "github.com/go-sql-driver/mysql"
)

// MySQL 系的 TLS 选项（ConnectionConfig.SSLMode）。空串等同 SSLDisable，与旧版本行为一致。
const (
	SSLDisable    = "disable"     // 不加密
	SSLPreferred  = "preferred"   // 服务器支持就加密，不校验证书；不支持则明文
	SSLRequired   = "required"    // 必须加密，不校验证书
	SSLVerifyCA   = "verify-ca"   // 必须加密，校验证书由受信任的 CA 签发（不校验主机名）
	SSLVerifyFull = "verify-full" // 必须加密，校验证书与主机名
)

// mysqlTLSVerifyCA 是注册给驱动的自定义 TLS 配置名：只校验证书链，不校验主机名。
const mysqlTLSVerifyCA = "minidb-verify-ca"

func init() {
	// 标准库的校验会同时核对主机名；这里关掉它，自行只校验证书链。
	_ = mysql.RegisterTLSConfig(mysqlTLSVerifyCA, &tls.Config{
		InsecureSkipVerify: true,
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return fmt.Errorf("服务器没有提供证书")
			}
			opts := x509.VerifyOptions{Intermediates: x509.NewCertPool()}
			for _, c := range cs.PeerCertificates[1:] {
				opts.Intermediates.AddCert(c)
			}
			_, err := cs.PeerCertificates[0].Verify(opts)
			return err
		},
	})
}

// mysqlTLSParam 把 SSLMode 换成驱动的 tls 参数；返回空串表示不启用 TLS。
func mysqlTLSParam(sslMode string) string {
	switch sslMode {
	case SSLPreferred:
		return "preferred"
	case SSLRequired:
		return "skip-verify"
	case SSLVerifyCA:
		return mysqlTLSVerifyCA
	case SSLVerifyFull:
		return "true"
	}
	return ""
}

func mysqlDSN(cfg *ConnectionConfig, interpolateParams bool) string {
	c := mysql.Config{
		User:                 cfg.User,
		Passwd:               cfg.Password,
		Net:                  "tcp",
		Addr:                 fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		DBName:               cfg.Database,
		AllowNativePasswords: true,
		ParseTime:            true,
		Loc:                  time.Local,
		InterpolateParams:    interpolateParams,
		// 只限制建连超时。不能设置 ReadTimeout/WriteTimeout：它们作用于每一次网络读写，
		// 会让任何执行时间超过该值的查询、DDL、导出批次以 i/o timeout 失败。
		// 语句级超时与取消统一由调用方的 context 控制。
		Timeout: defaultConnectTimeout,
		Params: map[string]string{
			"charset": "utf8mb4",
		},
	}
	if tlsParam := mysqlTLSParam(cfg.SSLMode); tlsParam != "" {
		c.TLSConfig = tlsParam
	}
	return c.FormatDSN()
}
