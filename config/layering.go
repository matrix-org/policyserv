package config

import (
	"encoding/json"
	"log"

	"github.com/kelseyhightower/envconfig"
)

var baseConfigRaw *CommunityConfig

func init() {
	buildBaseConfig()
}

func buildBaseConfig() {
	baseConfigRaw = &CommunityConfig{}
	err := envconfig.Process("ps", baseConfigRaw)
	if err != nil {
		log.Fatal(err)
	}
}

func NewCommunityConfigForJSON(configJson []byte) (*CommunityConfig, error) {
	return newCommunityConfigForJSONWithBase(baseConfigRaw, configJson)
}

func newCommunityConfigForJSONWithBase(baseConfig *CommunityConfig, configJson []byte) (*CommunityConfig, error) {
	// Clone the base config so we can apply the JSON as overlay/overrides
	overlay, err := baseConfig.Clone()
	if err != nil {
		return nil, err
	}
	if configJson != nil && len(configJson) != 2 { // only apply the overlay if there's actual JSON to apply
		err = json.Unmarshal(configJson, overlay) // apply JSON over top of base ("default") config
		if err != nil {
			return nil, err
		}
	}
	return overlay, nil
}

func NewDefaultCommunityConfig() (*CommunityConfig, error) {
	baseConfigRaw = &CommunityConfig{}
	err := envconfig.Process("not_the_normal_prefix_to_prevent_picking_up_live_env_values__if_you_set_this_then_please_do_not_do_that", baseConfigRaw)
	return baseConfigRaw, err
}
