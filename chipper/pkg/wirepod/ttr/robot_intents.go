package wirepod_ttr

import (
	"context"
	"errors"
	"strings"

	"github.com/fforchino/vector-go-sdk/pkg/vector"
	"github.com/fforchino/vector-go-sdk/pkg/vectorpb"
	"github.com/kercre123/wire-pod/chipper/pkg/logger"
)

type robotIntentDefinition struct {
	Intent      string
	Description string
}

var robotIntentCatalog = []robotIntentDefinition{
	{"intent_character_age", "answer Vector's age"},
	{"intent_explore_start", "start exploring"},
	{"intent_system_charger", "go home or return to the charger"},
	{"intent_system_sleep", "go to sleep"},
	{"intent_greeting_goodmorning", "good morning greeting"},
	{"intent_greeting_goodnight", "good night greeting"},
	{"intent_greeting_goodbye", "goodbye greeting"},
	{"intent_seasonal_happynewyear", "happy new year behavior"},
	{"intent_seasonal_happyholidays", "holiday greeting behavior"},
	{"intent_amazon_signin", "start Alexa sign-in"},
	{"intent_imperative_forward", "drive forward"},
	{"intent_imperative_turnaround", "turn around"},
	{"intent_imperative_turnleft", "turn left"},
	{"intent_imperative_turnright", "turn right"},
	{"intent_play_rollcube", "roll the cube"},
	{"intent_play_popawheelie", "pop a wheelie with the cube"},
	{"intent_play_fistbump", "fist bump"},
	{"intent_play_blackjack", "start blackjack"},
	{"intent_photo_take_extend", "take a photo"},
	{"intent_imperative_praise", "react to praise"},
	{"intent_imperative_abuse", "react to criticism"},
	{"intent_imperative_apologize", "react to an apology"},
	{"intent_imperative_backup", "back up"},
	{"intent_imperative_volumedown", "turn volume down"},
	{"intent_imperative_volumeup", "turn volume up"},
	{"intent_imperative_lookatme", "look at the user"},
	{"intent_imperative_shutup", "stop talking"},
	{"intent_imperative_come", "come here"},
	{"intent_imperative_love", "react to affection"},
	{"intent_clock_checktimer", "check current timer"},
	{"intent_global_stop_extend", "stop or cancel the active timer"},
	{"intent_clock_time", "tell the time"},
	{"intent_imperative_quiet", "be quiet or stop current behavior"},
	{"intent_imperative_dance", "dance"},
	{"intent_play_pickupcube", "pick up the cube"},
	{"intent_imperative_fetchcube", "fetch the cube"},
	{"intent_imperative_findcube", "find the cube"},
	{"intent_play_anytrick", "do a trick"},
	{"intent_message_recordmessage_extend", "record a message"},
	{"intent_message_playmessage_extend", "play a saved message"},
	{"intent_blackjack_hit", "blackjack hit"},
	{"intent_blackjack_stand", "blackjack stand"},
	{"intent_play_keepaway", "play keep away"},
}

func robotIntentNames() []string {
	names := make([]string, 0, len(robotIntentCatalog))
	for _, intent := range robotIntentCatalog {
		names = append(names, intent.Intent)
	}
	return names
}

func robotIntentPromptCatalog() string {
	var parts []string
	for _, intent := range robotIntentCatalog {
		parts = append(parts, intent.Intent+" ("+intent.Description+")")
	}
	return strings.Join(parts, ", ")
}

func validRobotIntent(intentName string) bool {
	for _, intent := range robotIntentCatalog {
		if intent.Intent == intentName {
			return true
		}
	}
	return false
}

func DoRunBuiltinIntent(intentName string, robot *vector.Vector) error {
	if robot == nil {
		return errors.New("robot connection is not available")
	}
	intentName = strings.TrimSpace(intentName)
	if !validRobotIntent(intentName) {
		return errors.New("invalid built-in robot intent " + intentName)
	}
	if intentName == "intent_system_charger" {
		return DoGoToCharger(robot)
	}
	_, err := robot.Conn.AppIntent(context.Background(), &vectorpb.AppIntentRequest{Intent: intentName})
	if err != nil {
		return err
	}
	logger.Println("Sent built-in Vector intent " + intentName)
	return nil
}
