package config

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// TencentConfig 腾讯云 CVM 运行时配置（独立文件 config/tencent.yaml，不混入主 config）。
// 用于 runtime-go tencent provider：实例创建到腾讯云 CVM。
// 文件不存在时 GetTencent() 返回 nil——表示不用腾讯云（走 localdocker）。
type TencentConfig struct {
	Enabled bool `mapstructure:"enabled"` // 是否启用腾讯云 runtime（false 走 localdocker）

	Region          string `mapstructure:"region"`           // ap-beijing
	Zone            string `mapstructure:"zone"`             // ap-beijing-6
	SecretID        string `mapstructure:"secretId"`         // 留空走 TENCENTCLOUD_SECRET_ID
	SecretKey       string `mapstructure:"secretKey"`        // 留空走 TENCENTCLOUD_SECRET_KEY
	VpcID           string `mapstructure:"vpcId"`            // 例：vpc-xxxxxxxx（在 config/tencent.yaml 配置）
	SubnetID        string `mapstructure:"subnetId"`         // 例：subnet-xxxxxxxx
	SecurityGroupID string `mapstructure:"securityGroupId"`  // 例：sg-xxxxxxxx
	ImageID         string `mapstructure:"imageId"`          // 例：img-xxxxxxxx（Ubuntu 24.04）
	InstanceType    string `mapstructure:"instanceType"`     // 例：SA2.MEDIUM4
	BandwidthMbps   int    `mapstructure:"bandwidthMbps"`    // 公网带宽上限（默认5；桌面实例建议10+）
	LoginKey        string `mapstructure:"loginKey"`         // 例：skey-xxxxxxxx（SSH 密钥对）
	LoginUser       string `mapstructure:"loginUser"`        // ubuntu
}

var tencentCfg *TencentConfig

// LoadTencent 从 config/tencent.yaml 加载腾讯云配置。文件不存在返回 (nil, nil)。
func LoadTencent() (*TencentConfig, error) {
	v := viper.New()
	v.SetConfigName("tencent")
	v.SetConfigType("yaml")
	v.AddConfigPath("config")
	v.AddConfigPath("./config")
	if pwd, err := os.Getwd(); err == nil {
		v.AddConfigPath(pwd + "/config")
	}
	if err := v.ReadInConfig(); err != nil {
		// 文件不存在视为「未启用腾讯云」，不报错
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			tencentCfg = nil
			return nil, nil
		}
		return nil, fmt.Errorf("read tencent config: %w", err)
	}
	c := &TencentConfig{}
	if err := v.Unmarshal(c); err != nil {
		return nil, fmt.Errorf("unmarshal tencent config: %w", err)
	}
	// 凭据兜底：文件没写就走环境变量。
	if c.SecretID == "" {
		c.SecretID = os.Getenv("TENCENTCLOUD_SECRET_ID")
	}
	if c.SecretKey == "" {
		c.SecretKey = os.Getenv("TENCENTCLOUD_SECRET_KEY")
	}
	tencentCfg = c
	return c, nil
}

// GetTencent 已加载的腾讯云配置（未加载返回 nil）。
func GetTencent() *TencentConfig { return tencentCfg }
