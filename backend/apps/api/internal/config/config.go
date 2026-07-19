// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package config

import (
	"github.com/interviewmaster/interviewmaster/backend/internal/platform/appconfig"
	"github.com/zeromicro/go-zero/rest"
)

type Config struct {
	rest.RestConf
	Runtime appconfig.Settings
	Auth struct {
		AccessSecret string
		AccessExpire int64
	}
}
