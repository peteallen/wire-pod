package wirepod_localstt

import (
	"fmt"
	"os"
	"strings"

	"github.com/kercre123/wire-pod/chipper/pkg/vars"
	sr "github.com/kercre123/wire-pod/chipper/pkg/wirepod/speechrequest"
	vosk "github.com/kercre123/wire-pod/chipper/pkg/wirepod/stt/vosk"
	whispercpp "github.com/kercre123/wire-pod/chipper/pkg/wirepod/stt/whisper.cpp"
)

var Name string = "auto"

func provider() string {
	selected := strings.TrimSpace(vars.APIConfig.STT.Service)
	if selected == "" {
		selected = strings.TrimSpace(os.Getenv("STT_SERVICE"))
	}
	if selected == "" {
		selected = vosk.Name
	}
	if selected == "whisper" {
		return whispercpp.Name
	}
	return selected
}

func Init() error {
	switch provider() {
	case vosk.Name:
		return vosk.Init()
	case whispercpp.Name:
		if strings.TrimSpace(vars.APIConfig.STT.Model) != "" {
			os.Setenv("WHISPER_MODEL", strings.TrimSpace(vars.APIConfig.STT.Model))
		}
		return whispercpp.Init()
	default:
		return fmt.Errorf("unsupported local STT provider: %s", provider())
	}
}

func STT(req sr.SpeechRequest) (string, error) {
	switch provider() {
	case vosk.Name:
		return vosk.STT(req)
	case whispercpp.Name:
		return whispercpp.STT(req)
	default:
		return "", fmt.Errorf("unsupported local STT provider: %s", provider())
	}
}
