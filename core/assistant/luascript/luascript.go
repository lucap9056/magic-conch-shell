package luascript

import (
	_ "embed"
)

//go:embed script.lua
var script string

func GetScript() string {
	return script
}
