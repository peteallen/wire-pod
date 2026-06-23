package wirepod_ttr

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/fforchino/vector-go-sdk/pkg/vector"
	"github.com/fforchino/vector-go-sdk/pkg/vectorpb"
	"github.com/kercre123/wire-pod/chipper/pkg/logger"
	"github.com/kercre123/wire-pod/chipper/pkg/vars"
)

type normalizedEyeColor struct {
	Hex        string
	Hue        float64
	Saturation float64
}

func ShouldBypassLocalIntentMatcher() bool {
	return vars.APIConfig.Knowledge.Enable &&
		vars.APIConfig.Knowledge.IntentGraph &&
		vars.APIConfig.Knowledge.Provider != "houndify"
}

func normalizeEyeColorHex(hexColor string) (normalizedEyeColor, error) {
	color := strings.TrimSpace(hexColor)
	color = strings.TrimPrefix(color, "#")
	if len(color) == 3 {
		color = string([]byte{color[0], color[0], color[1], color[1], color[2], color[2]})
	}
	if len(color) != 6 {
		return normalizedEyeColor{}, fmt.Errorf("invalid hex color %q", hexColor)
	}

	r, err := parseHexByte(color[0:2])
	if err != nil {
		return normalizedEyeColor{}, fmt.Errorf("invalid red channel in %q", hexColor)
	}
	g, err := parseHexByte(color[2:4])
	if err != nil {
		return normalizedEyeColor{}, fmt.Errorf("invalid green channel in %q", hexColor)
	}
	b, err := parseHexByte(color[4:6])
	if err != nil {
		return normalizedEyeColor{}, fmt.Errorf("invalid blue channel in %q", hexColor)
	}

	hue, saturation := rgbToHSLHueSaturation(r, g, b)
	return normalizedEyeColor{
		Hex:        "#" + strings.ToUpper(color),
		Hue:        hue,
		Saturation: saturation,
	}, nil
}

func parseHexByte(input string) (uint8, error) {
	value, err := strconv.ParseUint(input, 16, 8)
	if err != nil {
		return 0, err
	}
	return uint8(value), nil
}

func rgbToHSLHueSaturation(red, green, blue uint8) (float64, float64) {
	r := float64(red) / 255
	g := float64(green) / 255
	b := float64(blue) / 255

	maxValue := math.Max(r, math.Max(g, b))
	minValue := math.Min(r, math.Min(g, b))
	delta := maxValue - minValue
	lightness := (maxValue + minValue) / 2
	if delta == 0 {
		return 0, 0
	}

	var hue float64
	switch maxValue {
	case r:
		hue = math.Mod((g-b)/delta, 6)
	case g:
		hue = ((b - r) / delta) + 2
	default:
		hue = ((r - g) / delta) + 4
	}
	hue /= 6
	if hue < 0 {
		hue += 1
	}

	saturation := delta / (1 - math.Abs(2*lightness-1))
	return hue, saturation
}

func DoSetEyeColor(hexColor string, robot *vector.Vector) error {
	color, err := normalizeEyeColorHex(hexColor)
	if err != nil {
		return err
	}
	return applyCustomEyeColor(color, robot)
}

func applyCustomEyeColor(color normalizedEyeColor, robot *vector.Vector) error {
	if robot == nil {
		return errors.New("robot connection is not available")
	}
	if robot.Cfg.Target == "" || robot.Cfg.Token == "" {
		return errors.New("robot target or token is missing")
	}

	if _, err := robot.Conn.SetEyeColor(context.Background(), &vectorpb.SetEyeColorRequest{
		Hue:        float32(color.Hue),
		Saturation: float32(color.Saturation),
	}); err != nil {
		logger.Println("Unable to set immediate SDK eye color: " + err.Error())
	}

	payload := map[string]any{
		"update_settings": true,
		"settings": map[string]any{
			"custom_eye_color": map[string]any{
				"enabled":    true,
				"hue":        color.Hue,
				"saturation": color.Saturation,
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "https://"+robot.Cfg.Target+"/v1/update_settings", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+robot.Cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("robot update_settings returned %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}

	logger.Println(fmt.Sprintf("Set Vector eye color to %s (hue %.3f, saturation %.3f)", color.Hex, color.Hue, color.Saturation))
	return nil
}
