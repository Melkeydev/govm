package utils

import "runtime"

type ErrMsg error

type VersionsMsg []GoVersion

type DeleteCompleteMsg struct {
	Version string
}

var goBinary = "go"

func init() {
	if runtime.GOOS == "windows" {
		goBinary = "go.exe"
	}
}
